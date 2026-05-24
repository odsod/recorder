package meet_test

import (
	"testing"

	"github.com/odsod/recorder/internal/conference/meet"
)

func TestMatchesURL(t *testing.T) {
	p := meet.New()

	tests := []struct {
		url  string
		want bool
	}{
		{"https://meet.google.com/abc-defg-hij", true},
		{"https://meet.google.com/abc-defg-hij?authuser=0", true},
		{"https://teams.microsoft.com/meeting/123", false},
		{"https://google.com", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := p.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestParseSnapshot(t *testing.T) {
	p := meet.New()

	t.Run("valid", func(t *testing.T) {
		json := `[{"name":"Alice Smith","classes":["cls-a","cls-b","cls-c"]},{"name":"Bob Jones","classes":["cls-a","cls-d"]}]`
		result, err := p.ParseSnapshot(json)
		if err != nil {
			t.Fatal(err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 participants, got %d", len(result))
		}
		if result[0].Name != "Alice Smith" {
			t.Errorf("expected 'Alice Smith', got %q", result[0].Name)
		}
		if len(result[0].Classes) != 3 {
			t.Errorf("expected 3 classes, got %d", len(result[0].Classes))
		}
		if result[1].Name != "Bob Jones" {
			t.Errorf("expected 'Bob Jones', got %q", result[1].Name)
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
	p := meet.New()

	t.Run("valid", func(t *testing.T) {
		json := `[{"name":"Alice Smith","speaking":true},{"name":"Bob Jones","speaking":false}]`
		result, err := p.ParsePoll(json)
		if err != nil {
			t.Fatal(err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 participants, got %d", len(result))
		}
		if result[0].Name != "Alice Smith" || !result[0].Speaking {
			t.Errorf("participant 0: got %+v", result[0])
		}
		if result[1].Name != "Bob Jones" || result[1].Speaking {
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
	p := meet.New()

	t.Run("valid class", func(t *testing.T) {
		js, err := p.PollExpression("speaking-indicator")
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
	p := meet.New()
	if p.Name() != "meet" {
		t.Errorf("expected 'meet', got %q", p.Name())
	}
}
