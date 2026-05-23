package signals

import (
	"context"
	"time"

	"github.com/odsod/recorder/internal/cdp"
	"github.com/odsod/recorder/internal/timeline"
)

func RunSpeakerCollector(
	ctx context.Context,
	speakerTimeline *timeline.SpeakerTimeline,
	participantSet *timeline.ParticipantSet,
	meetingState *timeline.MeetingState,
	ports []int,
	log func(string),
) {
	detector := cdp.NewSpeakerDetector(ports)
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
