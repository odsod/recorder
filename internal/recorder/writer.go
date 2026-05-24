package recorder

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/odsod/recorder/internal/transcript"
)

// TranscriptWriter appends events to the daily transcript file.
type TranscriptWriter struct {
	outputDir string
	path      string
	date      string
}

// NewTranscriptWriter creates a writer targeting the given output directory.
func NewTranscriptWriter(outputDir string) *TranscriptWriter {
	return &TranscriptWriter{outputDir: outputDir}
}

// Path returns the current day's transcript file path, creating it if needed.
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

// AppendEvent writes a formatted event line to the transcript file.
func (w *TranscriptWriter) AppendEvent(e transcript.Event) {
	w.appendLine(e.String())
}

func (w *TranscriptWriter) appendLine(line string) {
	f, err := os.OpenFile(w.Path(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		slog.ErrorContext(context.Background(), "transcript open failed",
			"err", err,
		)
		return
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(line + "\n"); err != nil {
		slog.ErrorContext(context.Background(), "transcript write failed",
			"err", err,
		)
	}
}

func (w *TranscriptWriter) initFile() {
	if err := os.MkdirAll(w.outputDir, 0o755); err != nil {
		slog.ErrorContext(context.Background(), "transcript mkdir failed",
			"err", err,
		)
		return
	}
	header := fmt.Sprintf("---\ndate: %s\ntype: recorder-transcript\n---\n\n", w.date)
	if err := os.WriteFile(w.path, []byte(header), 0o644); err != nil {
		slog.ErrorContext(context.Background(), "transcript init failed",
			"err", err,
		)
	}
}
