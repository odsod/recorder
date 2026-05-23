package transcript

import (
	"strings"
	"testing"
)

func TestParse_RoundTrip(t *testing.T) {
	events := []Event{
		{Time: ts("09:00:00"), Type: Speech, Source: "sys", Speaker: "Alice", Text: "Hello world"},
		{Time: ts("09:01:00"), Type: Speech, Source: "mic", Text: "Reply here"},
		{Time: ts("09:02:00"), Type: Meeting, Title: "Sprint Planning"},
		{Time: ts("09:03:00"), Type: Participants, People: []string{"Alice", "Bob"}},
		{Time: ts("09:04:00"), Type: Pin},
		{Time: ts("09:05:00"), Type: Idle, Text: "3 min"},
		{Time: ts("09:06:00"), Type: Recorder, Text: "started"},
		{Time: ts("09:07:00"), Type: Note, Text: "remember this"},
	}

	var lines []string
	for _, e := range events {
		lines = append(lines, e.String())
	}
	data := []byte(strings.Join(lines, "\n") + "\n")

	got, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Events) != len(events) {
		t.Fatalf("parsed %d events, want %d", len(got.Events), len(events))
	}

	for i, got := range got.Events {
		want := events[i]
		if got.Type != want.Type {
			t.Errorf("event[%d].Type = %d, want %d", i, got.Type, want.Type)
		}
		if got.Source != want.Source {
			t.Errorf("event[%d].Source = %q, want %q", i, got.Source, want.Source)
		}
		if got.Speaker != want.Speaker {
			t.Errorf("event[%d].Speaker = %q, want %q", i, got.Speaker, want.Speaker)
		}
		if got.Text != want.Text {
			t.Errorf("event[%d].Text = %q, want %q", i, got.Text, want.Text)
		}
		if got.Title != want.Title {
			t.Errorf("event[%d].Title = %q, want %q", i, got.Title, want.Title)
		}
		if len(got.People) != len(want.People) {
			t.Errorf("event[%d].People = %v, want %v", i, got.People, want.People)
		}
	}
}

func TestParse_MeetingEnded(t *testing.T) {
	e := Event{Time: ts("10:00:00"), Type: Meeting}
	got, err := Parse([]byte(e.String() + "\n"))
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Events) != 1 {
		t.Fatalf("parsed %d events, want 1", len(got.Events))
	}
	if got.Events[0].Type != Meeting {
		t.Errorf("Type = %d, want Meeting", got.Events[0].Type)
	}
	if got.Events[0].Title != "" {
		t.Errorf("Title = %q, want empty (ended)", got.Events[0].Title)
	}
}

func TestParse_SkipsInvalidLines(t *testing.T) {
	content := "---\ndate: 2026-01-01\n---\n\nnot a valid line\n[09:00:00] 🔊 **sys** hello\n"
	got, err := Parse([]byte(content))
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Events) != 1 {
		t.Fatalf("parsed %d events, want 1", len(got.Events))
	}
}
