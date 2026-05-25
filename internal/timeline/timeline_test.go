package timeline

import (
	"testing"
	"time"
)

func ts(s string) time.Time {
	t, _ := time.Parse("15:04:05", s)
	return t
}

func TestSpeakerTimeline_SingleSpeakerInWindow(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	tl.Append(ts("09:00:00"), "Alice")
	tl.Append(ts("09:00:30"), "")

	result := tl.SpeakersIn(ts("09:00:00"), ts("09:00:25"))
	assertStrings(tt, result, []string{"Alice"})
}

func TestSpeakerTimeline_MultipleSpeakersInWindow(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	tl.Append(ts("09:00:00"), "Alice")
	tl.Append(ts("09:00:10"), "")
	tl.Append(ts("09:00:10"), "Bob")
	tl.Append(ts("09:00:20"), "")

	result := tl.SpeakersIn(ts("09:00:00"), ts("09:00:20"))
	// Both spoke for 10s each; Alice first alphabetically in tie-break isn't
	// guaranteed, but both must be present.
	if len(result) != 2 {
		tt.Fatalf("expected 2 speakers, got %v", result)
	}
	has := map[string]bool{result[0]: true, result[1]: true}
	if !has["Alice"] || !has["Bob"] {
		tt.Fatalf("expected Alice and Bob, got %v", result)
	}
}

func TestSpeakerTimeline_DominantSpeakerFirst(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	// Alice speaks 09:00:00-09:00:05 (5s)
	tl.Append(ts("09:00:00"), "Alice")
	tl.Append(ts("09:00:05"), "")
	// Bob speaks 09:00:05-09:00:20 (15s)
	tl.Append(ts("09:00:05"), "Bob")
	tl.Append(ts("09:00:20"), "")

	result := tl.SpeakersIn(ts("09:00:00"), ts("09:00:20"))
	assertStrings(tt, result, []string{"Bob", "Alice"})
}

func TestSpeakerTimeline_SpeakerActiveAtStart(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	tl.Append(ts("09:00:00"), "Alice")
	tl.Append(ts("09:00:30"), "")
	tl.Append(ts("09:00:30"), "Bob")
	tl.Append(ts("09:00:35"), "")

	// Window starts at 09:00:10, Alice was already speaking.
	// Alice: 09:00:10-09:00:30 = 20s, Bob: 09:00:30-09:00:35 = 5s
	result := tl.SpeakersIn(ts("09:00:10"), ts("09:00:35"))
	assertStrings(tt, result, []string{"Alice", "Bob"})
}

func TestSpeakerTimeline_NoSpeakersInWindow(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	tl.Append(ts("09:00:00"), "")

	result := tl.SpeakersIn(ts("09:00:05"), ts("09:00:10"))
	assertStrings(tt, result, nil)
}

func TestSpeakerTimeline_SpeakerBeforeWindowCarriesOver(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	tl.Append(ts("09:00:00"), "Alice")

	result := tl.SpeakersIn(ts("09:00:05"), ts("09:00:10"))
	assertStrings(tt, result, []string{"Alice"})
}

func TestSpeakerTimeline_NoneBeforeWindowMeansEmpty(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	tl.Append(ts("09:00:00"), "Alice")
	tl.Append(ts("09:00:05"), "")

	result := tl.SpeakersIn(ts("09:00:10"), ts("09:00:15"))
	assertStrings(tt, result, nil)
}

func TestSpeakerTimeline_Eviction(tt *testing.T) {
	tl := NewSpeakerTimeline(60)
	tl.Append(ts("09:00:00"), "Alice")
	tl.Append(ts("09:05:00"), "Bob")

	result := tl.SpeakersIn(ts("09:00:00"), ts("09:00:30"))
	assertStrings(tt, result, nil)

	result = tl.SpeakersIn(ts("09:04:30"), ts("09:05:00"))
	assertStrings(tt, result, []string{"Bob"})
}

