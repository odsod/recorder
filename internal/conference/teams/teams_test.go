package teams_test

import (
	"testing"

	"github.com/odsod/recorder/internal/conference/teams"
)

func TestMatchesURL(t *testing.T) {
	p := teams.New()

	tests := []struct {
		url  string
		want bool
	}{
		{"https://teams.microsoft.com/meeting/123", true},
		{"https://teams.microsoft.com/l/meetup-join/abc", true},
		{"https://meet.google.com/abc-defg-hij", false},
		{"https://microsoft.com", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := p.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestParseSnapshot(t *testing.T) {
	p := teams.New()

	t.Run("valid", func(t *testing.T) {
		json := `[{"name":"participant-tile-alice","classes":["cls-x","cls-y"]},{"name":"participant-tile-bob","classes":["cls-x"]}]`
		result, err := p.ParseSnapshot(json)
		if err != nil {
			t.Fatal(err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 participants, got %d", len(result))
		}
		if result[0].Name != "participant-tile-alice" {
			t.Errorf("expected 'participant-tile-alice', got %q", result[0].Name)
		}
		if len(result[0].Classes) != 2 {
			t.Errorf("expected 2 classes, got %d", len(result[0].Classes))
		}
	})

	t.Run("empty array", func(t *testing.T) {
		result, err := p.ParseSnapshot("[]")
		if err != nil {
			t.Fatal(err)
		}
		if len(result) != 0 {
			t.Errorf("expected 0 participants, got %d", len(result))
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := p.ParseSnapshot("not json")
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestParsePoll(t *testing.T) {
	p := teams.New()

	t.Run("valid", func(t *testing.T) {
		json := `[{"name":"participant-tile-alice","speaking":true},{"name":"participant-tile-bob","speaking":false}]`
		result, err := p.ParsePoll(json)
		if err != nil {
			t.Fatal(err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 participants, got %d", len(result))
		}
		if result[0].Name != "participant-tile-alice" || !result[0].Speaking {
			t.Errorf("participant 0: got %+v", result[0])
		}
		if result[1].Name != "participant-tile-bob" || result[1].Speaking {
			t.Errorf("participant 1: got %+v", result[1])
		}
	})

	t.Run("empty array", func(t *testing.T) {
		result, err := p.ParsePoll("[]")
		if err != nil {
			t.Fatal(err)
		}
		if len(result) != 0 {
			t.Errorf("expected 0 participants, got %d", len(result))
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := p.ParsePoll("not json")
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestPollExpression(t *testing.T) {
	p := teams.New()

	t.Run("valid class", func(t *testing.T) {
		js, err := p.PollExpression("active-speaker")
		if err != nil {
			t.Fatal(err)
		}
		if js == "" {
			t.Fatal("expected non-empty JS")
		}
	})

	t.Run("invalid class with spaces", func(t *testing.T) {
		_, err := p.PollExpression("invalid class")
		if err == nil {
			t.Fatal("expected error for class with spaces")
		}
	})

	t.Run("invalid class with special chars", func(t *testing.T) {
		_, err := p.PollExpression("cls'; alert('xss');//")
		if err == nil {
			t.Fatal("expected error for class with special chars")
		}
	})

	t.Run("empty class", func(t *testing.T) {
		_, err := p.PollExpression("")
		if err == nil {
			t.Fatal("expected error for empty class")
		}
	})
}

func TestName(t *testing.T) {
	p := teams.New()
	if p.Name() != "teams" {
		t.Errorf("expected 'teams', got %q", p.Name())
	}
}
