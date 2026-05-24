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

type MeetingChange struct {
	Title string
}

type PollResult struct {
	Participants  []ParticipantState
	MeetingChange *MeetingChange
}

type SpeakerDetector struct {
	client         *Client
	ports          []int
	activeWSURL    string
	activeTitle    string
	activePlatform *PlatformConfig
	speakingClass  string
	prevSnapshot   map[string]map[string]struct{}
}

func NewSpeakerDetector(client *Client, ports []int) *SpeakerDetector {
	return &SpeakerDetector{client: client, ports: ports}
}

func (d *SpeakerDetector) Poll(ctx context.Context) (PollResult, error) {
	wsURL, tab, platform, err := d.findMeetingTab(ctx)
	if err != nil {
		return PollResult{}, err
	}

	var result PollResult

	if wsURL != d.activeWSURL {
		if wsURL == "" && d.activeWSURL != "" {
			result.MeetingChange = &MeetingChange{Title: ""}
		} else if wsURL != "" {
			title := tab.Title
			if title == "" {
				title = tab.URL
			}
			result.MeetingChange = &MeetingChange{Title: title}
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
		result.MeetingChange = &MeetingChange{Title: tab.Title}
	}
	d.activeTitle = tab.Title

	var participants []ParticipantState
	if d.speakingClass != "" {
		participants, err = d.pollCached(ctx, wsURL, platform)
	} else {
		participants, err = d.pollDiscovery(ctx, wsURL, platform)
	}
	result.Participants = participants
	return result, err
}

func (d *SpeakerDetector) findMeetingTab(ctx context.Context) (string, Tab, *PlatformConfig, error) {
	for _, port := range d.ports {
		tabs, err := d.client.ListTabs(ctx, port)
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
						return tab.WebSocketDebuggerURL, tab, &Platforms[i], nil
					}
				}
			}
		}
	}
	return "", Tab{}, nil, nil
}

func (d *SpeakerDetector) pollCached(
	ctx context.Context,
	wsURL string,
	platform *PlatformConfig,
) ([]ParticipantState, error) {
	if !validCSSClass.MatchString(d.speakingClass) {
		d.speakingClass = ""
		return nil, nil
	}
	js := fmt.Sprintf(platform.PollJSTemplate, d.speakingClass, d.speakingClass)
	val, err := eval(ctx, wsURL, js)
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

func (d *SpeakerDetector) pollDiscovery(
	ctx context.Context,
	wsURL string,
	platform *PlatformConfig,
) ([]ParticipantState, error) {
	val, err := eval(ctx, wsURL, platform.SnapshotJS)
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
