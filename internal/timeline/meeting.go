package timeline

import (
	"sync"
	"time"
)

// MeetingState tracks the current meeting title with one-shot change notification.
type MeetingState struct {
	mu        sync.Mutex
	current   string
	changed   bool
	changedAt time.Time
}

// NewMeetingState creates an empty meeting state.
func NewMeetingState() *MeetingState {
	return &MeetingState{}
}

// Set updates the meeting title and flags the state as changed.
func (m *MeetingState) Set(title string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if title != m.current {
		m.current = title
		m.changed = true
		m.changedAt = time.Now()
	}
}

// Consume returns the current title and clears the changed flag.
func (m *MeetingState) Consume() (string, time.Time, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.changed {
		return "", time.Time{}, false
	}
	m.changed = false
	return m.current, m.changedAt, true
}
