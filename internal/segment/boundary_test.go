package segment

import (
	"testing"
	"time"

	"github.com/odsod/recorder/internal/transcript"
)

func t(s string) time.Time {
	ts, _ := time.Parse("15:04:05", s)
	return ts
}

func speech(ts string, text string) transcript.Event {
	return transcript.Event{Time: t(ts), Type: transcript.Speech, Source: "sys", Text: text}
}

func TestDetectBoundaries_SilenceGap(tt *testing.T) {
	events := []transcript.Event{
		speech("09:00:00", "hello"),
		speech("09:01:00", "world"),
		speech("09:10:00", "after gap"),
	}
	boundaries := DetectBoundaries(events, t("09:11:00"))
	if len(boundaries) != 1 {
		tt.Fatalf("expected 1 boundary, got %d", len(boundaries))
	}
	if boundaries[0].Time != t("09:01:00") {
		tt.Errorf("boundary at %v, want 09:01:00", boundaries[0].Time)
	}
}

func TestDetectBoundaries_NoGap(tt *testing.T) {
	events := []transcript.Event{
		speech("09:00:00", "hello"),
		speech("09:01:00", "world"),
		speech("09:02:00", "foo"),
	}
	boundaries := DetectBoundaries(events, t("09:03:00"))
	if len(boundaries) != 0 {
		tt.Fatalf("expected 0 boundaries, got %d", len(boundaries))
	}
}

func TestDetectBoundaries_TrailingSilence(tt *testing.T) {
	events := []transcript.Event{
		speech("09:00:00", "hello"),
	}
	now := t("09:00:00").Add(6 * time.Minute)
	boundaries := DetectBoundaries(events, now)
	if len(boundaries) != 1 {
		tt.Fatalf("expected 1 boundary for trailing silence, got %d", len(boundaries))
	}
}

func TestDetectBoundaries_Pin(tt *testing.T) {
	events := []transcript.Event{
		speech("09:00:00", "hello"),
		speech("09:01:00", "world"),
		{Time: t("09:01:30"), Type: transcript.Pin},
	}
	boundaries := DetectBoundaries(events, t("09:02:00"))
	if len(boundaries) != 1 {
		tt.Fatalf("expected 1 boundary for pin, got %d", len(boundaries))
	}
	if boundaries[0].Reason != "pin" {
		tt.Errorf("expected reason 'pin', got %q", boundaries[0].Reason)
	}
}

func TestSnapPin_SnapsToGap(tt *testing.T) {
	events := []transcript.Event{
		speech("09:00:00", ""),
		speech("09:00:05", ""),
		speech("09:00:30", ""),
	}
	pinTime := t("09:00:35")
	snapped := SnapPin(pinTime, events)
	if snapped != t("09:00:05") {
		tt.Errorf("snapped to %v, want 09:00:05", snapped)
	}
}

func TestSnapPin_NoGap(tt *testing.T) {
	events := []transcript.Event{
		speech("09:00:00", ""),
		speech("09:00:01", ""),
		speech("09:00:02", ""),
	}
	pinTime := t("09:00:05")
	snapped := SnapPin(pinTime, events)
	if snapped != pinTime {
		tt.Errorf("snapped to %v, want pin time %v (no qualifying gap)", snapped, pinTime)
	}
}

func TestSnapPin_OutsideLookback(tt *testing.T) {
	events := []transcript.Event{
		speech("09:00:00", ""),
		speech("09:00:30", ""),
	}
	pinTime := t("09:02:30")
	snapped := SnapPin(pinTime, events)
	if snapped != pinTime {
		tt.Errorf("snapped to %v, want pin time (gap outside lookback)", snapped)
	}
}

func TestDedupe_MergesClose(tt *testing.T) {
	boundaries := []Boundary{
		{Time: t("09:00:00"), Reason: "silence"},
		{Time: t("09:01:00"), Reason: "pin"},
		{Time: t("09:05:00"), Reason: "silence"},
	}
	result := Dedupe(boundaries)
	if len(result) != 2 {
		tt.Fatalf("expected 2 after dedup, got %d", len(result))
	}
	if result[0].Time != t("09:00:00") {
		tt.Errorf("first boundary should be 09:00:00")
	}
	if result[1].Time != t("09:05:00") {
		tt.Errorf("second boundary should be 09:05:00")
	}
}

func TestDedupe_Empty(tt *testing.T) {
	result := Dedupe(nil)
	if result != nil {
		tt.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestSplitAtBoundaries_Basic(tt *testing.T) {
	events := []transcript.Event{
		speech("09:00:00", "hello"),
		speech("09:01:00", "world"),
		speech("09:10:00", "after"),
		speech("09:11:00", "gap"),
	}
	boundaries := []Boundary{
		{Time: t("09:01:00"), Reason: "silence"},
	}
	segments := SplitAtBoundaries(events, boundaries)
	if len(segments) != 2 {
		tt.Fatalf("expected 2 segments, got %d", len(segments))
	}
	if segments[0].ID != "0900" {
		tt.Errorf("first segment ID = %q, want 0900", segments[0].ID)
	}
	if segments[1].ID != "0910" {
		tt.Errorf("second segment ID = %q, want 0910", segments[1].ID)
	}
}

func TestSplitAtBoundaries_NoEvents(tt *testing.T) {
	result := SplitAtBoundaries(nil, nil)
	if result != nil {
		tt.Errorf("expected nil for empty events")
	}
}

func TestSlugify(tt *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"API Migration & Query Optimization", "api-migration-query-optimization"},
		{"simple title", "simple-title"},
		{"  spaces  ", "spaces"},
		{"UPPERCASE", "uppercase"},
	}
	for _, tc := range tests {
		got := Slugify(tc.input)
		if got != tc.want {
			tt.Errorf("Slugify(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestIsHallucination(tt *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"Thank you for watching", true},
		{"", true},
		{"ab", true},
		{"This is normal speech content", false},
		{"[empty output]", true},
		{"no meaningful speech was detected in this segment", true},
	}
	for _, tc := range tests {
		got := isHallucination(tc.text)
		if got != tc.want {
			tt.Errorf("isHallucination(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

func TestExtractSpeakerAndText(tt *testing.T) {
	tests := []struct {
		event       transcript.Event
		wantSpeaker string
		wantText    string
	}{
		{
			transcript.Event{Type: transcript.Speech, Source: "sys", Speaker: "Alice", Text: "Hello world"},
			"Alice",
			"Hello world",
		},
		{
			transcript.Event{Type: transcript.Speech, Source: "mic", Text: "No speaker prefix"},
			"mic",
			"No speaker prefix",
		},
		{
			transcript.Event{Type: transcript.Speech, Source: "sys", Speaker: "Bob Smith", Text: "Some text"},
			"Bob Smith",
			"Some text",
		},
	}
	for _, tc := range tests {
		speaker, text := extractSpeakerAndText(tc.event)
		if speaker != tc.wantSpeaker {
			tt.Errorf("speaker = %q, want %q", speaker, tc.wantSpeaker)
		}
		if text != tc.wantText {
			tt.Errorf("text = %q, want %q", text, tc.wantText)
		}
	}
}
