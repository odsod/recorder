package speech

import (
	"strings"
	"time"

	"github.com/odsod/recorder/internal/transcript"
)

// SystemReferenceTracker tracks emitted system text for mic dedup references.
type SystemReferenceTracker struct {
	lastSystemText string
}

// MicRefs returns current system events or a prior system-text fallback.
func (t *SystemReferenceTracker) MicRefs(start time.Time, systemEvents []transcript.Event) []transcript.Event {
	if len(systemEvents) > 0 {
		return systemEvents
	}
	if t.lastSystemText == "" {
		return nil
	}
	return []transcript.Event{{Time: start, Text: t.lastSystemText}}
}

// Update records the text from emitted system events.
func (t *SystemReferenceTracker) Update(systemEvents []transcript.Event) {
	if len(systemEvents) == 0 {
		return
	}
	t.lastSystemText = joinEventText(systemEvents)
}

func joinEventText(events []transcript.Event) string {
	parts := make([]string, 0, len(events))
	for _, e := range events {
		if e.Text != "" {
			parts = append(parts, e.Text)
		}
	}
	return strings.Join(parts, " ")
}
