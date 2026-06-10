package signals

import (
	"context"
	"log/slog"
	"time"
)

// SpeakerTimelineWriter records debounced speaker transitions.
type SpeakerTimelineWriter interface {
	SetSpeakerActive(ts time.Time, name string, active bool)
	Append(ts time.Time, name string)
}

// ParticipantWriter records participants seen in the current meeting.
type ParticipantWriter interface {
	Update(names map[string]struct{}) map[string]struct{}
	Reset()
}

// MeetingWriter records active meeting title changes.
type MeetingWriter interface {
	Set(title string)
}

// SpeakerCollector coordinates one CDP poll with participant, meeting, and speaker state.
type SpeakerCollector struct {
	Detector SpeakerPoller
	Tracker  *SpeakerTracker
	Timeline SpeakerTimelineWriter
	People   ParticipantWriter
	Meetings MeetingWriter
}

// PollOnce polls detector once and writes debounced state changes.
func (c *SpeakerCollector) PollOnce(ctx context.Context, now time.Time) error {
	if c.Tracker == nil {
		c.Tracker = NewSpeakerTracker(DefaultSpeakerTrackerConfig())
	}

	result, err := c.Detector.Poll(ctx)
	if err != nil {
		return err
	}

	if result.MeetingChange != nil {
		c.Meetings.Set(result.MeetingChange.Title)
		if result.MeetingChange.Title != "" {
			slog.InfoContext(ctx, "meeting joined",
				"title", result.MeetingChange.Title,
			)
		} else {
			slog.InfoContext(ctx, "meeting ended")
		}
		c.People.Reset()
		c.Tracker = NewSpeakerTracker(DefaultSpeakerTrackerConfig())
		c.Timeline.Append(now, "")
	}

	if result.Participants == nil {
		return nil
	}

	names := make(map[string]struct{})
	for _, p := range result.Participants {
		names[p.Name] = struct{}{}
	}
	c.People.Update(names)

	for _, transition := range c.Tracker.Observe(now, result.Participants) {
		c.Timeline.SetSpeakerActive(transition.Time, transition.Name, transition.Active)
		if transition.Active {
			slog.InfoContext(ctx, "speaker started",
				"name", transition.Name,
			)
		} else {
			slog.InfoContext(ctx, "speaker stopped",
				"name", transition.Name,
			)
		}
	}

	return nil
}
