package kwin

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

type Window struct {
	ID    string
	Class string
	Title string
}

func Available(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := exec.CommandContext(ctx, "kdotool", "--version").Run()
	return err == nil
}

func ListWindows(ctx context.Context) ([]Window, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "kdotool", "search", ".").Output()
	if err != nil {
		return nil, err
	}

	var windows []Window
	for _, id := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		className := getProp(ctx, id, "getwindowclassname")
		if className == "" {
			continue
		}
		title := getProp(ctx, id, "getwindowname")
		if title == "" {
			title = className
		}
		windows = append(windows, Window{ID: id, Class: className, Title: title})
	}
	return windows, nil
}

func getProp(ctx context.Context, windowID, command string) string {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "kdotool", command, windowID).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
