package segment

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/odsod/recorder/internal/transcript"
)

// Segment boundary tuning constants.
const (
	SilenceThreshold = 300 // 5 min gap between speech → boundary
	PinLookback      = 90  // seconds to search backwards for snap target
	PinSnapGap       = 3   // min gap that qualifies as snap target
	DedupWindow      = 120 // merge boundaries within 2 min
)

var bareMeetingTitles = map[string]bool{
	"Meet":              true,
	"Google Meet":       true,
	"meet.google.com_/": true,
}

// Boundary marks a segment split point with a human-readable reason.
type Boundary struct {
	Time   time.Time
	Reason string
}

// Segment is a contiguous slice of transcript events between boundaries.
type Segment struct {
	Start  time.Time
	End    time.Time
	Events []transcript.Event
	ID     string
}

// SnapPin moves a pin boundary to the nearest preceding speech gap.
func SnapPin(pinTime time.Time, speechEvents []transcript.Event) time.Time {
	var before []transcript.Event
	for _, e := range speechEvents {
		if !e.Time.After(pinTime) {
			before = append(before, e)
		}
	}
	if len(before) < 2 {
		return pinTime
	}

	for i := len(before) - 1; i > 0; i-- {
		curr := before[i]
		prev := before[i-1]
		gap := curr.Time.Sub(prev.Time).Seconds()
		lookback := pinTime.Sub(prev.Time).Seconds()
		if lookback > PinLookback {
			break
		}
		if gap >= PinSnapGap {
			return prev.Time
		}
	}
	return pinTime
}

// DetectBoundaries finds segment split points from silence, meetings, and pins.
func DetectBoundaries(events []transcript.Event, now time.Time) []Boundary {
	var boundaries []Boundary

	var speech []transcript.Event
	for _, e := range events {
		if e.IsSpeech() {
			speech = append(speech, e)
		}
	}
	meetings := meetingEvents(events)

	// 1. Silence gaps
	for i := 1; i < len(speech); i++ {
		gap := speech[i].Time.Sub(speech[i-1].Time).Seconds()
		if gap >= SilenceThreshold {
			boundaries = append(boundaries, Boundary{
				Time:   speech[i-1].Time,
				Reason: fmt.Sprintf("silence %.0fm", gap/60),
			})
		}
	}

	// Trailing silence
	if len(speech) > 0 {
		trailing := now.Sub(speech[len(speech)-1].Time).Seconds()
		if trailing >= SilenceThreshold {
			boundaries = append(boundaries, Boundary{
				Time:   speech[len(speech)-1].Time,
				Reason: fmt.Sprintf("trailing silence %.0fm", trailing/60),
			})
		}
	}

	// 2. Meeting identity change
	var lastTitle string
	for _, m := range meetings {
		if lastTitle != "" && m.title != lastTitle {
			boundaries = append(boundaries, Boundary{
				Time:   m.time,
				Reason: "meeting change → " + m.title,
			})
		}
		lastTitle = m.title
	}

	// 3. Pins
	for _, e := range events {
		if e.IsPin() {
			snapped := SnapPin(e.Time, speech)
			boundaries = append(boundaries, Boundary{Time: snapped, Reason: "pin"})
		}
	}

	sort.Slice(boundaries, func(i, j int) bool {
		return boundaries[i].Time.Before(boundaries[j].Time)
	})
	return Dedupe(boundaries)
}

// Dedupe merges boundaries that fall within DedupWindow of each other.
func Dedupe(boundaries []Boundary) []Boundary {
	if len(boundaries) == 0 {
		return nil
	}
	result := []Boundary{boundaries[0]}
	for _, b := range boundaries[1:] {
		if b.Time.Sub(result[len(result)-1].Time).Seconds() >= DedupWindow {
			result = append(result, b)
		}
	}
	return result
}

// SplitAtBoundaries partitions events into segments at the given boundaries.
func SplitAtBoundaries(events []transcript.Event, boundaries []Boundary) []Segment {
	if len(events) == 0 {
		return nil
	}

	var speech []transcript.Event
	for _, e := range events {
		if e.IsSpeech() {
			speech = append(speech, e)
		}
	}
	if len(speech) == 0 {
		return nil
	}

	cutTimes := make([]time.Time, len(boundaries))
	for i, b := range boundaries {
		cutTimes[i] = b.Time
	}

	starts := []time.Time{speech[0].Time}
	for _, ct := range cutTimes {
		found := ct
		for _, e := range speech {
			if e.Time.After(ct) {
				found = e.Time
				break
			}
		}
		starts = append(starts, found)
	}

	var segments []Segment
	for i, start := range starts {
		var end time.Time
		if i < len(starts)-1 {
			end = starts[i+1]
		} else {
			end = speech[len(speech)-1].Time
		}

		var segEvents []transcript.Event
		for _, e := range events {
			if !e.Time.Before(start) && !e.Time.After(end) {
				segEvents = append(segEvents, e)
			}
		}

		hasSpeech := slices.ContainsFunc(segEvents, transcript.Event.IsSpeech)
		if hasSpeech {
			segments = append(segments, Segment{
				Start:  start,
				End:    end,
				Events: segEvents,
				ID:     start.Format("1504"),
			})
		}
	}
	return segments
}

type meetingEvent struct {
	time  time.Time
	title string
}

func meetingEvents(events []transcript.Event) []meetingEvent {
	var meetings []meetingEvent
	for _, e := range events {
		if !e.IsMeeting() {
			continue
		}
		if e.Title != "" && !bareMeetingTitles[e.Title] {
			meetings = append(meetings, meetingEvent{time: e.Time, title: e.Title})
		}
	}
	return meetings
}

var (
	slugRe      = regexp.MustCompile(`[^a-z0-9\s-]`)
	slugSpaceRe = regexp.MustCompile(`[\s]+`)
)

// Slugify converts a title into a filesystem-safe slug.
func Slugify(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = slugRe.ReplaceAllString(s, "")
	s = slugSpaceRe.ReplaceAllString(s, "-")
	if len(s) > 50 {
		s = s[:50]
	}
	return s
}

// FormatTranscript formats speech events for LLM summarization.
func FormatTranscript(seg Segment) string {
	var lines []string
	for _, e := range seg.Events {
		if !e.IsSpeech() {
			continue
		}
		if isHallucination(e.Text) {
			continue
		}
		speaker, text := extractSpeakerAndText(e)
		lines = append(lines, fmt.Sprintf("[%s] %s: %s", e.Time.Format("15:04"), speaker, text))
	}
	return strings.Join(lines, "\n")
}

func extractSpeakerAndText(e transcript.Event) (string, string) {
	if e.Speaker != "" {
		return e.Speaker, e.Text
	}
	return e.Source, e.Text
}

func isHallucination(text string) bool {
	hallucinationPatterns := []string{
		"thank you for watching",
		"you're welcome",
		"obrigado",
		"obrigada",
		"takk for at du så med",
		"takk for at du så på",
		"gracias",
		"undertexter av",
		"undertekster av",
		"nothing meaningful was detected",
		"no meaningful speech was detected",
		"no content to clean",
		"[empty output]",
		"empty output",
		"no substantive speech content",
	}
	lower := strings.ToLower(strings.TrimSpace(text))
	if len(lower) < 3 {
		return true
	}
	for _, p := range hallucinationPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
