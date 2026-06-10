package signals

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestSpeakerCollectorPollOnceUpdatesParticipants(t *testing.T) {
	now := time.Unix(100, 0)
	people := &fakeParticipantWriter{}
	collector := newTestCollector(PollResult{
		Participants: participants(silent("Alice"), speaking("Bob")),
	}, people)

	if err := collector.PollOnce(context.Background(), now); err != nil {
		t.Fatalf("PollOnce returned error: %v", err)
	}

	assertNameSet(t, people.updates[0], []string{"Alice", "Bob"})
	if people.resets != 0 {
		t.Fatalf("resets = %d, want 0", people.resets)
	}
}

func TestSpeakerCollectorPollOnceResetsOnMeetingChange(t *testing.T) {
	now := time.Unix(100, 0)
	tracker := testTracker()
	_ = tracker.Observe(now.Add(-500*time.Millisecond), participants(speaking("Alice")))

	people := &fakeParticipantWriter{}
	meetings := &fakeMeetingWriter{}
	timeline := &fakeSpeakerTimelineWriter{}
	collector := &SpeakerCollector{
		Detector: fakeDetector{result: PollResult{
			MeetingChange: &MeetingChange{Title: "Weekly Sync"},
			Participants:  participants(speaking("Alice")),
		}},
		Tracker:  tracker,
		Timeline: timeline,
		People:   people,
		Meetings: meetings,
	}

	if err := collector.PollOnce(context.Background(), now); err != nil {
		t.Fatalf("PollOnce returned error: %v", err)
	}

	if !reflect.DeepEqual(meetings.titles, []string{"Weekly Sync"}) {
		t.Fatalf("meeting titles = %v, want Weekly Sync", meetings.titles)
	}
	if people.resets != 1 {
		t.Fatalf("resets = %d, want 1", people.resets)
	}
	assertAppends(t, timeline.appends, []timelineAppend{{Time: now}})
	if len(timeline.transitions) != 0 {
		t.Fatalf("transitions = %v, want none after tracker reset", timeline.transitions)
	}
	assertNameSet(t, people.updates[0], []string{"Alice"})
}

func TestSpeakerCollectorPollOnceClearsOnMeetingEnd(t *testing.T) {
	now := time.Unix(100, 0)
	people := &fakeParticipantWriter{}
	meetings := &fakeMeetingWriter{}
	timeline := &fakeSpeakerTimelineWriter{}
	collector := &SpeakerCollector{
		Detector: fakeDetector{result: PollResult{
			MeetingChange: &MeetingChange{},
		}},
		Tracker:  testTracker(),
		Timeline: timeline,
		People:   people,
		Meetings: meetings,
	}

	if err := collector.PollOnce(context.Background(), now); err != nil {
		t.Fatalf("PollOnce returned error: %v", err)
	}

	if !reflect.DeepEqual(meetings.titles, []string{""}) {
		t.Fatalf("meeting titles = %v, want empty title", meetings.titles)
	}
	if people.resets != 1 {
		t.Fatalf("resets = %d, want 1", people.resets)
	}
	assertAppends(t, timeline.appends, []timelineAppend{{Time: now}})
}

func TestSpeakerCollectorPollOnceWritesSpeakerTransitions(t *testing.T) {
	start := time.Unix(100, 0)
	timeline := &fakeSpeakerTimelineWriter{}
	detector := &queuedDetector{results: []PollResult{
		{Participants: participants(speaking("Alice"))},
		{Participants: participants(speaking("Alice"))},
		{Participants: participants(silent("Alice"))},
	}}
	collector := &SpeakerCollector{
		Detector: detector,
		Tracker:  testTracker(),
		Timeline: timeline,
		People:   &fakeParticipantWriter{},
		Meetings: &fakeMeetingWriter{},
	}

	times := []time.Time{
		start,
		start.Add(500 * time.Millisecond),
		start.Add(3 * time.Second),
	}
	for _, at := range times {
		if err := collector.PollOnce(context.Background(), at); err != nil {
			t.Fatalf("PollOnce returned error: %v", err)
		}
	}

	assertTransitions(t, timeline.transitions, []SpeakerTransition{
		{Time: start.Add(500 * time.Millisecond), Name: "Alice", Active: true},
		{Time: start.Add(2500 * time.Millisecond), Name: "Alice", Active: false},
	})
}

