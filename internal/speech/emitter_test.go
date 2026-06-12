package speech

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/odsod/recorder/internal/timeline"
	"github.com/odsod/recorder/internal/transcript"
)

func TestEmitter_OneSegmentEmitsOneEvent(t *testing.T) {
	start := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
	emitter := Emitter{Cleaner: staticCleaner{text: "cleaned"}}

	got, err := emitter.Emit(context.Background(), "sys", []Segment{
		{Start: start, End: start.Add(time.Second), Text: "raw"},
	}, nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	want := transcript.Event{Time: start, Type: transcript.Speech, Source: "sys", Text: "cleaned"}
	if !reflect.DeepEqual(got[0], want) {
		t.Fatalf("event = %+v, want %+v", got[0], want)
	}
}

func TestEmitter_MultipleSegmentsEmitMultipleEvents(t *testing.T) {
	start := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
	emitter := Emitter{Cleaner: identityCleaner{}}

	got, err := emitter.Emit(context.Background(), "sys", []Segment{
		{Start: start, End: start.Add(time.Second), Text: "first"},
		{Start: start.Add(2 * time.Second), End: start.Add(3 * time.Second), Text: "second"},
	}, nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 2 || got[0].Text != "first" || got[1].Text != "second" {
		t.Fatalf("events = %+v", got)
	}
}

func TestEmitter_AttachesSpeakerPercentages(t *testing.T) {
	start := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
	lookup := staticSpeakerLookup{attribution: timeline.SpeakerAttribution{
		Candidates: []timeline.SpeakerCandidate{
			{Name: "Alice", CoveragePct: 0.75},
			{Name: "Bob", CoveragePct: 0.25},
		},
	}}
	emitter := Emitter{Cleaner: identityCleaner{}, SpeakerLookup: lookup}

	got, err := emitter.Emit(context.Background(), "sys", []Segment{
		{Start: start, End: start.Add(time.Second), Text: "raw"},
	}, nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got[0].Speaker != "Alice 75% / Bob 25%" {
		t.Fatalf("speaker = %q", got[0].Speaker)
	}
}

func TestEmitter_SysFiltersSoleOwner(t *testing.T) {
	start := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
	lookup := staticSpeakerLookup{attribution: timeline.SpeakerAttribution{
		Candidates: []timeline.SpeakerCandidate{
			{Name: "Oscar", CoveragePct: 1.0},
		},
	}}
	emitter := Emitter{
		Cleaner:       identityCleaner{},
		SpeakerLookup: lookup,
		OwnerName:     "Oscar",
	}

	got, err := emitter.Emit(context.Background(), "sys", []Segment{
		{Start: start, End: start.Add(time.Second), Text: "hello"},
	}, nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got[0].Speaker != "" {
		t.Fatalf("speaker = %q, want empty (owner filtered on sys)", got[0].Speaker)
	}
}

func TestEmitter_SysKeepsOwnerWithOthers(t *testing.T) {
	start := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
	lookup := staticSpeakerLookup{attribution: timeline.SpeakerAttribution{
		Candidates: []timeline.SpeakerCandidate{
			{Name: "Alice", CoveragePct: 0.8},
			{Name: "Oscar", CoveragePct: 0.5},
		},
	}}
	emitter := Emitter{
		Cleaner:       identityCleaner{},
		SpeakerLookup: lookup,
		OwnerName:     "Oscar",
	}

	got, err := emitter.Emit(context.Background(), "sys", []Segment{
		{Start: start, End: start.Add(time.Second), Text: "hello"},
	}, nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got[0].Speaker != "Alice" {
		t.Fatalf("speaker = %q, want just Alice (owner filtered on sys)", got[0].Speaker)
	}
}

func TestEmitter_MicKeepsOwner(t *testing.T) {
	start := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
	lookup := staticSpeakerLookup{attribution: timeline.SpeakerAttribution{
		Candidates: []timeline.SpeakerCandidate{
			{Name: "Oscar", CoveragePct: 1.0},
		},
	}}
	emitter := Emitter{
		Cleaner:       identityCleaner{},
		SpeakerLookup: lookup,
		OwnerName:     "Oscar",
	}

	got, err := emitter.Emit(context.Background(), "mic", []Segment{
		{Start: start, End: start.Add(time.Second), Text: "hello"},
	}, nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got[0].Speaker != "Oscar" {
		t.Fatalf("speaker = %q, want Oscar (owner kept on mic)", got[0].Speaker)
	}
}

func TestEmitter_MicDefaultsToSelfWhenNoSpeaker(t *testing.T) {
	start := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
	lookup := staticSpeakerLookup{attribution: timeline.SpeakerAttribution{}}
	emitter := Emitter{
		Cleaner:       identityCleaner{},
		SpeakerLookup: lookup,
		OwnerName:     "Oscar",
	}

	got, err := emitter.Emit(context.Background(), "mic", []Segment{
		{Start: start, End: start.Add(time.Second), Text: "hello"},
	}, nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got[0].Speaker != "Oscar" {
		t.Fatalf("speaker = %q, want Oscar (mic defaults to self)", got[0].Speaker)
	}
}

func TestEmitter_MicDuplicateSkipped(t *testing.T) {
	start := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
	emitter := Emitter{
		Cleaner: identityCleaner{},
		Deduper: NearbyDeduper{Threshold: 0.6, Tolerance: 5 * time.Second},
	}

	got, err := emitter.Emit(context.Background(), "mic", []Segment{
		{Start: start, End: start.Add(time.Second), Text: "hello from the room"},
	}, []transcript.Event{{Time: start, Text: "hello from the room"}})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d events, want 0", len(got))
	}
}

func TestEmitter_CleanupEmptyFallback(t *testing.T) {
	start := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
	emitter := Emitter{Cleaner: staticCleaner{text: ""}}

	got, err := emitter.Emit(context.Background(), "sys", []Segment{
		{Start: start, End: start.Add(time.Second), Text: "raw"},
	}, nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got[0].Text != "raw" {
		t.Fatalf("text = %q, want raw", got[0].Text)
	}
}

func TestEmitter_CleanupErrorReturnsFallbackEventAndError(t *testing.T) {
	start := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
	cleanupErr := errors.New("cleanup failed")
	emitter := Emitter{Cleaner: staticCleaner{err: cleanupErr}}

	got, err := emitter.Emit(context.Background(), "sys", []Segment{
		{Start: start, End: start.Add(time.Second), Text: "raw"},
	}, nil)

	if len(got) != 1 || got[0].Text != "raw" {
		t.Fatalf("events = %+v", got)
	}
	if err == nil || !strings.Contains(err.Error(), cleanupErr.Error()) {
		t.Fatalf("err = %v, want cleanup error", err)
	}
}

type staticCleaner struct {
	text string
	err  error
}

func (c staticCleaner) Cleanup(context.Context, string, []string) (string, error) {
	return c.text, c.err
}

type identityCleaner struct{}

func (identityCleaner) Cleanup(_ context.Context, text string, _ []string) (string, error) {
	return text, nil
}

type staticSpeakerLookup struct {
	attribution timeline.SpeakerAttribution
}

func (l staticSpeakerLookup) Coverage(
	time.Time,
	time.Time,
	timeline.SpeakerLookupOptions,
) timeline.SpeakerAttribution {
	return l.attribution
}
