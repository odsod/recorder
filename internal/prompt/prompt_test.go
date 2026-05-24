package prompt

import (
	"bytes"
	"strings"
	"testing"

	"github.com/odsod/recorder/internal/config"
)

func TestRun_AllPrompts(t *testing.T) {
	cfg := config.Config{
		Prompts: config.Prompts{
			Cleanup:   "cleanup text",
			Summarize: "summarize text",
			Combine:   "combine text",
		},
	}

	var buf bytes.Buffer
	if err := Run(cfg, nil, &buf); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	out := buf.String()
	for _, want := range []string{"=== cleanup ===", "cleanup text", "=== summarize ===", "summarize text", "=== combine ===", "combine text"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestRun_SelectPrompt(t *testing.T) {
	cfg := config.Config{
		Prompts: config.Prompts{Cleanup: "only cleanup"},
	}

	var buf bytes.Buffer
	if err := Run(cfg, []string{"cleanup"}, &buf); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if buf.String() != "=== cleanup ===\nonly cleanup\n" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestRun_UnknownPrompt(t *testing.T) {
	err := Run(config.Config{}, []string{"bad"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for unknown prompt")
	}
}
