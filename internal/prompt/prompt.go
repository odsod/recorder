package prompt

import (
	"fmt"
	"io"
	"strings"

	"github.com/odsod/recorder/internal/config"
)

// Run prints resolved system prompts to w for debugging.
// Optional args select one prompt: cleanup, summarize, combine.
func Run(cfg config.Config, args []string, w io.Writer) error {
	names, err := selectedPrompts(args)
	if err != nil {
		return err
	}

	for i, name := range names {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "=== %s ===\n", name); err != nil {
			return err
		}
		text := promptText(cfg.Prompts, name)
		if _, err := fmt.Fprint(w, text); err != nil {
			return err
		}
		if !strings.HasSuffix(text, "\n") {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
	}
	return nil
}

func selectedPrompts(args []string) ([]string, error) {
	all := []string{"cleanup", "summarize", "combine"}
	if len(args) == 0 {
		return all, nil
	}

	selected := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "cleanup", "summarize", "combine":
			selected = append(selected, arg)
		default:
			return nil, fmt.Errorf("unknown prompt %q (want cleanup, summarize, or combine)", arg)
		}
	}
	return selected, nil
}

func promptText(prompts config.Prompts, name string) string {
	switch name {
	case "cleanup":
		return prompts.Cleanup
	case "summarize":
		return prompts.Summarize
	case "combine":
		return prompts.Combine
	default:
		return ""
	}
}
