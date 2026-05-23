package cdp

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var validCSSClass = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type ParticipantState struct {
	Name     string
	Speaking bool
}

type SpeakerDetector struct {
	ports          []int
	activeWSURL    string
	activePlatform *PlatformConfig
	speakingClass  string
	prevSnapshot   map[string]map[string]struct{}
}

func NewSpeakerDetector(ports []int) *SpeakerDetector {
	return &SpeakerDetector{ports: ports}
}

func (d *SpeakerDetector) SpeakingClass() string {
	return d.speakingClass
}

func (d *SpeakerDetector) Poll(ctx context.Context) ([]ParticipantState, error) {
	wsURL, platform, err := d.findMeetingTab(ctx)
	if err != nil {
		return nil, err
	}
	if wsURL == "" {
		return nil, nil
	}

	if wsURL != d.activeWSURL {
		d.activeWSURL = wsURL
		d.activePlatform = platform
		d.speakingClass = ""
		d.prevSnapshot = nil
	}

	if d.speakingClass != "" {
		return d.pollCached(ctx, wsURL, platform)
	}
	return d.pollDiscovery(ctx, wsURL, platform)
}

func (d *SpeakerDetector) findMeetingTab(ctx context.Context) (string, *PlatformConfig, error) {
	for _, port := range d.ports {
		tabs, err := ListTabs(ctx, port)
		if err != nil {
			continue
		}
		for _, tab := range tabs {
			if tab.Type != "page" {
				continue
			}
			for i := range Platforms {
				if strings.Contains(tab.URL, Platforms[i].URLPattern) {
					if tab.WebSocketDebuggerURL != "" {
						return tab.WebSocketDebuggerURL, &Platforms[i], nil
					}
				}
			}
		}
	}
	return "", nil, nil
}

func (d *SpeakerDetector) pollCached(ctx context.Context, wsURL string, platform *PlatformConfig) ([]ParticipantState, error) {
	if !validCSSClass.MatchString(d.speakingClass) {
		d.speakingClass = ""
		return nil, nil
	}
	js := fmt.Sprintf(platform.PollJSTemplate, d.speakingClass, d.speakingClass)
	val, err := Eval(ctx, wsURL, js)
	if err != nil {
		return nil, err
	}
	if val == "" {
		return nil, nil
	}

	var data []struct {
		Name     string `json:"name"`
		Speaking bool   `json:"speaking"`
	}
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, err
	}

	result := make([]ParticipantState, len(data))
	for i, d := range data {
		result[i] = ParticipantState{Name: d.Name, Speaking: d.Speaking}
	}
	return result, nil
}

func (d *SpeakerDetector) pollDiscovery(ctx context.Context, wsURL string, platform *PlatformConfig) ([]ParticipantState, error) {
	val, err := Eval(ctx, wsURL, platform.SnapshotJS)
	if err != nil {
		return nil, err
	}
	if val == "" {
		return nil, nil
	}

	var data []struct {
		Name    string   `json:"name"`
		Classes []string `json:"classes"`
	}
	if err := json.Unmarshal([]byte(val), &data); err != nil {
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
			// Pick the shortest changed class name
			shortest := ""
			for c := range changed {
				if shortest == "" || len(c) < len(shortest) {
					shortest = c
				}
			}
			d.speakingClass = shortest
			d.prevSnapshot = current

			result := make([]ParticipantState, len(names))
			for i, name := range names {
				_, speaking := current[name][d.speakingClass]
				result[i] = ParticipantState{Name: name, Speaking: speaking}
			}
			return result, nil
		}
	}

	d.prevSnapshot = current
	result := make([]ParticipantState, len(names))
	for i, name := range names {
		result[i] = ParticipantState{Name: name, Speaking: false}
	}
	return result, nil
}
