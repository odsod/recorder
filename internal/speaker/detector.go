package speaker

import (
	"context"
	"log/slog"

	"github.com/odsod/recorder/internal/conference"
	"github.com/odsod/recorder/internal/protocol/cdp"
	"github.com/odsod/recorder/internal/signals"
)

// TabLister lists debuggable browser tabs from Chrome DevTools Protocol.
type TabLister interface {
	ListTabs(ctx context.Context, req cdp.ListTabsRequest) (cdp.ListTabsResponse, error)
}

// Evaluator executes JavaScript expressions in browser tabs via CDP.
type Evaluator interface {
	Evaluate(ctx context.Context, req cdp.EvaluateRequest) (cdp.EvaluateResponse, error)
}

// Detector polls conferencing tabs for participant speaking state.
// It implements a two-phase detection strategy: first discovering which CSS
// class indicates speaking state via snapshot diffing, then using that class
// for efficient cached polling. Discovery requires the candidate class to
// toggle on/off at least togglesRequired times before locking it in.
type Detector struct {
	tabs           TabLister
	evaluator      Evaluator
	ports          []int
	providers      []conference.Provider
	activeWSURL    string
	activeTitle    string
	activeProvider conference.Provider
	speakingClass  string
	prevSnapshot   map[string]map[string]struct{}
	// candidateClass is the current class being validated during discovery.
	candidateClass string
	// candidateToggles counts how many times the candidate has toggled.
	candidateToggles int
	// candidatePresent tracks whether the candidate was present in the last snapshot.
	candidatePresent bool
}

// togglesRequired is the number of on/off transitions a candidate class must
// exhibit before being confirmed as the speaking indicator.
const togglesRequired = 3

// NewDetector creates a speaker detector that polls the given CDP ports using
// the provided conference providers to identify meeting tabs.
func NewDetector(tabs TabLister, evaluator Evaluator, ports []int, providers []conference.Provider) *Detector {
	return &Detector{tabs: tabs, evaluator: evaluator, ports: ports, providers: providers}
}

// Poll checks for active meeting tabs and returns current participant speaking
// state and any meeting changes.
func (d *Detector) Poll(ctx context.Context) (signals.PollResult, error) {
	wsURL, tab, provider, err := d.findMeetingTab(ctx)
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
		d.activeProvider = provider
		d.speakingClass = ""
		d.prevSnapshot = nil
		d.candidateClass = ""
		d.candidateToggles = 0
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
		participants, err = d.pollCached(ctx, wsURL)
	} else {
		participants, err = d.pollDiscovery(ctx, wsURL)
	}
	result.Participants = participants
	return result, err
}

func (d *Detector) findMeetingTab(ctx context.Context) (string, cdp.Tab, conference.Provider, error) {
	for _, port := range d.ports {
		resp, err := d.tabs.ListTabs(ctx, cdp.ListTabsRequest{Port: port})
		if err != nil {
			continue
		}
		for _, tab := range resp.Tabs {
			if tab.Type != "page" {
				continue
			}
			for _, p := range d.providers {
				if p.MatchesURL(tab.URL) {
					if tab.WebSocketDebuggerURL != "" {
						return tab.WebSocketDebuggerURL, tab, p, nil
					}
				}
			}
		}
	}
	return "", cdp.Tab{}, nil, nil
}

func (d *Detector) pollCached(ctx context.Context, wsURL string) ([]signals.ParticipantState, error) {
	js, pollErr := d.activeProvider.PollExpression(d.speakingClass)
	if pollErr != nil {
		d.speakingClass = ""
		return nil, nil //nolint:nilerr // intentional: invalid class resets to discovery
	}
	resp, err := d.evaluator.Evaluate(ctx, cdp.EvaluateRequest{
		WebSocketURL: wsURL, Expression: js,
	})
	if err != nil {
		return nil, err
	}
	if resp.Value == "" {
		return nil, nil
	}

	participants, err := d.activeProvider.ParsePoll(resp.Value)
	if err != nil {
		return nil, err
	}

	result := make([]signals.ParticipantState, len(participants))
	for i, p := range participants {
		result[i] = signals.ParticipantState{Name: p.Name, Speaking: p.Speaking}
	}
	return result, nil
}

func (d *Detector) pollDiscovery(ctx context.Context, wsURL string) ([]signals.ParticipantState, error) {
	resp, err := d.evaluator.Evaluate(ctx, cdp.EvaluateRequest{
		WebSocketURL: wsURL, Expression: d.activeProvider.SnapshotExpression(),
	})
	if err != nil {
		return nil, err
	}
	if resp.Value == "" {
		return nil, nil
	}

	snapshots, err := d.activeProvider.ParseSnapshot(resp.Value)
	if err != nil {
		return nil, err
	}

	current := make(map[string]map[string]struct{})
	var names []string
	for _, snap := range snapshots {
		classSet := make(map[string]struct{})
		for _, c := range snap.Classes {
			classSet[c] = struct{}{}
		}
		current[snap.Name] = classSet
		names = append(names, snap.Name)
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
			// Pick shortest changed class as candidate.
			shortest := ""
			for c := range changed {
				if shortest == "" || len(c) < len(shortest) {
					shortest = c
				}
			}

			if d.candidateClass == "" || d.candidateClass != shortest {
				// New candidate — start tracking it.
				d.candidateClass = shortest
				d.candidateToggles = 0
				d.candidatePresent = d.classPresent(current, shortest)
			} else {
				// Same candidate — check if it toggled.
				nowPresent := d.classPresent(current, shortest)
				if nowPresent != d.candidatePresent {
					d.candidateToggles++
					d.candidatePresent = nowPresent
				}
			}

			if d.candidateToggles >= togglesRequired {
				slog.InfoContext(ctx, "speaking class confirmed",
					"class", d.candidateClass,
					"toggles", d.candidateToggles,
				)
				d.speakingClass = d.candidateClass
				d.candidateClass = ""
				d.candidateToggles = 0
				d.prevSnapshot = current

				result := make([]signals.ParticipantState, len(names))
				for i, name := range names {
					_, speaking := current[name][d.speakingClass]
					result[i] = signals.ParticipantState{Name: name, Speaking: speaking}
				}
				return result, nil
			}
		}
	}

	d.prevSnapshot = current
	result := make([]signals.ParticipantState, len(names))
	for i, name := range names {
		result[i] = signals.ParticipantState{Name: name, Speaking: false}
	}
	return result, nil
}

// classPresent reports whether any participant currently has the given class.
func (d *Detector) classPresent(snapshot map[string]map[string]struct{}, class string) bool {
	for _, classes := range snapshot {
		if _, ok := classes[class]; ok {
			return true
		}
	}
	return false
}
