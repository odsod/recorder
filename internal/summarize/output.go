package summarize

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/odsod/recorder/internal/segment"
)

// WriteSegmentFile writes a segment's summary and transcript to a markdown file.
func WriteSegmentFile(title, summary string, seg segment.Segment, date, outputDir string) (string, error) {
	durationMin := int(seg.End.Sub(seg.Start).Seconds() / 60)
	timeRange := fmt.Sprintf("%s–%s", seg.Start.Format("15:04"), seg.End.Format("15:04"))
	slug := segment.Slugify(title)
	participants := ExtractParticipants(seg)

	var content strings.Builder
	content.WriteString("---\n")
	fmt.Fprintf(&content, "title: %q\n", title)
	fmt.Fprintf(&content, "date: %s\n", date)
	fmt.Fprintf(&content, "time: %q\n", timeRange)
	fmt.Fprintf(&content, "duration: %dm\n", durationMin)
	content.WriteString("type: segment\n")
	fmt.Fprintf(&content, "source: \"[[raw/transcripts/%s-recorder.md]]\"\n", date)
	if len(participants) > 0 {
		pJSON, _ := json.Marshal(participants)
		fmt.Fprintf(&content, "participants: %s\n", string(pJSON))
	}
	content.WriteString("---\n\n")
	content.WriteString(summary)
	content.WriteString("\n\n---\n\n## Transcript\n\n")
	content.WriteString(segment.FormatTranscript(seg))
	content.WriteString("\n")

	filename := fmt.Sprintf("%s-%s-%s.md", date, seg.ID, slug)
	path := filepath.Join(outputDir, filename)
	tmpPath := path + ".tmp"

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(tmpPath, []byte(content.String()), 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", err
	}
	return filename, nil
}
