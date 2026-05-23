package transcript

import (
	"testing"
	"time"
)

func ts(s string) time.Time {
	t, _ := time.Parse("15:04:05", s)
	return t
}

func TestEvent_String_SysWithSpeaker(t *testing.T) {
	e := Event{Time: ts("15:04:32"), Type: Speech, Source: "sys", Speaker: "Alice", Text: "Hello world"}
	got := e.String()
	want := "[15:04:32] 🔊 **sys** [Alice] Hello world"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEvent_String_MicNoSpeaker(t *testing.T) {
	e := Event{Time: ts("09:00:00"), Type: Speech, Source: "mic", Text: "Some text"}
	got := e.String()
	want := "[09:00:00] 🎤 **mic** Some text"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEvent_String_Pin(t *testing.T) {
	e := Event{Time: ts("12:00:00"), Type: Pin}
	got := e.String()
	want := "[12:00:00] 📍 **pin**"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEvent_String_Recorder(t *testing.T) {
	e := Event{Time: ts("08:00:00"), Type: Recorder, Text: "started"}
	got := e.String()
	want := "[08:00:00] 🟢 **rec** started"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEvent_String_Meeting(t *testing.T) {
	e := Event{Time: ts("09:30:00"), Type: Meeting, Title: "Sprint Planning"}
	got := e.String()
	want := "[09:30:00] 🪟 **mtg** joined: Sprint Planning"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEvent_String_MeetingEnded(t *testing.T) {
	e := Event{Time: ts("10:00:00"), Type: Meeting}
	got := e.String()
	want := "[10:00:00] 🪟 **mtg** ended"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEvent_String_Participants(t *testing.T) {
	e := Event{Time: ts("09:31:00"), Type: Participants, People: []string{"Alice", "Bob"}}
	got := e.String()
	want := "[09:31:00] 👥 **ppl** Alice, Bob"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
