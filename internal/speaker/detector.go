package speaker

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/odsod/recorder/internal/protocol/cdp"
	"github.com/odsod/recorder/internal/signals"
)

var validCSSClass = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type TabLister interface {
	ListTabs(ctx context.Context, req cdp.ListTabsRequest) (cdp.ListTabsResponse, error)
}

type Evaluator interface {
	Evaluate(ctx context.Context, req cdp.EvaluateRequest) (cdp.EvaluateResponse, error)
}

type Detector struct {
	tabs           TabLister
	evaluator      Evaluator
	ports          []int
	activeWSURL    string
	activeTitle    string
	activePlatform *PlatformConfig
	speakingClass  string
	prevSnapshot   map[string]map[string]struct{}
}

func NewDetector(tabs TabLister, evaluator Evaluator, ports []int) *Detector {
	return &Detector{tabs: tabs, evaluator: evaluator, ports: ports}
}

func (d *Detector) Poll(ctx context.Context) (signals.PollResult, error) {
	wsURL, tab, platform, err := d.findMeetingTab(ctx)
	if err != nil {
		return signals.PollResult{}, err
	}

	var result signals.PollResult

	if wsURL != d.activeWSURL {
		if wsURL == "" && d.activeWSURL != "" {
			result.MeetingChange = &signals.MeetingChange{Title: ""}
		} else if wsURL != "" {
			title := tab.Title
			if title == "" {
				title = tab.URL
			}
			result.MeetingChange = &signals.MeetingChange{Title: title}
		}
		d.activeWSURL = wsURL
		d.activeTitle = ""
		d.activePlatform = platform
		d.speakingClass = ""
		d.prevSnapshot = nil
	}

	if wsURL == "" {
		return result, nil
	}

	if tab.Title != d.activeTitle && d.activeTitle != "" {
		result.MeetingChange = &signals.MeetingChange{Title: tab.Title}
	}
	d.activeTitle = tab.Title

	var participants []signals.ParticipantState
	if d.speakingClass != "" {
		participants, err = d.pollCached(ctx, wsURL, platform)
	} else {
		participants, err = d.pollDiscovery(ctx, wsURL, platform)
	}
	result.Participants = participants
	return result, err
}

func (d *Detector) findMeetingTab(ctx context.Context) (string, cdp.Tab, *PlatformConfig, error) {
	for _, port := range d.ports {
		resp, err := d.tabs.ListTabs(ctx, cdp.ListTabsRequest{Port: port})
		if err != nil {
			continue
		}
		for _, tab := range resp.Tabs {
			if tab.Type != "page" {
				continue
			}
			for i := range Platforms {
				if strings.Contains(tab.URL, Platforms[i].URLPattern) {
					if tab.WebSocketDebuggerURL != "" {
						return tab.WebSocketDebuggerURL, tab, &Platforms[i], nil
					}
				}
			}
		}
	}
	return "", cdp.Tab{}, nil, nil
}

func (d *Detector) pollCached(
	ctx context.Context,
	wsURL string,
	platform *PlatformConfig,
) ([]signals.ParticipantState, error) {
	if !validCSSClass.MatchString(d.speakingClass) {
		d.speakingClass = ""
		return nil, nil
	}
	js := fmt.Sprintf(platform.PollJSTemplate, d.speakingClass, d.speakingClass)
	resp, err := d.evaluator.Evaluate(ctx, cdp.EvaluateRequest{
		WebSocketURL: wsURL, Expression: js,
	})
	if err != nil {
		return nil, err
	}
	if resp.Value == "" {
		return nil, nil
	}

	var data []struct {
		Name     string `json:"name"`
		Speaking bool   `json:"speaking"`
	}
	if err := json.Unmarshal([]byte(resp.Value), &data); err != nil {
		return nil, err
	}

	result := make([]signals.ParticipantState, len(data))
	for i, p := range data {
		result[i] = signals.ParticipantState{Name: p.Name, Speaking: p.Speaking}
	}
	return result, nil
}

func (d *Detector) pollDiscovery(
	ctx context.Context,
	wsURL string,
	platform *PlatformConfig,
) ([]signals.ParticipantState, error) {
	resp, err := d.evaluator.Evaluate(ctx, cdp.EvaluateRequest{
		WebSocketURL: wsURL, Expression: platform.SnapshotJS,
	})
	if err != nil {
		return nil, err
	}
	if resp.Value == "" {
		return nil, nil
	}

	var data []struct {
		Name    string   `json:"name"`
		Classes []string `json:"classes"`
	}
	if err := json.Unmarshal([]byte(resp.Value), &data); err != nil {
		return nil, err
	}

	current := make(map[string]map[string]struct{})
	var names []string
	for _, tile := range data {
		classSet := make(map[string]struct{})
		for _, c := range tile.Classes {
			classSet[c] = struct{}{}
		}
		current[tile.Name] = classSet
		names = append(names, tile.Name)
	}

	if d.prevSnapshot != nil {
		changed := make(map[string]struct{})
		for name, classes := range current {
			prev := d.prevSnapshot[name]
			for c := range classes {
				if _, ok := prev[c]; !ok {
					changed[c] = struct{}{}
				}
			}
			for c := range prev {
				if _, ok := classes[c]; !ok {
					changed[c] = struct{}{}
				}
			}
		}

		if len(changed) > 0 {
			shortest := ""
			for c := range changed {
				if shortest == "" || len(c) < len(shortest) {
					shortest = c
				}
			}
			d.speakingClass = shortest
			d.prevSnapshot = current

			result := make([]signals.ParticipantState, len(names))
			for i, name := range names {
				_, speaking := current[name][d.speakingClass]
				result[i] = signals.ParticipantState{Name: name, Speaking: speaking}
			}
			return result, nil
		}
	}

	d.prevSnapshot = current
	result := make([]signals.ParticipantState, len(names))
	for i, name := range names {
		result[i] = signals.ParticipantState{Name: name, Speaking: false}
	}
	return result, nil
}
