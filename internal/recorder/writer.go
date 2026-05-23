package recorder

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/odsod/recorder/internal/transcript"
)

type TranscriptWriter struct {
	outputDir string
	path      string
	date      string
}

func NewTranscriptWriter(outputDir string) *TranscriptWriter {
	return &TranscriptWriter{outputDir: outputDir}
}

func (w *TranscriptWriter) Path() string {
	today := time.Now().Format("2006-01-02")
	if w.date != today {
		w.date = today
		w.path = filepath.Join(w.outputDir, today+"-recorder.md")
		if _, err := os.Stat(w.path); os.IsNotExist(err) {
			w.initFile()
		}
	}
	return w.path
}

func (w *TranscriptWriter) AppendEvent(e transcript.Event) {
	w.appendLine(e.String())
}

func (w *TranscriptWriter) appendLine(line string) {
	f, err := os.OpenFile(w.Path(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "transcript open: %v\n", err)
		return
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(line + "\n"); err != nil {
		fmt.Fprintf(os.Stderr, "transcript write: %v\n", err)
	}
}

func (w *TranscriptWriter) initFile() {
	if err := os.MkdirAll(w.outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "transcript mkdir: %v\n", err)
		return
	}
	header := fmt.Sprintf("---\ndate: %s\ntype: recorder-transcript\n---\n\n", w.date)
	if err := os.WriteFile(w.path, []byte(header), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "transcript init: %v\n", err)
	}
}
