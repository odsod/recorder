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
			states, err := detector.Poll(ctx)
			if err != nil || states == nil {
				continue
			}

			now := time.Now()
			names := make(map[string]struct{})
			for _, s := range states {
				names[s.Name] = struct{}{}
			}
			participantSet.Update(names)

			var speaker string
			for _, s := range states {
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
