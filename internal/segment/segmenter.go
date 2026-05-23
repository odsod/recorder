package segment

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const summarizeTimeout = 3 * time.Minute

type SegmentHandler interface {
	Summarize(ctx context.Context, seg Segment, date string) (title string, summary string, skip bool, err error)
	WriteSegment(title, summary string, seg Segment, date string) (string, error)
}

type FuncHandler struct {
	SummarizeFn    func(ctx context.Context, seg Segment, date string) (string, string, bool, error)
	WriteSegmentFn func(title, summary string, seg Segment, date string) (string, error)
}

func (h *FuncHandler) Summarize(ctx context.Context, seg Segment, date string) (string, string, bool, error) {
	return h.SummarizeFn(ctx, seg, date)
}

func (h *FuncHandler) WriteSegment(title, summary string, seg Segment, date string) (string, error) {
	return h.WriteSegmentFn(title, summary, seg, date)
}

type IncrementalSegmenter struct {
	ctx          context.Context
	handler      SegmentHandler
	log          func(string)
	appendSeg    func(timestamp, text string)
	events       []Event
	speechEvents []Event
	lastSpeech   time.Time
	pending      *Boundary
	wg           sync.WaitGroup
}

func NewSegmenter(
	ctx context.Context,
	handler SegmentHandler,
	log func(string),
	appendSeg func(timestamp, text string),
) *IncrementalSegmenter {
	return &IncrementalSegmenter{
		ctx:       ctx,
		handler:   handler,
		log:       log,
		appendSeg: appendSeg,
	}
}

func (s *IncrementalSegmenter) OnSpeech(t time.Time, tag, text string) {
	event := Event{Time: t, Tag: tag, Text: text}
	s.events = append(s.events, event)
	s.speechEvents = append(s.speechEvents, event)

	if s.pending != nil && !s.lastSpeech.IsZero() {
		s.finalize()
	}
	s.lastSpeech = t
}

func (s *IncrementalSegmenter) OnSignal(t time.Time, tag, emoji, text string) {
	s.events = append(s.events, Event{Time: t, Tag: tag, Emoji: emoji, Text: text})
}

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

func (s *IncrementalSegmenter) OnMeetingChange(newTitle string, t time.Time) {
	if !s.lastSpeech.IsZero() && len(s.events) > 0 && s.pending == nil {
		s.pending = &Boundary{
			Time:   t,
			Reason: fmt.Sprintf("meeting change → %s", newTitle),
		}
		s.log(fmt.Sprintf("segmenter: boundary detected (meeting change → %s)", newTitle))
	}
}

func (s *IncrementalSegmenter) OnPin(t time.Time) {
	snapped := SnapPin(t, s.speechEvents)
	s.pending = &Boundary{Time: snapped, Reason: "pin"}
	s.log("segmenter: boundary detected (pin)")
}

func (s *IncrementalSegmenter) Flush(ctx context.Context) {
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

	var segEvents []Event
	for _, e := range s.events {
		if !e.Time.After(boundaryTime) {
			segEvents = append(segEvents, e)
		}
	}

	var segSpeech []Event
	for _, e := range segEvents {
		if IsSpeech(e) {
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
		s.appendSeg(time.Now().Format("15:04:05"), fmt.Sprintf("| %s skip", seg.ID))
		s.log(fmt.Sprintf("segmenter: skipped segment %s", seg.ID))
		return
	}

	filename, err := s.handler.WriteSegment(title, summary, seg, date)
	if err != nil {
		s.log(fmt.Sprintf("segmenter error: %v", err))
		return
	}

	slug := Slugify(title)
	s.appendSeg(time.Now().Format("15:04:05"), fmt.Sprintf("| %s %s", seg.ID, slug))
	s.log(fmt.Sprintf("segmenter: wrote %s", filename))
}

func filterAfter(events []Event, t time.Time) []Event {
	var result []Event
	for _, e := range events {
		if e.Time.After(t) {
			result = append(result, e)
		}
	}
	return result
}
