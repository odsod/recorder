package speech

import (
	"testing"

	"github.com/odsod/recorder/internal/timeline"
)

func TestFormatAttribution_Empty(t *testing.T) {
	if got := FormatAttribution(timeline.SpeakerAttribution{}); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestFormatAttribution_OneSpeaker(t *testing.T) {
	got := FormatAttribution(timeline.SpeakerAttribution{
		Candidates: []timeline.SpeakerCandidate{
			{Name: "Alice", CoveragePct: 0.55},
		},
	})
	want := "Alice 55%"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatAttribution_MultipleSpeakers(t *testing.T) {
	got := FormatAttribution(timeline.SpeakerAttribution{
		Candidates: []timeline.SpeakerCandidate{
			{Name: "Alice", CoveragePct: 0.553},
			{Name: "Bob", CoveragePct: 0.094},
		},
	})
	want := "Alice 55% / Bob 9%"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatAttribution_RoundsPercentages(t *testing.T) {
	got := FormatAttribution(timeline.SpeakerAttribution{
		Candidates: []timeline.SpeakerCandidate{
			{Name: "Alice", CoveragePct: 0.556},
		},
	})
	want := "Alice 56%"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
