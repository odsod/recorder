package recorder

import (
	"testing"
	"time"

	"github.com/odsod/recorder/internal/timeline"
)

func TestAttributeSpeaker(t *testing.T) {
	const ratio = 0.05

	tests := []struct {
		name     string
		speakers []timeline.SpeakerDuration
		want     string
	}{
		{
			name:     "empty",
			speakers: nil,
			want:     "",
		},
		{
			name:     "single speaker",
			speakers: []timeline.SpeakerDuration{{Name: "Alice", Duration: 10 * time.Second}},
			want:     "Alice",
		},
		{
			name: "unambiguous - second speaker below 5% threshold",
			speakers: []timeline.SpeakerDuration{
				{Name: "Alice", Duration: 20 * time.Second},
				{Name: "Bob", Duration: 500 * time.Millisecond},
			},
			want: "Alice",
		},
		{
			name: "ambiguous - short interjection above threshold",
			speakers: []timeline.SpeakerDuration{
				{Name: "Alice", Duration: 15 * time.Second},
				{Name: "Bob", Duration: 1500 * time.Millisecond},
			},
			want: "Alice(90%),Bob(10%)",
		},
		{
			name: "ambiguous - roughly equal",
			speakers: []timeline.SpeakerDuration{
				{Name: "Alice", Duration: 8 * time.Second},
				{Name: "Bob", Duration: 7 * time.Second},
			},
			want: "Alice(53%),Bob(47%)",
		},
		{
			name: "equal time",
			speakers: []timeline.SpeakerDuration{
				{Name: "Alice", Duration: 10 * time.Second},
				{Name: "Bob", Duration: 10 * time.Second},
			},
			want: "Alice(50%),Bob(50%)",
		},
		{
			name: "three speakers - only top two considered",
			speakers: []timeline.SpeakerDuration{
				{Name: "Alice", Duration: 10 * time.Second},
				{Name: "Bob", Duration: 8 * time.Second},
				{Name: "Carol", Duration: 2 * time.Second},
			},
			want: "Alice(55%),Bob(45%)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := attributeSpeaker(tc.speakers, ratio)
			if got != tc.want {
				t.Errorf("attributeSpeaker(%v) = %q, want %q", tc.speakers, got, tc.want)
			}
		})
	}
}
