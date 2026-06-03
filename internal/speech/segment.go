package speech

import (
	"strings"
	"time"

	"github.com/odsod/recorder/internal/protocol/whisper"
)

// Segment is one timestamped speech span ready for cleanup and emission.
type Segment struct {
	Start time.Time
	End   time.Time
	Text  string
}

// FromWhisper converts a Whisper response into wall-clock speech segments.
func FromWhisper(resp whisper.TranscribeResponse, chunkStart, chunkEnd time.Time) []Segment {
	if len(resp.Segments) > 0 {
		segments := make([]Segment, 0, len(resp.Segments))
		for _, segment := range resp.Segments {
			text := strings.TrimSpace(segment.Text)
			if text == "" {
				continue
			}
			start := chunkStart.Add(durationFromSeconds(segment.StartSec))
			end := chunkStart.Add(durationFromSeconds(segment.EndSec))
			if !end.After(start) {
				end = start.Add(time.Second)
			}
			segments = append(segments, Segment{Start: start, End: end, Text: text})
		}
		return segments
	}

	text := strings.TrimSpace(resp.Text)
	if text == "" {
		return nil
	}
	return []Segment{{Start: chunkStart, End: chunkEnd, Text: text}}
}

func durationFromSeconds(seconds float64) time.Duration {
	return time.Duration(seconds * float64(time.Second))
}
