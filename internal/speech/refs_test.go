package speech

import (
	"reflect"
	"testing"
	"time"

	"github.com/odsod/recorder/internal/transcript"
)

func TestSystemReferenceTracker_MicRefsReturnsCurrentEvents(t *testing.T) {
	start := refsStart()
	systemEvents := []transcript.Event{
		{Time: start, Text: "current system"},
	}
	tracker := SystemReferenceTracker{lastSystemText: "previous system"}

	got := tracker.MicRefs(start, systemEvents)

	if !reflect.DeepEqual(got, systemEvents) {
		t.Fatalf("MicRefs() = %+v, want %+v", got, systemEvents)
	}
}

func TestSystemReferenceTracker_MicRefsWithoutPriorReturnsNil(t *testing.T) {
	tracker := SystemReferenceTracker{}

	got := tracker.MicRefs(refsStart(), nil)

	if got != nil {
		t.Fatalf("MicRefs() = %+v, want nil", got)
	}
}

func TestSystemReferenceTracker_MicRefsUsesPriorTextAtStart(t *testing.T) {
	start := refsStart()
	tracker := SystemReferenceTracker{lastSystemText: "previous system"}

	got := tracker.MicRefs(start, nil)
	want := []transcript.Event{{Time: start, Text: "previous system"}}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MicRefs() = %+v, want %+v", got, want)
	}
}

func TestSystemReferenceTracker_UpdateJoinsEventTextInOrder(t *testing.T) {
	start := refsStart()
	tracker := SystemReferenceTracker{}

	tracker.Update([]transcript.Event{
		{Time: start, Text: "first"},
		{Time: start.Add(time.Second), Text: "second"},
	})
	got := tracker.MicRefs(start.Add(time.Minute), nil)
	want := []transcript.Event{{Time: start.Add(time.Minute), Text: "first second"}}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MicRefs() after Update() = %+v, want %+v", got, want)
	}
}

func TestSystemReferenceTracker_UpdateIgnoresEmptyText(t *testing.T) {
	start := refsStart()
	tracker := SystemReferenceTracker{}

	tracker.Update([]transcript.Event{
		{Time: start, Text: ""},
		{Time: start.Add(time.Second), Text: "system"},
		{Time: start.Add(2 * time.Second), Text: ""},
		{Time: start.Add(3 * time.Second), Text: "text"},
	})
	got := tracker.MicRefs(start.Add(time.Minute), nil)
	want := []transcript.Event{{Time: start.Add(time.Minute), Text: "system text"}}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MicRefs() after Update() = %+v, want %+v", got, want)
	}
}

func TestSystemReferenceTracker_EmptyUpdateDoesNotOverwritePriorText(t *testing.T) {
	start := refsStart()
	tracker := SystemReferenceTracker{lastSystemText: "previous system"}

	tracker.Update(nil)
	got := tracker.MicRefs(start, nil)
	want := []transcript.Event{{Time: start, Text: "previous system"}}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MicRefs() after empty Update() = %+v, want %+v", got, want)
	}
}

func refsStart() time.Time {
	return time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
}
