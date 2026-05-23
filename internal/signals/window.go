package signals

import (
	"context"
	"strings"
	"time"

	"github.com/odsod/recorder/internal/kwin"
	"github.com/odsod/recorder/internal/timeline"
)

func RunWindowCollector(
	ctx context.Context,
	windowTimeline *timeline.WindowTimeline,
	patterns []string,
	pollInterval time.Duration,
	log func(string),
) {
	if !kwin.Available(ctx) {
		return
	}

	knownWindows := make(map[string]string)
	initialized := false

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			poll(ctx, windowTimeline, patterns, knownWindows, &initialized)
		}
	}
}

func poll(
	ctx context.Context,
	windowTimeline *timeline.WindowTimeline,
	patterns []string,
	knownWindows map[string]string,
	initialized *bool,
) {
	windows, err := kwin.ListWindows(ctx)
	if err != nil {
		return
	}

	now := time.Now()
	current := make(map[string]string)

	for _, w := range windows {
		match := false
		for _, p := range patterns {
			if strings.Contains(w.Class, p) {
				match = true
				break
			}
		}
		if !match {
			continue
		}
		current[w.ID] = w.Title
	}

	if *initialized {
		for id, title := range current {
			if _, known := knownWindows[id]; !known {
				windowTimeline.Append(now, title, "opened")
			} else if knownWindows[id] != title {
				windowTimeline.Append(now, title, "renamed")
			}
		}
		for id := range knownWindows {
			if _, exists := current[id]; !exists {
				windowTimeline.Append(now, knownWindows[id], "closed")
			}
		}
	} else {
		for _, title := range current {
			windowTimeline.Append(now, title, "active")
		}
	}

	*initialized = true
	for k := range knownWindows {
		delete(knownWindows, k)
	}
	for k, v := range current {
		knownWindows[k] = v
	}
}
