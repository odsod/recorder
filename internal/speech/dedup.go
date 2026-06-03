package speech

import (
	"time"

	"github.com/odsod/recorder/internal/transcribe"
	"github.com/odsod/recorder/internal/transcript"
)

// Deduper decides whether a speech segment duplicates nearby reference events.
type Deduper interface {
	IsDuplicate(segment Segment, refs []transcript.Event) bool
}

// NearbyDeduper compares text only against references near the segment start.
type NearbyDeduper struct {
	Threshold float64
	Tolerance time.Duration
}

// IsDuplicate reports whether segment text overlaps a nearby reference event.
func (d NearbyDeduper) IsDuplicate(segment Segment, refs []transcript.Event) bool {
	for _, e := range refs {
		if !nearby(segment.Start, e.Time, d.Tolerance) {
			continue
		}
		if transcribe.TextsOverlap(e.Text, segment.Text, d.Threshold) {
			return true
		}
	}
	return false
}

func nearby(a, b time.Time, tolerance time.Duration) bool {
	diff := a.Sub(b)
	if diff < 0 {
		diff = -diff
	}
	return diff <= tolerance
}
