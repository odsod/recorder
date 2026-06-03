package timeline

import "sync"

// ParticipantSet tracks known participants for the current meeting.
type ParticipantSet struct {
	mu    sync.Mutex
	names map[string]struct{}
}

// NewParticipantSet creates an empty participant set.
func NewParticipantSet() *ParticipantSet {
	return &ParticipantSet{names: make(map[string]struct{})}
}

// Update merges names into the set and returns only newly seen names.
func (p *ParticipantSet) Update(names map[string]struct{}) map[string]struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	newNames := make(map[string]struct{})
	for name := range names {
		if name == "" {
			continue
		}
		if _, ok := p.names[name]; ok {
			continue
		}
		p.names[name] = struct{}{}
		newNames[name] = struct{}{}
	}
	if len(newNames) == 0 {
		return nil
	}
	return newNames
}

// GetAll returns a snapshot of all known participants.
func (p *ParticipantSet) GetAll() map[string]struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	all := make(map[string]struct{}, len(p.names))
	for name := range p.names {
		all[name] = struct{}{}
	}
	return all
}

// Reset clears all known participants.
func (p *ParticipantSet) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.names = make(map[string]struct{})
}
