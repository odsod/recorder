package speech

import (
	"testing"
	"time"

	"github.com/odsod/recorder/internal/protocol/whisper"
)

func TestFromWhisper_UsesVerboseSegments(t *testing.T) {
	start := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Second)

	got := FromWhisper(whisper.TranscribeResponse{
		Text: "full text",
		Segments: []whisper.Segment{
			{StartSec: 1.5, EndSec: 3.25, Text: " first segment "},
			{StartSec: 4, EndSec: 4, Text: "second segment"},
		},
	}, start, end)

	if len(got) != 2 {
		t.Fatalf("got %d segments, want 2", len(got))
	}
	if !got[0].Start.Equal(start.Add(1500*time.Millisecond)) || !got[0].End.Equal(start.Add(3250*time.Millisecond)) {
		t.Fatalf("first segment times = %s-%s", got[0].Start, got[0].End)
	}
	if got[0].Text != "first segment" {
		t.Fatalf("first segment text = %q", got[0].Text)
	}
}

func TestFromWhisper_TextFallback(t *testing.T) {
	start := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Second)

	got := FromWhisper(whisper.TranscribeResponse{Text: " full text "}, start, end)

	if len(got) != 1 {
		t.Fatalf("got %d segments, want 1", len(got))
	}
	if !got[0].Start.Equal(start) || !got[0].End.Equal(end) || got[0].Text != "full text" {
		t.Fatalf("segment = %+v", got[0])
	}
}

func TestFromWhisper_SkipsEmptySegmentText(t *testing.T) {
	start := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)

	got := FromWhisper(whisper.TranscribeResponse{
		Segments: []whisper.Segment{
			{StartSec: 1, EndSec: 2, Text: "  "},
			{StartSec: 3, EndSec: 4, Text: "kept"},
		},
	}, start, start.Add(10*time.Second))

	if len(got) != 1 || got[0].Text != "kept" {
		t.Fatalf("segments = %+v", got)
	}
}

func TestFromWhisper_InvalidDurationFallback(t *testing.T) {
	start := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)

	got := FromWhisper(whisper.TranscribeResponse{
		Segments: []whisper.Segment{
			{StartSec: 4, EndSec: 4, Text: "segment"},
		},
	}, start, start.Add(10*time.Second))

	if len(got) != 1 {
		t.Fatalf("got %d segments, want 1", len(got))
	}
	wantEnd := start.Add(5 * time.Second)
	if !got[0].End.Equal(wantEnd) {
		t.Fatalf("end = %s, want %s", got[0].End, wantEnd)
	}
}
