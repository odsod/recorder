package speech

import (
	"fmt"
	"math"
	"strings"

	"github.com/odsod/recorder/internal/timeline"
)

// filterOwner removes the owner from sys-channel attribution to avoid
// spurious self-attribution from mic bleed into system audio.
func filterOwner(attr timeline.SpeakerAttribution, owner string) timeline.SpeakerAttribution {
	filtered := make([]timeline.SpeakerCandidate, 0, len(attr.Candidates))
	for _, c := range attr.Candidates {
		if c.Name != owner {
			filtered = append(filtered, c)
		}
	}
	return timeline.SpeakerAttribution{Candidates: filtered}
}

// FormatAttribution renders speaker coverage for transcript prefixes.
// Single speaker: just the name. Multiple speakers: relative percentages summing to ~100%.
func FormatAttribution(attribution timeline.SpeakerAttribution) string {
	if len(attribution.Candidates) == 0 {
		return ""
	}
	if len(attribution.Candidates) == 1 {
		return attribution.Candidates[0].Name
	}
	var totalCoverage float64
	for _, c := range attribution.Candidates {
		totalCoverage += c.CoveragePct
	}
	parts := make([]string, 0, len(attribution.Candidates))
	for _, candidate := range attribution.Candidates {
		pct := candidate.CoveragePct / totalCoverage * 100
		parts = append(parts, fmt.Sprintf("%s %.0f%%", candidate.Name, math.Round(pct)))
	}
	return strings.Join(parts, " / ")
}
