package timeline

import (
	"testing"
	"time"
)

func t(s string) time.Time {
	ts, _ := time.Parse("15:04:05", s)
	return ts
}

func TestSpeakerTimeline_SingleSpeakerInWindow(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	tl.Append(t("09:00:00"), "Alice")
	tl.Append(t("09:00:30"), "")

	result := tl.SpeakersIn(t("09:00:00"), t("09:00:25"))
	assertStrings(tt, result, []string{"Alice"})
}

func TestSpeakerTimeline_MultipleSpeakersInWindow(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	tl.Append(t("09:00:00"), "Alice")
	tl.Append(t("09:00:10"), "Bob")
	tl.Append(t("09:00:20"), "")

	result := tl.SpeakersIn(t("09:00:00"), t("09:00:20"))
	assertStrings(tt, result, []string{"Alice", "Bob"})
}

func TestSpeakerTimeline_SpeakerActiveAtStart(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	tl.Append(t("09:00:00"), "Alice")
	tl.Append(t("09:00:30"), "Bob")

	result := tl.SpeakersIn(t("09:00:10"), t("09:00:35"))
	assertStrings(tt, result, []string{"Alice", "Bob"})
}

func TestSpeakerTimeline_NoSpeakersInWindow(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	tl.Append(t("09:00:00"), "")

	result := tl.SpeakersIn(t("09:00:05"), t("09:00:10"))
	assertStrings(tt, result, nil)
}

func TestSpeakerTimeline_SpeakerBeforeWindowCarriesOver(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	tl.Append(t("09:00:00"), "Alice")

	result := tl.SpeakersIn(t("09:00:05"), t("09:00:10"))
	assertStrings(tt, result, []string{"Alice"})
}

func TestSpeakerTimeline_NoneBeforeWindowMeansEmpty(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	tl.Append(t("09:00:00"), "Alice")
	tl.Append(t("09:00:05"), "")

	result := tl.SpeakersIn(t("09:00:10"), t("09:00:15"))
	assertStrings(tt, result, nil)
}

func TestSpeakerTimeline_DedupSameSpeaker(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	tl.Append(t("09:00:00"), "Alice")
	tl.Append(t("09:00:05"), "Alice")
	tl.Append(t("09:00:10"), "Alice")

	result := tl.SpeakersIn(t("09:00:00"), t("09:00:15"))
	assertStrings(tt, result, []string{"Alice"})
}

func TestSpeakerTimeline_Eviction(tt *testing.T) {
	tl := NewSpeakerTimeline(60)
	tl.Append(t("09:00:00"), "Alice")
	tl.Append(t("09:05:00"), "Bob")

	result := tl.SpeakersIn(t("09:00:00"), t("09:00:30"))
	assertStrings(tt, result, nil)

	result = tl.SpeakersIn(t("09:04:30"), t("09:05:00"))
	assertStrings(tt, result, []string{"Bob"})
}

func TestSpeakerTimeline_EmptyTimeline(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	result := tl.SpeakersIn(t("09:00:00"), t("09:00:10"))
	assertStrings(tt, result, nil)
}

func TestSpeakerTimeline_OrderByFirstAppearance(tt *testing.T) {
	tl := NewSpeakerTimeline(600)
	tl.Append(t("09:00:00"), "Bob")
	tl.Append(t("09:00:05"), "Alice")
	tl.Append(t("09:00:10"), "Bob")

	result := tl.SpeakersIn(t("09:00:00"), t("09:00:15"))
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
