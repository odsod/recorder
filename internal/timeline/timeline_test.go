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

	result = tl.SpeakersIn(ts("09:04:30"), ts("09:05:01"))
	assertStrings(tt, result, []string{"Bob"})
}

func TestSpeakerTimeline_EmptyTimeline(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	result := tl.SpeakersIn(ts("09:00:00"), ts("09:00:10"))
	assertStrings(tt, result, nil)
}

func TestSpeakerTimeline_ConcurrentSpeakers(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	tl.SetSpeakerActive(ts("09:00:00"), "Alice", true)
	tl.SetSpeakerActive(ts("09:00:02"), "Bob", true)
	tl.SetSpeakerActive(ts("09:00:10"), "Alice", false)
	tl.SetSpeakerActive(ts("09:00:15"), "Bob", false)

	result := tl.SpeakersIn(ts("09:00:00"), ts("09:00:15"))
	assertStrings(tt, result, []string{"Bob", "Alice"})
}

func TestSpeakerTimeline_WithDurations(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	tl.Append(ts("09:00:00"), "Alice")
	tl.Append(ts("09:00:05"), "")
	tl.Append(ts("09:00:05"), "Bob")
	tl.Append(ts("09:00:20"), "")

	result := tl.SpeakersInWithDurations(ts("09:00:00"), ts("09:00:20"))
	if len(result) != 2 {
		tt.Fatalf("expected 2 speakers, got %v", result)
	}
	if result[0].Name != "Bob" || result[0].Duration != 15*time.Second {
		tt.Errorf("expected Bob 15s first, got %s %v", result[0].Name, result[0].Duration)
	}
	if result[1].Name != "Alice" || result[1].Duration != 5*time.Second {
		tt.Errorf("expected Alice 5s second, got %s %v", result[1].Name, result[1].Duration)
	}
}

func TestSpeakerTimeline_CoverageOverlappingSpeakers(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	tl.SetSpeakerActive(ts("09:00:00"), "Alice", true)
	tl.SetSpeakerActive(ts("09:00:02"), "Bob", true)
	tl.SetSpeakerActive(ts("09:00:08"), "Alice", false)
	tl.SetSpeakerActive(ts("09:00:10"), "Bob", false)

	got := tl.Coverage(ts("09:00:00"), ts("09:00:10"), SpeakerLookupOptions{
		MinCandidatePct:      0.05,
		MinCandidateDuration: 250 * time.Millisecond,
	})

	assertCandidates(tt, got.Candidates, []wantCandidate{
		{Name: "Alice", Coverage: 8 * time.Second, Pct: 0.8},
		{Name: "Bob", Coverage: 8 * time.Second, Pct: 0.8},
	})
}

func TestSpeakerTimeline_CoverageFiltersByLowThreshold(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	tl.SetSpeakerActive(ts("09:00:00"), "Alice", true)
	tl.SetSpeakerActive(ts("09:00:10"), "Alice", false)
	tl.SetSpeakerActive(ts("09:00:09"), "Bob", true)
	tl.SetSpeakerActive(ts("09:00:10"), "Bob", false)

	got := tl.Coverage(ts("09:00:00"), ts("09:00:10"), SpeakerLookupOptions{
		MinCandidatePct:      0.05,
		MinCandidateDuration: 250 * time.Millisecond,
	})

	assertCandidates(tt, got.Candidates, []wantCandidate{
		{Name: "Alice", Coverage: 10 * time.Second, Pct: 1.0},
		{Name: "Bob", Coverage: 1 * time.Second, Pct: 0.1},
	})
}

func TestSpeakerTimeline_CoverageFiltersByDuration(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	tl.SetSpeakerActive(ts("09:00:00"), "Alice", true)
	tl.SetSpeakerActive(ts("09:00:00").Add(100*time.Millisecond), "Alice", false)

	got := tl.Coverage(ts("09:00:00"), ts("09:00:01"), SpeakerLookupOptions{
		MinCandidatePct:      0.05,
		MinCandidateDuration: 250 * time.Millisecond,
	})

	if len(got.Candidates) != 0 {
		tt.Fatalf("got %v, want no candidates", got.Candidates)
	}
}

func TestSpeakerTimeline_CoverageNoActiveSpeakers(tt *testing.T) {
	tl := NewSpeakerTimeline(600)

	got := tl.Coverage(ts("09:00:00"), ts("09:00:10"), SpeakerLookupOptions{
		MinCandidatePct:      0.05,
		MinCandidateDuration: 250 * time.Millisecond,
	})

	if len(got.Candidates) != 0 {
		tt.Fatalf("got %v, want no candidates", got.Candidates)
	}
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

type wantCandidate struct {
	Name     string
	Coverage time.Duration
	Pct      float64
}

func assertCandidates(tt *testing.T, got []SpeakerCandidate, want []wantCandidate) {
	tt.Helper()
	if len(got) != len(want) {
		tt.Fatalf("got %v, want %v", got, want)
	}
	for i, candidate := range got {
		w := want[i]
		if candidate.Name != w.Name {
			tt.Fatalf("candidate[%d].Name = %q, want %q", i, candidate.Name, w.Name)
		}
		if candidate.Coverage != w.Coverage {
			tt.Fatalf("candidate[%d].Coverage = %s, want %s", i, candidate.Coverage, w.Coverage)
		}
		if diff := candidate.CoveragePct - w.Pct; diff < -0.0001 || diff > 0.0001 {
			tt.Fatalf("candidate[%d].CoveragePct = %.4f, want %.4f", i, candidate.CoveragePct, w.Pct)
		}
	}
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
