package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func renderedCleanupPrompt(t *testing.T, vars PromptVarsConfig) string {
	t.Helper()
	got, err := renderTemplate("cleanup", defaultCleanupTemplate, promptTemplateData(vars))
	if err != nil {
		t.Fatalf("renderTemplate() error = %v", err)
	}
	return got
}

func TestRenderTemplate_DefaultVars(t *testing.T) {
	vars := defaultPromptVars()
	cleanup := renderedCleanupPrompt(t, vars)

	if !strings.Contains(cleanup, "Swedish or English") {
		t.Error("expected default languages in cleanup prompt")
	}
	if !strings.Contains(cleanup, "liksom") {
		t.Error("expected default filler words in cleanup prompt")
	}

	prompts, err := resolvePrompts(PromptPathsConfig{}, vars)
	if err != nil {
		t.Fatalf("resolvePrompts() error = %v", err)
	}
	if !strings.Contains(prompts.Summarize, "software engineer") {
		t.Error("expected default owner role in summarize prompt")
	}
	if !strings.Contains(prompts.Summarize, "a human inbox") {
		t.Error("expected default summary destination in summarize prompt")
	}
	if !strings.Contains(prompts.Combine, "≤8 words") {
		t.Error("expected default title max words in combine prompt")
	}
}

func TestRenderTemplate_CustomVars(t *testing.T) {
	vars := mergePromptVars(PromptVarsConfig{
		Languages: []string{"English"},
		Owner: OwnerPromptVars{
			Role:       "product manager",
			SummaryFor: "weekly notes",
		},
	}, defaultPromptVars())

	prompts, err := resolvePrompts(PromptPathsConfig{}, vars)
	if err != nil {
		t.Fatalf("resolvePrompts() error = %v", err)
	}
	if strings.Contains(prompts.Cleanup, "Swedish or English") {
		t.Error("custom languages should replace default in cleanup prompt")
	}
	if !strings.Contains(prompts.Summarize, "product manager") {
		t.Error("expected custom owner role in summarize prompt")
	}
	if !strings.Contains(prompts.Summarize, "weekly notes") {
		t.Error("expected custom summary destination in summarize prompt")
	}
}

func TestResolvePrompt_EmptyPathUsesRenderedDefault(t *testing.T) {
	vars := defaultPromptVars()
	data := promptTemplateData(vars)
	want, err := renderTemplate("cleanup", defaultCleanupTemplate, data)
	if err != nil {
		t.Fatalf("renderTemplate() error = %v", err)
	}

	got, err := resolvePrompt("cleanup", "", defaultCleanupTemplate, data)
	if err != nil {
		t.Fatalf("resolvePrompt() error = %v", err)
	}
	if got != want {
		t.Fatal("expected rendered default when path is empty")
	}
}

func TestResolvePrompt_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.txt")
	custom := "custom prompt for {{ .Owner.Role }}"
	if err := os.WriteFile(path, []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}

	vars := defaultPromptVars()
	got, err := resolvePrompt("cleanup", path, defaultCleanupTemplate, promptTemplateData(vars))
	if err != nil {
		t.Fatalf("resolvePrompt() error = %v", err)
	}
	if got != "custom prompt for software engineer" {
		t.Fatalf("got %q", got)
	}
}

func TestResolvePrompt_SeedsRawTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "cleanup.md")
	vars := defaultPromptVars()
	data := promptTemplateData(vars)

	got, err := resolvePrompt("cleanup", path, defaultCleanupTemplate, data)
	if err != nil {
		t.Fatalf("resolvePrompt() error = %v", err)
	}
	if !strings.Contains(got, "Swedish or English") {
		t.Fatal("expected rendered default after seeding")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("seeded file missing: %v", err)
	}
	if !strings.Contains(string(content), "{{ .LanguagesOr }}") {
		t.Fatal("seeded file should contain raw template placeholders")
	}
}

func TestRenderTemplate_InvalidSyntax(t *testing.T) {
	_, err := renderTemplate("bad", "{{ .Missing", promptTemplateData(defaultPromptVars()))
	if err == nil {
		t.Fatal("expected error for invalid template syntax")
	}
}

func TestLoad_ResolvesPromptsWithoutConfigFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !strings.Contains(cfg.Prompts.Cleanup, "speech transcript cleanup") {
		t.Error("expected resolved cleanup prompt")
	}
	if !strings.Contains(cfg.Prompts.Summarize, "ambient meeting recordings") {
		t.Error("expected resolved summarize prompt")
	}
}

func TestLoad_PromptVarsOverride(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)

	configPath := filepath.Join(root, "recorder", "config.json")
	configJSON := `{"promptVars":{"owner":{"role":"product manager","summaryFor":"weekly notes"}}}`
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(configJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !strings.Contains(cfg.Prompts.Summarize, "product manager") {
		t.Error("expected promptVars override in summarize prompt")
	}
	if !strings.Contains(cfg.Prompts.Summarize, "weekly notes") {
		t.Error("expected promptVars override for summary destination")
	}
}

func TestLoad_SeedsConfiguredPromptPath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)

	promptPath := filepath.Join(root, "recorder", "prompts", "cleanup.md")
	configPath := filepath.Join(root, "recorder", "config.json")
	configJSON := `{"prompts":{"cleanup":"` + promptPath + `"}}`
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(configJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !strings.Contains(cfg.Prompts.Cleanup, "Swedish or English") {
		t.Fatal("expected rendered default after seeding")
	}
	if _, err := os.Stat(promptPath); err != nil {
		t.Fatalf("expected seeded prompt file: %v", err)
	}
	seeded, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(seeded), "{{ .LanguagesOr }}") {
		t.Fatal("seeded file should be raw template")
	}
}

func TestMergePromptVars_PartialOverride(t *testing.T) {
	merged := mergePromptVars(PromptVarsConfig{
		Languages: []string{"English"},
	}, defaultPromptVars())

	if len(merged.Languages) != 1 || merged.Languages[0] != "English" {
		t.Errorf("languages = %v, want [English]", merged.Languages)
	}
	if merged.Owner.Role != "software engineer" {
		t.Errorf("owner.role = %q, want default preserved", merged.Owner.Role)
	}
	if merged.TitleMaxWords != 8 {
		t.Errorf("titleMaxWords = %d, want default 8", merged.TitleMaxWords)
	}
}

func TestJoinOr(t *testing.T) {
	tests := []struct {
		items []string
		want  string
	}{
		{nil, ""},
		{[]string{"English"}, "English"},
		{[]string{"Swedish", "English"}, "Swedish or English"},
		{[]string{"A", "B", "C"}, "A, B or C"},
	}
	for _, tc := range tests {
		if got := joinOr(tc.items); got != tc.want {
			t.Errorf("joinOr(%v) = %q, want %q", tc.items, got, tc.want)
		}
	}
}
