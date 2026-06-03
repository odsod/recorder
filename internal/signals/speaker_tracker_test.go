package signals

import (
	"reflect"
	"testing"
	"time"
)

func TestSpeakerTracker_RepeatedFlashesStartSpeaker(t *testing.T) {
	tracker := testTracker()
	start := time.Unix(0, 0)

	got := tracker.Observe(start, participants(speaking("Alice")))
	if len(got) != 0 {
		t.Fatalf("got %v, want no transition after one flash", got)
	}

	got = tracker.Observe(start.Add(750*time.Millisecond), participants(speaking("Alice")))
	assertTransitions(t, got, []SpeakerTransition{
		{Time: start.Add(750 * time.Millisecond), Name: "Alice", Active: true},
	})
}

func TestSpeakerTracker_IgnoresIsolatedBlip(t *testing.T) {
	tracker := testTracker()
	start := time.Unix(0, 0)

	_ = tracker.Observe(start, participants(speaking("Alice")))
	got := tracker.Observe(start.Add(2*time.Second), participants(silent("Alice")))
	if len(got) != 0 {
		t.Fatalf("got %v, want no transition", got)
	}
}

func TestSpeakerTracker_HoldsThroughDropout(t *testing.T) {
	tracker := testTracker()
	start := time.Unix(0, 0)

	_ = tracker.Observe(start, participants(speaking("Alice")))
	_ = tracker.Observe(start.Add(500*time.Millisecond), participants(speaking("Alice")))
	got := tracker.Observe(start.Add(1500*time.Millisecond), participants(silent("Alice")))
	if len(got) != 0 {
		t.Fatalf("got %v, want no stop during grace period", got)
	}
}

func TestSpeakerTracker_StopsAfterGrace(t *testing.T) {
	tracker := testTracker()
	start := time.Unix(0, 0)

	_ = tracker.Observe(start, participants(speaking("Alice")))
	_ = tracker.Observe(start.Add(500*time.Millisecond), participants(speaking("Alice")))
	got := tracker.Observe(start.Add(3*time.Second), participants(silent("Alice")))
	assertTransitions(t, got, []SpeakerTransition{
		{Time: start.Add(2500 * time.Millisecond), Name: "Alice", Active: false},
	})
}

func TestSpeakerTracker_AllowsOverlappingSpeakers(t *testing.T) {
	tracker := testTracker()
	start := time.Unix(0, 0)

	_ = tracker.Observe(start, participants(speaking("Alice"), speaking("Bob")))
	got := tracker.Observe(start.Add(500*time.Millisecond), participants(speaking("Alice"), speaking("Bob")))
	assertTransitions(t, got, []SpeakerTransition{
		{Time: start.Add(500 * time.Millisecond), Name: "Alice", Active: true},
		{Time: start.Add(500 * time.Millisecond), Name: "Bob", Active: true},
	})
}

func testTracker() *SpeakerTracker {
	return NewSpeakerTracker(SpeakerTrackerConfig{
		StartWindow:  1500 * time.Millisecond,
		StartSamples: 2,
		StopGrace:    2 * time.Second,
	})
}

func speaking(name string) ParticipantState {
	return ParticipantState{Name: name, Speaking: true}
}

func silent(name string) ParticipantState {
	return ParticipantState{Name: name}
}

func participants(states ...ParticipantState) []ParticipantState {
	return states
}

func assertTransitions(t *testing.T, got, want []SpeakerTransition) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
