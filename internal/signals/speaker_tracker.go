package signals

import (
	"slices"
	"sort"
	"time"
)

// SpeakerTransition is a debounced active/inactive transition for one speaker.
type SpeakerTransition struct {
	Time   time.Time
	Name   string
	Active bool
}

// SpeakerTracker converts noisy raw speaker samples into stable transitions.
type SpeakerTracker struct {
	cfg    SpeakerTrackerConfig
	states map[string]*speakerState
}

// SpeakerTrackerConfig controls debounce behavior.
type SpeakerTrackerConfig struct {
	StartWindow  time.Duration
	StartSamples int
	StopGrace    time.Duration
}

type speakerState struct {
	samples  []time.Time
	lastSeen time.Time
	active   bool
}

// DefaultSpeakerTrackerConfig returns conservative settings for Meet's short indicator flashes.
func DefaultSpeakerTrackerConfig() SpeakerTrackerConfig {
	return SpeakerTrackerConfig{
		StartWindow:  1500 * time.Millisecond,
		StartSamples: 2,
		StopGrace:    2 * time.Second,
	}
}

// NewSpeakerTracker creates a tracker with the given config.
func NewSpeakerTracker(cfg SpeakerTrackerConfig) *SpeakerTracker {
	if cfg.StartWindow <= 0 {
		cfg.StartWindow = 1500 * time.Millisecond
	}
	if cfg.StartSamples <= 0 {
		cfg.StartSamples = 2
	}
	if cfg.StopGrace <= 0 {
		cfg.StopGrace = 2 * time.Second
	}
	return &SpeakerTracker{
		cfg:    cfg,
		states: make(map[string]*speakerState),
	}
}

// Observe ingests one raw poll sample and returns stable transitions.
func (t *SpeakerTracker) Observe(at time.Time, participants []ParticipantState) []SpeakerTransition {
	seenNames := make(map[string]struct{})
	for _, p := range participants {
		if p.Name == "" {
			continue
		}
		seenNames[p.Name] = struct{}{}
		state := t.state(p.Name)
		state.trimSamples(at, t.cfg.StartWindow)
		if p.Speaking {
			state.samples = append(state.samples, at)
			state.lastSeen = at
		}
	}

	for name, state := range t.states {
		if _, ok := seenNames[name]; !ok {
			state.trimSamples(at, t.cfg.StartWindow)
		}
	}

	var transitions []SpeakerTransition
	names := make([]string, 0, len(t.states))
	for name := range t.states {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		state := t.states[name]
		if !state.active && len(state.samples) >= t.cfg.StartSamples {
			state.active = true
			transitions = append(transitions, SpeakerTransition{Time: at, Name: name, Active: true})
			continue
		}
		if state.active && !state.lastSeen.IsZero() && at.Sub(state.lastSeen) > t.cfg.StopGrace {
			state.active = false
			state.samples = nil
			transitions = append(transitions, SpeakerTransition{
				Time:   state.lastSeen.Add(t.cfg.StopGrace),
				Name:   name,
				Active: false,
			})
		}
	}

	return transitions
}

func (t *SpeakerTracker) state(name string) *speakerState {
	state := t.states[name]
	if state == nil {
		state = &speakerState{}
		t.states[name] = state
	}
	return state
}

func (s *speakerState) trimSamples(now time.Time, window time.Duration) {
	cutoff := now.Add(-window)
	idx := slices.IndexFunc(s.samples, func(sample time.Time) bool {
		return !sample.Before(cutoff)
	})
	if idx == -1 {
		s.samples = nil
		return
	}
	if idx > 0 {
		s.samples = s.samples[idx:]
	}
}
