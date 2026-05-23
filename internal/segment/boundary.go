package segment

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

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

type Event struct {
	Time  time.Time
	Tag   string
	Emoji string
	Text  string
}

type Boundary struct {
	Time   time.Time
	Reason string
}

type Segment struct {
	Start  time.Time
	End    time.Time
	Events []Event
	ID     string
}

func IsSpeech(e Event) bool {
	return e.Tag == "sys" || e.Tag == "mic"
}

func IsPin(e Event) bool {
	return e.Tag == "pin"
}

func IsMeeting(e Event) bool {
	return e.Tag == "win" || e.Tag == "mtg"
}

func SnapPin(pinTime time.Time, speechEvents []Event) time.Time {
	var before []Event
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

func DetectBoundaries(events []Event, now time.Time) []Boundary {
	var boundaries []Boundary

	var speech []Event
	for _, e := range events {
		if IsSpeech(e) {
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
				Reason: fmt.Sprintf("meeting change → %s", m.title),
			})
		}
		lastTitle = m.title
	}

	// 3. Pins
	for _, e := range events {
		if IsPin(e) {
			snapped := SnapPin(e.Time, speech)
			boundaries = append(boundaries, Boundary{Time: snapped, Reason: "pin"})
		}
	}

	sort.Slice(boundaries, func(i, j int) bool {
		return boundaries[i].Time.Before(boundaries[j].Time)
	})
	return Dedupe(boundaries)
}

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

func SplitAtBoundaries(events []Event, boundaries []Boundary) []Segment {
	if len(events) == 0 {
		return nil
	}

	var speech []Event
	for _, e := range events {
		if IsSpeech(e) {
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

		var segEvents []Event
		for _, e := range events {
			if !e.Time.Before(start) && !e.Time.After(end) {
				segEvents = append(segEvents, e)
			}
		}

		hasSpeech := false
		for _, e := range segEvents {
			if IsSpeech(e) {
				hasSpeech = true
				break
			}
		}
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

var (
	meetingTitleRe = regexp.MustCompile(`"([^"]+)"(?:\s+(?:opened|active))?$`)
	meetingArrowRe = regexp.MustCompile(`→\s*"([^"]+)"`)
)

func extractMeetingTitle(text string) string {
	if strings.HasPrefix(text, "joined: ") {
		title := strings.TrimPrefix(text, "joined: ")
		if bareMeetingTitles[title] {
			return ""
		}
		return title
	}
	if m := meetingArrowRe.FindStringSubmatch(text); m != nil {
		if bareMeetingTitles[m[1]] {
			return ""
		}
		return m[1]
	}
	if m := meetingTitleRe.FindStringSubmatch(text); m != nil {
		if bareMeetingTitles[m[1]] {
			return ""
		}
		return m[1]
	}
	return ""
}

func meetingEvents(events []Event) []meetingEvent {
	var meetings []meetingEvent
	for _, e := range events {
		if !IsMeeting(e) {
			continue
		}
		title := extractMeetingTitle(e.Text)
		if title != "" {
			meetings = append(meetings, meetingEvent{time: e.Time, title: title})
		}
	}
	return meetings
}

var (
	slugRe      = regexp.MustCompile(`[^a-z0-9\s-]`)
	slugSpaceRe = regexp.MustCompile(`[\s]+`)
)

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
		if !IsSpeech(e) {
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

func extractSpeakerAndText(e Event) (string, string) {
	if strings.HasPrefix(e.Text, "[") {
		if idx := strings.Index(e.Text, "]"); idx != -1 {
			return strings.TrimSpace(e.Text[1:idx]), strings.TrimSpace(e.Text[idx+1:])
		}
	}
	return e.Tag, e.Text
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
