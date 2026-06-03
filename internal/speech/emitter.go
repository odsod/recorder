package speech

import (
	"context"
	"errors"
	"time"

	"github.com/odsod/recorder/internal/timeline"
	"github.com/odsod/recorder/internal/transcript"
)

// Cleaner cleans raw transcription text.
type Cleaner interface {
	Cleanup(ctx context.Context, text string, participants []string) (string, error)
}

// SpeakerLookup returns speaker coverage for a time window.
type SpeakerLookup interface {
	Coverage(start, end time.Time, opts timeline.SpeakerLookupOptions) timeline.SpeakerAttribution
}

// ParticipantProvider returns the current known meeting participants.
type ParticipantProvider func() []string

// Emitter converts speech segments into transcript events.
type Emitter struct {
	Cleaner       Cleaner
	SpeakerLookup SpeakerLookup
	Deduper       Deduper
	Participants  ParticipantProvider
	LookupOptions timeline.SpeakerLookupOptions
}

// Emit cleans, attributes, and deduplicates segments without writing them.
func (e *Emitter) Emit(
	ctx context.Context,
	source string,
	segments []Segment,
	refs []transcript.Event,
) ([]transcript.Event, error) {
	var emitted []transcript.Event
	var errs []error
	for _, segment := range segments {
		if source == "mic" && e.Deduper != nil && e.Deduper.IsDuplicate(segment, refs) {
			continue
		}

		cleaned := segment.Text
		if e.Cleaner != nil {
			cleanupText, err := e.Cleaner.Cleanup(ctx, segment.Text, e.participants())
			if err != nil {
				errs = append(errs, err)
			}
			if cleanupText != "" {
				cleaned = cleanupText
			}
		}
		if cleaned == "" {
			continue
		}

		speaker := ""
		if e.SpeakerLookup != nil {
			speaker = FormatAttribution(e.SpeakerLookup.Coverage(
				segment.Start,
				segment.End,
				e.LookupOptions,
			))
		}

		emitted = append(emitted, transcript.Event{
			Time:    segment.Start,
			Type:    transcript.Speech,
			Source:  source,
			Text:    cleaned,
			Speaker: speaker,
		})
	}
	return emitted, errors.Join(errs...)
}

func (e *Emitter) participants() []string {
	if e.Participants == nil {
		return nil
	}
	return e.Participants()
}
