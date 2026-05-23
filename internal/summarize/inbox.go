package summarize

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/odsod/recorder/internal/segment"
)

func WriteInboxDraft(title, summary string, seg segment.Segment, date, inboxDir string) (string, error) {
	durationMin := int(seg.End.Sub(seg.Start).Seconds() / 60)
	timeRange := fmt.Sprintf("%s–%s", seg.Start.Format("15:04"), seg.End.Format("15:04"))
	slug := segment.Slugify(title)
	participants := ExtractParticipants(seg)

	var content strings.Builder
	content.WriteString("---\n")
	content.WriteString(fmt.Sprintf("title: %q\n", title))
	content.WriteString(fmt.Sprintf("date: %s\n", date))
	content.WriteString(fmt.Sprintf("time: %q\n", timeRange))
	content.WriteString(fmt.Sprintf("duration: %dm\n", durationMin))
	content.WriteString("type: segment\n")
	content.WriteString(fmt.Sprintf("source: \"[[raw/transcripts/%s-recorder.md]]\"\n", date))
	if len(participants) > 0 {
		pJSON, _ := json.Marshal(participants)
		content.WriteString(fmt.Sprintf("participants: %s\n", string(pJSON)))
	}
	content.WriteString("---\n\n")
	content.WriteString(summary)
	content.WriteString("\n\n---\n\n## Transcript\n\n")
	content.WriteString(segment.FormatTranscript(seg))
	content.WriteString("\n")

	filename := fmt.Sprintf("%s-%s-%s.md", date, seg.ID, slug)
	path := filepath.Join(inboxDir, filename)
	tmpPath := path + ".tmp"

	if err := os.MkdirAll(inboxDir, 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(tmpPath, []byte(content.String()), 0644); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", err
	}
	return filename, nil
}

