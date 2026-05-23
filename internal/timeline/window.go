package timeline

import (
	"sync"
	"time"
)

type WindowEvent struct {
	Time   time.Time
	Title  string
	Action string // "opened", "closed", "renamed", "active"
}

type WindowTimeline struct {
	mu        sync.Mutex
	events    []WindowEvent
	maxAgeSec float64
}

func NewWindowTimeline(maxAgeSecs int) *WindowTimeline {
	return &WindowTimeline{maxAgeSec: float64(maxAgeSecs)}
}

func (w *WindowTimeline) Append(ts time.Time, title, action string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.events = append(w.events, WindowEvent{Time: ts, Title: title, Action: action})
	w.evict()
}

func (w *WindowTimeline) EventsBetween(start, end time.Time) []WindowEvent {
	w.mu.Lock()
	defer w.mu.Unlock()

	var result []WindowEvent
	for _, e := range w.events {
		if !e.Time.Before(start) && !e.Time.After(end) {
			result = append(result, e)
		}
	}
	return result
}

func (w *WindowTimeline) CurrentMeeting() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	for i := len(w.events) - 1; i >= 0; i-- {
		e := w.events[i]
		if e.Action == "closed" {
			return ""
		}
		if e.Action == "opened" || e.Action == "renamed" || e.Action == "active" {
			return e.Title
		}
	}
	return ""
}

func (w *WindowTimeline) evict() {
	if len(w.events) == 0 {
		return
	}
	cutoff := w.events[len(w.events)-1].Time
	i := 0
	for i < len(w.events) {
		age := cutoff.Sub(w.events[i].Time).Seconds()
		if age > w.maxAgeSec {
			i++
		} else {
			break
		}
	}
	if i > 0 {
		w.events = w.events[i:]
	}
}
