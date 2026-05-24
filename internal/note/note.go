package note

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/odsod/recorder/internal/config"
	"github.com/odsod/recorder/internal/recorder"
	"github.com/odsod/recorder/internal/transcript"
)

// Run appends a user note to today's transcript.
func Run(cfg config.Config, args []string) error {
	var text string
	var err error
	if len(args) > 0 {
		text = strings.Join(args, " ")
	} else {
		text, err = stdinPrompt()
		if err != nil {
			return err
		}
	}

	lines := noteLines(text)
	if len(lines) == 0 {
		return nil
	}

	w := recorder.NewTranscriptWriter(cfg.Transcript.OutputDir)
	now := time.Now()
	for _, line := range lines {
		w.AppendEvent(transcript.Event{Time: now, Type: transcript.Note, Text: line})
	}
	return nil
}

func stdinPrompt() (string, error) {
	fmt.Print("Note: ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return scanner.Text(), nil
	}
	return "", scanner.Err()
}

func noteLines(text string) []string {
	var lines []string
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
