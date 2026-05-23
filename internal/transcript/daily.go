package transcript

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type DailyTranscript struct {
	outputDir string
	path      string
	date      string
}

func New(outputDir string) *DailyTranscript {
	return &DailyTranscript{outputDir: outputDir}
}

func (dt *DailyTranscript) Path() string {
	today := time.Now().Format("2006-01-02")
	if dt.date != today {
		dt.date = today
		dt.path = filepath.Join(dt.outputDir, today+"-recorder.md")
		if _, err := os.Stat(dt.path); os.IsNotExist(err) {
			dt.initFile()
		}
	}
	return dt.path
}

func (dt *DailyTranscript) Append(timestamp, tag, text string, speakers []string) {
	line := FormatLine(timestamp, tag, text, speakers) + "\n"
	f, err := os.OpenFile(dt.Path(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "transcript open: %v\n", err)
		return
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(line); err != nil {
		fmt.Fprintf(os.Stderr, "transcript write: %v\n", err)
	}
}

func (dt *DailyTranscript) initFile() {
	if err := os.MkdirAll(dt.outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "transcript mkdir: %v\n", err)
		return
	}
	header := fmt.Sprintf("---\ndate: %s\ntype: recorder-transcript\n---\n\n", dt.date)
	if err := os.WriteFile(dt.path, []byte(header), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "transcript init: %v\n", err)
	}
}

func FormatMessage(tag, text string, speakers []string) string {
	emoji, name, _ := strings.Cut(tag, " ")
	var formattedTag string
	if name != "" {
		formattedTag = fmt.Sprintf("%s **%s**", emoji, name)
	} else {
		formattedTag = fmt.Sprintf("**%s**", emoji)
	}

	var sb strings.Builder
	sb.WriteString(formattedTag)
	if len(speakers) > 0 {
		sb.WriteString(" [")
		sb.WriteString(strings.Join(speakers, ", "))
		sb.WriteString("]")
	}
	if text != "" {
		sb.WriteString(" ")
		sb.WriteString(text)
	}
	return strings.TrimRight(sb.String(), " ")
}

func FormatLine(timestamp, tag, text string, speakers []string) string {
	return fmt.Sprintf("[%s] %s", timestamp, FormatMessage(tag, text, speakers))
}
