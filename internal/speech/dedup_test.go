package speech

import (
	"testing"
	"time"

	"github.com/odsod/recorder/internal/transcript"
)

func TestNearbyDeduper_NearbyDuplicate(t *testing.T) {
	start := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
	deduper := NearbyDeduper{Threshold: 0.6, Tolerance: 5 * time.Second}

	got := deduper.IsDuplicate(
		Segment{Start: start, Text: "hello from the meeting room"},
		[]transcript.Event{{Time: start.Add(2 * time.Second), Text: "Hello from the meeting room."}},
	)

	if !got {
		t.Fatal("got false, want true")
	}
}

func TestNearbyDeduper_DistantDuplicateFalse(t *testing.T) {
	start := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
	deduper := NearbyDeduper{Threshold: 0.6, Tolerance: 5 * time.Second}

	got := deduper.IsDuplicate(
		Segment{Start: start, Text: "hello from the meeting room"},
		[]transcript.Event{{Time: start.Add(6 * time.Second), Text: "hello from the meeting room"}},
	)

	if got {
		t.Fatal("got true, want false")
	}
}

func TestNearbyDeduper_TextNonOverlapFalse(t *testing.T) {
	start := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
	deduper := NearbyDeduper{Threshold: 0.6, Tolerance: 5 * time.Second}

	got := deduper.IsDuplicate(
		Segment{Start: start, Text: "hello from the meeting room"},
		[]transcript.Event{{Time: start, Text: "completely unrelated words here"}},
	)

	if got {
		t.Fatal("got true, want false")
	}
}

func TestNearbyDeduper_EmptyReferencesFalse(t *testing.T) {
	deduper := NearbyDeduper{Threshold: 0.6, Tolerance: 5 * time.Second}

	if deduper.IsDuplicate(Segment{Start: time.Now(), Text: "hello"}, nil) {
		t.Fatal("got true, want false")
	}
}
