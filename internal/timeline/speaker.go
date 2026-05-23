package timeline

import (
	"sync"
	"time"
)

type SpeakerChange struct {
	Time time.Time
	Name string // empty string means silence (no speaker)
}

type SpeakerTimeline struct {
	mu        sync.Mutex
	changes   []SpeakerChange
	maxAgeSec float64
}

func NewSpeakerTimeline(maxAgeSecs int) *SpeakerTimeline {
	return &SpeakerTimeline{maxAgeSec: float64(maxAgeSecs)}
}

func (t *SpeakerTimeline) Append(ts time.Time, name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.changes = append(t.changes, SpeakerChange{Time: ts, Name: name})
	t.evict()
}

func (t *SpeakerTimeline) SpeakersIn(start, end time.Time) []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	var result []string
	seen := make(map[string]bool)
	var activeAtStart string

	for _, c := range t.changes {
		if !c.Time.After(start) {
			activeAtStart = c.Name
		} else if !c.Time.After(end) {
			if c.Name != "" && !seen[c.Name] {
				seen[c.Name] = true
				result = append(result, c.Name)
			}
		} else {
			break
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

type ParticipantSet struct {
	mu    sync.Mutex
	names map[string]struct{}
}

func NewParticipantSet() *ParticipantSet {
	return &ParticipantSet{names: make(map[string]struct{})}
}

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

func (p *ParticipantSet) GetAll() map[string]struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	result := make(map[string]struct{}, len(p.names))
	for name := range p.names {
		result[name] = struct{}{}
	}
	return result
}

func (p *ParticipantSet) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.names = make(map[string]struct{})
}
