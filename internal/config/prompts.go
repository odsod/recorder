package config

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

//go:embed prompts/cleanup.md
var defaultCleanupTemplate string

//go:embed prompts/summarize.md
var defaultSummarizeTemplate string

//go:embed prompts/combine.md
var defaultCombineTemplate string

// PromptPathsConfig holds optional file paths that override built-in prompts.
// Empty path uses the embedded default without writing a file.
type PromptPathsConfig struct {
	Cleanup   string `json:"cleanup"`
	Summarize string `json:"summarize"`
	Combine   string `json:"combine"`
}

// Prompts holds resolved system prompt text used at runtime.
type Prompts struct {
	Cleanup   string
	Summarize string
	Combine   string
}

func resolvePrompts(paths PromptPathsConfig, vars PromptVarsConfig) (Prompts, error) {
	data := promptTemplateData(vars)

	cleanup, err := resolvePrompt("cleanup", paths.Cleanup, defaultCleanupTemplate, data)
	if err != nil {
		return Prompts{}, fmt.Errorf("prompts.cleanup: %w", err)
	}
	summarize, err := resolvePrompt("summarize", paths.Summarize, defaultSummarizeTemplate, data)
	if err != nil {
		return Prompts{}, fmt.Errorf("prompts.summarize: %w", err)
	}
	combine, err := resolvePrompt("combine", paths.Combine, defaultCombineTemplate, data)
	if err != nil {
		return Prompts{}, fmt.Errorf("prompts.combine: %w", err)
	}

	return Prompts{
		Cleanup:   cleanup,
		Summarize: summarize,
		Combine:   combine,
	}, nil
}

func resolvePrompt(name, path, defaultTemplate string, data PromptTemplateData) (string, error) {
	tmpl, err := loadPromptTemplate(path, defaultTemplate)
	if err != nil {
		return "", err
	}
	return renderTemplate(name, tmpl, data)
}

func loadPromptTemplate(path, defaultTemplate string) (string, error) {
	if path == "" {
		return defaultTemplate, nil
	}

	content, err := os.ReadFile(path)
	if err == nil {
		return string(content), nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}

	if err := seedPromptFile(path, defaultTemplate); err != nil {
		return "", err
	}
	return defaultTemplate, nil
}

func renderTemplate(name, tmpl string, data PromptTemplateData) (string, error) {
	t, err := template.New(name).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

func seedPromptFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