func TestSpeakerCollectorPollOnceReturnsDetectorError(t *testing.T) {
	wantErr := errors.New("cdp failed")
	people := &fakeParticipantWriter{}
	meetings := &fakeMeetingWriter{}
	timeline := &fakeSpeakerTimelineWriter{}
	collector := &SpeakerCollector{
		Detector: fakeDetector{err: wantErr},
		Tracker:  testTracker(),
		Timeline: timeline,
		People:   people,
		Meetings: meetings,
	}

	err := collector.PollOnce(context.Background(), time.Unix(100, 0))
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if people.resets != 0 || len(people.updates) != 0 || len(meetings.titles) != 0 ||
		len(timeline.appends) != 0 || len(timeline.transitions) != 0 {
		t.Fatalf("collector mutated state on error")
	}
}

func TestSpeakerCollectorPollOnceIgnoresNilParticipants(t *testing.T) {
	people := &fakeParticipantWriter{}
	collector := newTestCollector(PollResult{}, people)

	if err := collector.PollOnce(context.Background(), time.Unix(100, 0)); err != nil {
		t.Fatalf("PollOnce returned error: %v", err)
	}

	if len(people.updates) != 0 {
		t.Fatalf("updates = %v, want none", people.updates)
	}
}

func newTestCollector(result PollResult, people *fakeParticipantWriter) *SpeakerCollector {
	return &SpeakerCollector{
		Detector: fakeDetector{result: result},
		Tracker:  testTracker(),
		Timeline: &fakeSpeakerTimelineWriter{},
		People:   people,
		Meetings: &fakeMeetingWriter{},
	}
}

type fakeDetector struct {
	result PollResult
	err    error
}

func (d fakeDetector) Poll(context.Context) (PollResult, error) {
	return d.result, d.err
}

type queuedDetector struct {
	results []PollResult
	next    int
}

func (d *queuedDetector) Poll(context.Context) (PollResult, error) {
	result := d.results[d.next]
	d.next++
	return result, nil
}

type fakeSpeakerTimelineWriter struct {
	transitions []SpeakerTransition
	appends     []timelineAppend
}

func (w *fakeSpeakerTimelineWriter) SetSpeakerActive(ts time.Time, name string, active bool) {
	w.transitions = append(w.transitions, SpeakerTransition{Time: ts, Name: name, Active: active})
}

func (w *fakeSpeakerTimelineWriter) Append(ts time.Time, name string) {
	w.appends = append(w.appends, timelineAppend{Time: ts, Name: name})
}

type timelineAppend struct {
	Time time.Time
	Name string
}

type fakeParticipantWriter struct {
	updates []map[string]struct{}
	resets  int
}

func (w *fakeParticipantWriter) Update(names map[string]struct{}) map[string]struct{} {
	w.updates = append(w.updates, cloneNameSet(names))
	return names
}

func (w *fakeParticipantWriter) Reset() {
	w.resets++
}

type fakeMeetingWriter struct {
	titles []string
}

func (w *fakeMeetingWriter) Set(title string) {
	w.titles = append(w.titles, title)
}

func cloneNameSet(names map[string]struct{}) map[string]struct{} {
	cloned := make(map[string]struct{}, len(names))
	for name := range names {
		cloned[name] = struct{}{}
	}
	return cloned
}

func assertNameSet(t *testing.T, got map[string]struct{}, want []string) {
	t.Helper()
	wantSet := make(map[string]struct{}, len(want))
	for _, name := range want {
		wantSet[name] = struct{}{}
	}
	if !reflect.DeepEqual(got, wantSet) {
		t.Fatalf("names = %v, want %v", got, wantSet)
	}
}

func assertAppends(t *testing.T, got, want []timelineAppend) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("appends = %v, want %v", got, want)
	}
}
