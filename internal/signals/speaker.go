package signals

import (
	"context"
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
	log func(string),
) {
	var activeSpeaker string

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			result, err := detector.Poll(ctx)
			if err != nil {
				log("cdp: " + err.Error())
				continue
			}

			if result.MeetingChange != nil {
				meetingState.Set(result.MeetingChange.Title)
				if result.MeetingChange.Title != "" {
					log("meeting: " + result.MeetingChange.Title)
				} else {
					log("meeting: ended")
				}
				participantSet.Reset()
			}

			if result.Participants == nil {
				continue
			}

			now := time.Now()
			names := make(map[string]struct{})
			for _, s := range result.Participants {
				names[s.Name] = struct{}{}
			}
			participantSet.Update(names)

			var speaker string
			for _, s := range result.Participants {
				if s.Speaking {
					speaker = s.Name
					break
				}
			}

			if speaker != activeSpeaker {
				activeSpeaker = speaker
				speakerTimeline.Append(now, speaker)
			}
		}
	}
}
