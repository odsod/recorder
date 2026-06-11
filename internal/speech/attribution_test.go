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
	want := "Alice"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatAttribution_MultipleSpeakers(t *testing.T) {
	got := FormatAttribution(timeline.SpeakerAttribution{
		Candidates: []timeline.SpeakerCandidate{
			{Name: "Alice", CoveragePct: 0.75},
			{Name: "Bob", CoveragePct: 0.25},
		},
	})
	want := "Alice 75% / Bob 25%"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatAttribution_RelativePercentages(t *testing.T) {
	got := FormatAttribution(timeline.SpeakerAttribution{
		Candidates: []timeline.SpeakerCandidate{
			{Name: "Alice", CoveragePct: 1.0},
			{Name: "Bob", CoveragePct: 1.0},
		},
	})
	want := "Alice 50% / Bob 50%"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatAttribution_ThreeSpeakersRelative(t *testing.T) {
	got := FormatAttribution(timeline.SpeakerAttribution{
		Candidates: []timeline.SpeakerCandidate{
			{Name: "Alice", CoveragePct: 0.6},
			{Name: "Bob", CoveragePct: 0.3},
			{Name: "Carol", CoveragePct: 0.1},
		},
	})
	want := "Alice 60% / Bob 30% / Carol 10%"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFilterOwner(t *testing.T) {
	attr := timeline.SpeakerAttribution{
		Candidates: []timeline.SpeakerCandidate{
			{Name: "Alice", CoveragePct: 0.8},
			{Name: "Oscar", CoveragePct: 0.5},
		},
	}
	got := filterOwner(attr, "Oscar")
	if len(got.Candidates) != 1 || got.Candidates[0].Name != "Alice" {
		t.Fatalf("got %v, want [Alice]", got.Candidates)
	}
}

func TestFilterOwner_SoleOwner(t *testing.T) {
	attr := timeline.SpeakerAttribution{
		Candidates: []timeline.SpeakerCandidate{
			{Name: "Oscar", CoveragePct: 1.0},
		},
	}
	got := filterOwner(attr, "Oscar")
	if len(got.Candidates) != 0 {
		t.Fatalf("got %v, want empty", got.Candidates)
	}
}
