package transcript

import (
	"bufio"
	"regexp"
	"strings"
	"time"
)

// Transcript is a parsed daily event log.
type Transcript struct {
	Events []Event
}

var lineRe = regexp.MustCompile(`^\[(\d{2}:\d{2}:\d{2})\] (.+?) \*\*(\w+)\*\*(.*)$`)

// Parse reads transcript markdown lines into structured events.
func Parse(data []byte) (Transcript, error) {
	var events []Event
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		if e, ok := parseLine(scanner.Text()); ok {
			events = append(events, e)
		}
	}
	if err := scanner.Err(); err != nil {
		return Transcript{}, err
	}
	return Transcript{Events: events}, nil
}

func parseLine(line string) (Event, bool) {
	m := lineRe.FindStringSubmatch(line)
	if m == nil {
		return Event{}, false
	}

	ts, err := time.Parse("15:04:05", m[1])
	if err != nil {
		return Event{}, false
	}

	tag := m[3]
	text := strings.TrimSpace(m[4])

	e := Event{Time: ts, Text: text}
	switch tag {
	case "sys", "mic":
		e.Type = Speech
		e.Source = tag
		if strings.HasPrefix(text, "[") {
			if idx := strings.Index(text, "]"); idx != -1 {
				e.Speaker = strings.TrimSpace(text[1:idx])
				e.Text = strings.TrimSpace(text[idx+1:])
			}
		}
	case "mtg", "win":
		e.Type = Meeting
		e.Title = parseMeetingTitle(text)
		e.Text = ""
	case "ppl":
		e.Type = Participants
		e.People = parseParticipants(text)
		e.Text = ""
	case "pin":
		e.Type = Pin
	case "idl":
		e.Type = Idle
	case "rec":
		e.Type = Recorder
	case "seg":
		e.Type = Segment
	case "nfo":
		e.Type = Note
	default:
		return Event{}, false
	}

	return e, true
}

func parseMeetingTitle(text string) string {
	if after, ok := strings.CutPrefix(text, "joined: "); ok {
		return after
	}
	if text == "ended" {
		return ""
	}
	return text
}

func parseParticipants(text string) []string {
	var people []string
	for name := range strings.SplitSeq(text, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			people = append(people, name)
		}
	}
	return people
}
