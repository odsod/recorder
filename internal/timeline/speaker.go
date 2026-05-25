package timeline

import (
	"sync"
	"time"
)

// SpeakerChange records a speaker transition at a point in time.
type SpeakerChange struct {
	Time time.Time
	Name string // empty string means a speaker stopped
}

// SpeakerTimeline is a time-indexed log of speaker start/stop events with LRU eviction.
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

// SpeakersIn returns speakers active during [start, end], ordered by total
// speaking time (dominant speaker first).
func (t *SpeakerTimeline) SpeakersIn(start, end time.Time) []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Collect all speaker events relevant to the window.
	type span struct {
		name      string
		spanStart time.Time
		spanEnd   time.Time
	}

	// Track which speakers are active leading up to and within the window.
	activeSet := make(map[string]time.Time) // name -> when they started (clamped to window start)
	var spans []span

	for _, c := range t.changes {
		if c.Time.After(end) {
			break
		}
		if !c.Time.After(start) {
			// Events before window: track active state at window start.
			if c.Name != "" {
				activeSet[c.Name] = start
			} else {
				// A stop event before the window — remove all (old single-speaker model)
				// With multi-speaker, "" means one speaker stopped, but we don't know which.
				// Clear all since old entries used "" as "silence".
				activeSet = make(map[string]time.Time)
			}
		} else {
			// Events within the window.
			if c.Name != "" {
				activeSet[c.Name] = c.Time
			} else {
				// Speaker stop — close spans for all currently active speakers
				// that started before this stop event.
				for name, spanStart := range activeSet {
					spans = append(spans, span{name: name, spanStart: spanStart, spanEnd: c.Time})
				}
				activeSet = make(map[string]time.Time)
			}
		}
	}

	// Close any still-active spans at window end.
	for name, spanStart := range activeSet {
		spans = append(spans, span{name: name, spanStart: spanStart, spanEnd: end})
	}

	// Sum duration per speaker.
	durations := make(map[string]time.Duration)
	for _, s := range spans {
		durations[s.name] += s.spanEnd.Sub(s.spanStart)
	}

	// Sort by duration descending.
	type entry struct {
		name     string
		duration time.Duration
	}
	var entries []entry
	for name, dur := range durations {
		entries = append(entries, entry{name, dur})
	}
	for i := range entries {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].duration > entries[i].duration {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	result := make([]string, len(entries))
	for i, e := range entries {
		result[i] = e.name
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
