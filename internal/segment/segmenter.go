package segment

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/odsod/recorder/internal/transcript"
)

const summarizeTimeout = 3 * time.Minute

// Handler summarizes and writes completed segments.
type Handler interface {
	Summarize(ctx context.Context, seg Segment, date string) (title string, summary string, skip bool, err error)
	WriteSegment(title, summary string, seg Segment, date string) (string, error)
}

// FuncHandler adapts function values to Handler.
type FuncHandler struct {
	SummarizeFn    func(ctx context.Context, seg Segment, date string) (string, string, bool, error)
	WriteSegmentFn func(title, summary string, seg Segment, date string) (string, error)
}

// Summarize delegates to SummarizeFn.
func (h *FuncHandler) Summarize(ctx context.Context, seg Segment, date string) (string, string, bool, error) {
	return h.SummarizeFn(ctx, seg, date)
}

// WriteSegment delegates to WriteSegmentFn.
func (h *FuncHandler) WriteSegment(title, summary string, seg Segment, date string) (string, error) {
	return h.WriteSegmentFn(title, summary, seg, date)
}

// IncrementalSegmenter detects boundaries online and triggers summarization.
type IncrementalSegmenter struct {
	ctx       context.Context
	handler   Handler
	log       func(string)
	appendSeg func(transcript.Event)

	events       []transcript.Event
	speechEvents []transcript.Event
	lastSpeech   time.Time
	pending      *Boundary
	wg           sync.WaitGroup
}

// NewSegmenter creates an online segmenter with the given handler and callbacks.
func NewSegmenter(
	ctx context.Context,
	handler Handler,
	log func(string),
	appendSeg func(transcript.Event),
) *IncrementalSegmenter {
	return &IncrementalSegmenter{
		ctx:       ctx,
		handler:   handler,
		log:       log,
		appendSeg: appendSeg,
	}
}

// OnSpeech records speech and finalizes a pending boundary when speech resumes.
func (s *IncrementalSegmenter) OnSpeech(e transcript.Event) {
	s.events = append(s.events, e)
	s.speechEvents = append(s.speechEvents, e)

	if s.pending != nil && !s.lastSpeech.IsZero() {
		s.finalize()
	}
	s.lastSpeech = e.Time
}

// OnEvent records a non-speech transcript event.
func (s *IncrementalSegmenter) OnEvent(e transcript.Event) {
	s.events = append(s.events, e)
}

// OnSilence detects a boundary when silence crosses SilenceThreshold.
func (s *IncrementalSegmenter) OnSilence(durationSecs int) {
	if durationSecs >= SilenceThreshold && s.pending == nil {
		if !s.lastSpeech.IsZero() {
			s.pending = &Boundary{
				Time:   s.lastSpeech,
				Reason: fmt.Sprintf("silence %dm", durationSecs/60),
			}
			s.log(fmt.Sprintf("segmenter: boundary detected (silence %dm)", durationSecs/60))
		}
	}
}

// OnMeetingChange detects a boundary when the active meeting tab changes.
func (s *IncrementalSegmenter) OnMeetingChange(newTitle string, t time.Time) {
	if !s.lastSpeech.IsZero() && len(s.events) > 0 && s.pending == nil {
		s.pending = &Boundary{
			Time:   t,
			Reason: "meeting change → " + newTitle,
		}
		s.log(fmt.Sprintf("segmenter: boundary detected (meeting change → %s)", newTitle))
	}
}

// OnPin records a user-placed segment boundary hint.
func (s *IncrementalSegmenter) OnPin(t time.Time) {
	snapped := SnapPin(t, s.speechEvents)
	s.pending = &Boundary{Time: snapped, Reason: "pin"}
	s.log("segmenter: boundary detected (pin)")
}

// Flush finalizes any pending segment and waits for summarization goroutines.
func (s *IncrementalSegmenter) Flush(_ context.Context) {
	s.ctx = context.WithoutCancel(s.ctx)
	if s.pending != nil && len(s.speechEvents) > 0 {
		s.finalize()
	} else if len(s.speechEvents) > 0 {
		s.pending = &Boundary{
			Time:   s.speechEvents[len(s.speechEvents)-1].Time,
			Reason: "shutdown",
		}
		s.finalize()
	}
	s.wg.Wait()
}

func (s *IncrementalSegmenter) finalize() {
	if len(s.speechEvents) == 0 {
		s.pending = nil
		return
	}

	boundaryTime := s.pending.Time

	var segEvents []transcript.Event
	for _, e := range s.events {
		if !e.Time.After(boundaryTime) {
			segEvents = append(segEvents, e)
		}
	}

	var segSpeech []transcript.Event
	for _, e := range segEvents {
		if e.IsSpeech() {
			segSpeech = append(segSpeech, e)
		}
	}

	if len(segSpeech) == 0 {
		s.events = filterAfter(s.events, boundaryTime)
		s.speechEvents = filterAfter(s.speechEvents, boundaryTime)
		s.pending = nil
		return
	}

	seg := Segment{
		Start:  segSpeech[0].Time,
		End:    segSpeech[len(segSpeech)-1].Time,
		Events: segEvents,
		ID:     segSpeech[0].Time.Format("1504"),
	}

	s.events = filterAfter(s.events, boundaryTime)
	s.speechEvents = filterAfter(s.speechEvents, boundaryTime)
	s.pending = nil

	s.wg.Add(1)
	go s.summarizeAndWrite(seg)
}

func (s *IncrementalSegmenter) summarizeAndWrite(seg Segment) {
	defer s.wg.Done()

	ctx, cancel := context.WithTimeout(s.ctx, summarizeTimeout)
	defer cancel()

	date := seg.Start.Format("2006-01-02")
	title, summary, skip, err := s.handler.Summarize(ctx, seg, date)
	if err != nil {
		s.log(fmt.Sprintf("segmenter error: %v", err))
		return
	}

	if skip || summary == "" {
		s.appendSeg(transcript.Event{
			Time: time.Now(),
			Type: transcript.Segment,
			Text: fmt.Sprintf("| %s skip", seg.ID),
		})
		s.log("segmenter: skipped segment " + seg.ID)
		return
	}

	filename, err := s.handler.WriteSegment(title, summary, seg, date)
	if err != nil {
		s.log(fmt.Sprintf("segmenter error: %v", err))
		return
	}

	slug := Slugify(title)
	s.appendSeg(transcript.Event{
		Time: time.Now(),
		Type: transcript.Segment,
		Text: fmt.Sprintf("| %s %s", seg.ID, slug),
	})
	s.log("segmenter: wrote " + filename)
}

func filterAfter(events []transcript.Event, t time.Time) []transcript.Event {
	var result []transcript.Event
	for _, e := range events {
		if e.Time.After(t) {
			result = append(result, e)
		}
	}
	return result
}