func TestSpeakerTimeline_EmptyTimeline(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	result := tl.SpeakersIn(ts("09:00:00"), ts("09:00:10"))
	assertStrings(tt, result, nil)
}

func TestSpeakerTimeline_ConcurrentSpeakers(tt *testing.T) {
	// Simulates multi-speaker timeline: both Alice and Bob start speaking,
	// their events interleave.
	tl := NewSpeakerTimeline(600)
	tl.Append(ts("09:00:00"), "Alice")
	tl.Append(ts("09:00:02"), "Bob")
	// Alice stops
	tl.Append(ts("09:00:10"), "")
	// Bob continues (re-appears after the stop-all)
	tl.Append(ts("09:00:10"), "Bob")
	tl.Append(ts("09:00:15"), "")

	result := tl.SpeakersIn(ts("09:00:00"), ts("09:00:15"))
	// Bob: 2s + 5s = 7s (two spans: 09:00:02-09:00:10 via first clear, 09:00:10-09:00:15)
	// Wait — with "" clearing all, Alice: 09:00:00-09:00:10 = 10s, Bob: 09:00:02-09:00:10 + 09:00:10-09:00:15 = 8+5=13s
	// Actually: first "" at 09:00:10 closes all active (Alice started at 00, Bob at 02).
	// Alice span: 00-10 = 10s. Bob first span: 02-10 = 8s.
	// Then Bob starts again at 10, stops at 15: 5s. Bob total = 13s.
	// Bob > Alice, so Bob first.
	assertStrings(tt, result, []string{"Bob", "Alice"})
}

func TestParticipantSet_InitialUpdate(tt *testing.T) {
	ps := NewParticipantSet()
	newNames := ps.Update(setOf("Alice", "Bob"))
	assertSet(tt, newNames, setOf("Alice", "Bob"))
}

func TestParticipantSet_NoNewNames(tt *testing.T) {
	ps := NewParticipantSet()
	ps.Update(setOf("Alice"))
	newNames := ps.Update(setOf("Alice"))
	if newNames != nil {
		tt.Errorf("expected nil, got %v", newNames)
	}
}

func TestParticipantSet_IncrementalGrowth(tt *testing.T) {
	ps := NewParticipantSet()
	ps.Update(setOf("Alice"))
	newNames := ps.Update(setOf("Alice", "Bob"))
	assertSet(tt, newNames, setOf("Bob"))
}

func TestParticipantSet_GetAll(tt *testing.T) {
	ps := NewParticipantSet()
	ps.Update(setOf("Alice"))
	ps.Update(setOf("Bob"))
	assertSet(tt, ps.GetAll(), setOf("Alice", "Bob"))
}

func TestParticipantSet_Reset(tt *testing.T) {
	ps := NewParticipantSet()
	ps.Update(setOf("Alice"))
	ps.Reset()
	assertSet(tt, ps.GetAll(), setOf())
	newNames := ps.Update(setOf("Alice"))
	assertSet(tt, newNames, setOf("Alice"))
}

func assertStrings(tt *testing.T, got, want []string) {
	tt.Helper()
	if len(got) != len(want) {
		tt.Fatalf("got %v, want %v", got, want)
	}
	for i, g := range got {
		if g != want[i] { //nolint:gosec // bounds guaranteed by length check above
			tt.Fatalf("got %v, want %v", got, want)
		}
	}
}

func setOf(names ...string) map[string]struct{} {
	s := make(map[string]struct{}, len(names))
	for _, n := range names {
		s[n] = struct{}{}
	}
	return s
}

func assertSet(tt *testing.T, got, want map[string]struct{}) {
	tt.Helper()
	if len(got) != len(want) {
		tt.Fatalf("got %v, want %v", got, want)
	}
	for k := range want {
		if _, ok := got[k]; !ok {
			tt.Fatalf("missing %q in got %v", k, got)
		}
	}
}
