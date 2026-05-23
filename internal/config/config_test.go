package config

import (
	"path/filepath"
	"testing"
)

func TestExpandHome(t *testing.T) {
	home := HomeDir()
	tests := []struct {
		input string
		want  string
	}{
		{"~/foo/bar", filepath.Join(home, "foo/bar")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"", ""},
	}
	for _, tc := range tests {
		got := expandHome(tc.input)
		if got != tc.want {
			t.Errorf("expandHome(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestEnvDir_Fallback(t *testing.T) {
	got := envDir("NONEXISTENT_XDG_VAR_12345", "/fallback/path")
	if got != "/fallback/path" {
		t.Errorf("envDir with unset var = %q, want /fallback/path", got)
	}
}

func TestEnvDir_RejectsRelativePath(t *testing.T) {
	t.Setenv("TEST_XDG_RELATIVE", "relative/path")
	got := envDir("TEST_XDG_RELATIVE", "/fallback")
	if got != "/fallback" {
		t.Errorf("envDir with relative path = %q, want /fallback", got)
	}
}

func TestEnvDir_AcceptsAbsolutePath(t *testing.T) {
	t.Setenv("TEST_XDG_ABS", "/custom/config")
	got := envDir("TEST_XDG_ABS", "/fallback")
	if got != "/custom/config" {
		t.Errorf("envDir with absolute path = %q, want /custom/config", got)
	}
}

func TestEnvDir_ExpandsTilde(t *testing.T) {
	home := HomeDir()
	t.Setenv("TEST_XDG_TILDE", "~/.config/custom")
	got := envDir("TEST_XDG_TILDE", "/fallback")
	want := filepath.Join(home, ".config/custom")
	if got != want {
		t.Errorf("envDir with tilde = %q, want %q", got, want)
	}
}

func TestDefaults_HasReasonableValues(t *testing.T) {
	cfg := defaults()
	if cfg.Whisper.URL == "" {
		t.Error("whisper URL should have default")
	}
	if cfg.LLM.URL == "" {
		t.Error("LLM URL should have default")
	}
	if cfg.Transcript.OutputDir == "" {
		t.Error("transcript output dir should have default")
	}
	if cfg.Segments.OutputDir == "" {
		t.Error("segments output dir should have default")
	}
	if cfg.Dedup.Threshold == 0 {
		t.Error("dedup threshold should have default")
	}
	if cfg.Signals.SilenceThresholdS == 0 {
		t.Error("silence threshold should have default")
	}
	if cfg.Signals.CDPPorts == nil {
		t.Error("CDP ports should be initialized (empty slice, not nil)")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/nonexistent/path/that/does/not/exist")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with missing config should not error, got: %v", err)
	}
	if cfg.Whisper.URL == "" {
		t.Error("should return defaults when config file missing")
	}
}
