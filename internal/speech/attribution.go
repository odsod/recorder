package speech

import (
	"fmt"
	"math"
	"strings"

	"github.com/odsod/recorder/internal/timeline"
)

// FormatAttribution renders speaker coverage for transcript prefixes.
func FormatAttribution(attribution timeline.SpeakerAttribution) string {
	if len(attribution.Candidates) == 0 {
		return ""
	}
	parts := make([]string, 0, len(attribution.Candidates))
	for _, candidate := range attribution.Candidates {
		parts = append(parts, fmt.Sprintf("%s %.0f%%", candidate.Name, math.Round(candidate.CoveragePct*100)))
	}
	return strings.Join(parts, " / ")
}
