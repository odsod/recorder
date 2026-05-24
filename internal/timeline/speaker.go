package timeline

import (
	"sync"
	"time"
)

// SpeakerChange records a speaker transition at a point in time.
type SpeakerChange struct {
	Time time.Time
	Name string // empty string means silence (no speaker)
}

// SpeakerTimeline is a time-indexed log of speaker changes with LRU eviction.
type SpeakerTimeline struct {
	mu        sync.Mutex
	changes   []SpeakerChange
	maxAgeSec float64
}

// NewSpeakerTimeline creates a timeline that evicts entries older than maxAgeSecs.
func NewSpeakerTimeline(maxAgeSecs int) *SpeakerTimeline {
	return &SpeakerTimeline{maxAgeSec: float64(maxAgeSecs)}
}

// Append records a speaker change at the given timestamp.
func (t *SpeakerTimeline) Append(ts time.Time, name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.changes = append(t.changes, SpeakerChange{Time: ts, Name: name})
	t.evict()
}

// SpeakersIn returns speakers active during [start, end], ordered by first appearance.
func (t *SpeakerTimeline) SpeakersIn(start, end time.Time) []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	var result []string
	seen := make(map[string]bool)
	var activeAtStart string

loop:
	for _, c := range t.changes {
		switch {
		case !c.Time.After(start):
			activeAtStart = c.Name
		case !c.Time.After(end):
			if c.Name != "" && !seen[c.Name] {
				seen[c.Name] = true
				result = append(result, c.Name)
			}
		default:
			break loop
		}
	}

	if activeAtStart != "" {
		if seen[activeAtStart] {
			// Remove from current position and prepend
			filtered := make([]string, 0, len(result))
			for _, name := range result {
				if name != activeAtStart {
					filtered = append(filtered, name)
				}
			}
			result = filtered
		}
		result = append([]string{activeAtStart}, result...)
	}

	return result
}

func (t *SpeakerTimeline) evict() {
	if len(t.changes) == 0 {
		return
	}
	cutoff := t.changes[len(t.changes)-1].Time
	i := 0
	for i < len(t.changes) {
		age := cutoff.Sub(t.changes[i].Time).Seconds()
		if age > t.maxAgeSec {
			i++
		} else {
			break
		}
	}
	if i > 0 {
		t.changes = t.changes[i:]
	}
}

// ParticipantSet tracks unique participant names with change detection.
type ParticipantSet struct {
	mu    sync.Mutex
	names map[string]struct{}
}

// NewParticipantSet creates an empty participant set.
func NewParticipantSet() *ParticipantSet {
	return &ParticipantSet{names: make(map[string]struct{})}
}

// Update adds names to the set and returns only newly seen names.
func (p *ParticipantSet) Update(names map[string]struct{}) map[string]struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	newNames := make(map[string]struct{})
	for name := range names {
		if _, exists := p.names[name]; !exists {
			newNames[name] = struct{}{}
			p.names[name] = struct{}{}
		}
	}
	if len(newNames) == 0 {
		return nil
	}
	return newNames
}

// GetAll returns a copy of all known participant names.
func (p *ParticipantSet) GetAll() map[string]struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	result := make(map[string]struct{}, len(p.names))
	for name := range p.names {
		result[name] = struct{}{}
	}
	return result
}

// Reset clears all tracked participants.
func (p *ParticipantSet) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.names = make(map[string]struct{})
}
