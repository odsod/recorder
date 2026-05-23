package transcript

import (
	"bufio"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/odsod/recorder/internal/segment"
)

var lineRe = regexp.MustCompile(`^\[(\d{2}:\d{2}:\d{2})\] (.+?) \*\*(\w+)\*\*(.*)$`)

func ParseTranscript(path string) ([]segment.Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var events []segment.Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		m := lineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		ts, err := time.Parse("15:04:05", m[1])
		if err != nil {
			continue
		}

		emoji := m[2]
		tag := m[3]
		text := strings.TrimSpace(m[4])
		// Strip speaker attribution brackets
		if strings.HasPrefix(text, "[") {
			if idx := strings.Index(text, "]"); idx != -1 {
				text = strings.TrimSpace(text[idx+1:])
			}
		}

		events = append(events, segment.Event{
			Time:  ts,
			Tag:   tag,
			Emoji: emoji,
			Text:  text,
		})
	}

	return events, scanner.Err()
}
