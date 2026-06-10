package signals

import (
	"context"
	"log/slog"
	"time"

	"github.com/odsod/recorder/internal/timeline"
)

// ParticipantState is a meeting participant and whether they are speaking.
type ParticipantState struct {
	Name     string
	Speaking bool
}

// MeetingChange signals that the active meeting tab changed.
type MeetingChange struct {
	Title string
}

// PollResult is the outcome of a single speaker-detection poll.
type PollResult struct {
	Participants  []ParticipantState
	MeetingChange *MeetingChange
}

// SpeakerPoller polls CDP for active speakers and meeting state.
type SpeakerPoller interface {
	Poll(ctx context.Context) (PollResult, error)
}

// RunSpeakerCollector polls CDP and updates speaker and meeting timelines.
func RunSpeakerCollector(
	ctx context.Context,
	detector SpeakerPoller,
	speakerTimeline *timeline.SpeakerTimeline,
	participantSet *timeline.ParticipantSet,
	meetingState *timeline.MeetingState,
) {
	collector := &SpeakerCollector{
		Detector: detector,
		Tracker:  NewSpeakerTracker(DefaultSpeakerTrackerConfig()),
		Timeline: speakerTimeline,
		People:   participantSet,
		Meetings: meetingState,
	}

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := collector.PollOnce(ctx, time.Now()); err != nil {
				slog.ErrorContext(ctx, "cdp poll failed",
					"err", err,
				)
			}
		}
	}
}
