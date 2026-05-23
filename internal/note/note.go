package note

import (
	"os/exec"
	"strings"
	"time"

	"github.com/odsod/recorder/internal/config"
	"github.com/odsod/recorder/internal/transcript"
)

func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	text, err := prompt()
	if err != nil {
		return nil // user cancelled
	}

	lines := noteLines(text)
	if len(lines) == 0 {
		return nil
	}

	t := transcript.New(cfg.Transcript.OutputDir)
	timestamp := time.Now().Format("15:04:05")
	for _, line := range lines {
		t.Append(timestamp, "\U0001f4dd nfo", line, nil)
	}
	return nil
}

func prompt() (string, error) {
	cmd := exec.Command("kdialog",
		"--title", "Note",
		"--geometry", "420",
		"--inputbox", "Note:",
	)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func noteLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
