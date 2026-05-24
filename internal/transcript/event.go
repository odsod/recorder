package transcript

import (
	"fmt"
	"strings"
	"time"
)

// EventType categorizes transcript log entries.
type EventType int

// Transcript event categories.
const (
	Speech EventType = iota
	Meeting
	Participants
	Pin
	Idle
	Recorder
	Segment
	Note
)

// Event is a single append-only transcript log entry.
type Event struct {
	Time    time.Time
	Type    EventType
	Source  string   // "sys" or "mic" for speech events
	Text    string   // transcribed text (speech), display text (signals)
	Speaker string   // active speaker name (speech events)
	Title   string   // meeting title (meeting events)
	People  []string // participant names (participants events)
}

// IsSpeech reports whether the event is a mic or system transcription.
func (e Event) IsSpeech() bool {
	return e.Type == Speech
}

// IsMeeting reports whether the event is a meeting join or end signal.
func (e Event) IsMeeting() bool {
	return e.Type == Meeting
}

// IsPin reports whether the event is a user segment boundary hint.
func (e Event) IsPin() bool {
	return e.Type == Pin
}

// Tag returns the short markdown tag for this event type.
func (e Event) Tag() string {
	switch e.Type {
	case Speech:
		return e.Source
	case Meeting:
		return "mtg"
	case Participants:
		return "ppl"
	case Pin:
		return "pin"
	case Idle:
		return "idl"
	case Recorder:
		return "rec"
	case Segment:
		return "seg"
	case Note:
		return "nfo"
	default:
		return "?"
	}
}

// Emoji returns the display emoji for this event type.
func (e Event) Emoji() string {
	switch e.Type {
	case Speech:
		if e.Source == "mic" {
			return "🎤"
		}
		return "🔊"
	case Meeting:
		return "🪟"
	case Participants:
		return "👥"
	case Pin:
		return "📍"
	case Idle:
		return "💤"
	case Recorder:
		return "🟢"
	case Segment:
		return "✂️"
	case Note:
		return "📝"
	default:
		return "❓"
	}
}

func (e Event) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "[%s] %s **%s**", e.Time.Format("15:04:05"), e.Emoji(), e.Tag())

	if e.IsSpeech() && e.Speaker != "" {
		fmt.Fprintf(&sb, " [%s]", e.Speaker)
	}

	switch e.Type {
	case Speech:
		if e.Text != "" {
			sb.WriteString(" ")
			sb.WriteString(e.Text)
		}
	case Meeting:
		if e.Title != "" {
			sb.WriteString(" joined: ")
			sb.WriteString(e.Title)
		} else {
			sb.WriteString(" ended")
		}
	case Participants:
		if len(e.People) > 0 {
			sb.WriteString(" ")
			sb.WriteString(strings.Join(e.People, ", "))
		}
	default:
		if e.Text != "" {
			sb.WriteString(" ")
			sb.WriteString(e.Text)
		}
	}

	return sb.String()
}
