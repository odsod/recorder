package note

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/odsod/recorder/internal/config"
	"github.com/odsod/recorder/internal/transcript"
)

func Run(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	var text string
	if len(args) > 0 {
		text = strings.Join(args, " ")
	} else {
		text, err = prompt()
		if err != nil {
			return nil
		}
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
	if kdialogAvailable() {
		return kdialogPrompt()
	}
	return stdinPrompt()
}

func kdialogAvailable() bool {
	_, err := exec.LookPath("kdialog")
	return err == nil
}

func kdialogPrompt() (string, error) {
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
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
