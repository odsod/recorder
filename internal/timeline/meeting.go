package timeline

import (
	"sync"
	"time"
)

type MeetingState struct {
	mu        sync.Mutex
	current   string
	changed   bool
	changedAt time.Time
}

func NewMeetingState() *MeetingState {
	return &MeetingState{}
}

func (m *MeetingState) Set(title string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if title != m.current {
		m.current = title
		m.changed = true
		m.changedAt = time.Now()
	}
}

func (m *MeetingState) Consume() (string, time.Time, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.changed {
		return "", time.Time{}, false
	}
	m.changed = false
	return m.current, m.changedAt, true
}
