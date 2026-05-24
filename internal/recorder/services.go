package recorder

import (
	"context"

	"github.com/odsod/recorder/internal/audio"
	"github.com/odsod/recorder/internal/protocol/whisper"
	"github.com/odsod/recorder/internal/segment"
	"github.com/odsod/recorder/internal/signals"
)

// Transcriber transcribes audio to text via the whisper server.
type Transcriber interface {
	Transcribe(ctx context.Context, req whisper.TranscribeRequest) (whisper.TranscribeResponse, error)
}

// TextCleaner cleans up raw ASR transcription text via LLM.
type TextCleaner interface {
	Cleanup(ctx context.Context, text string) (string, error)
}

// SegmentSummarizer produces structured summaries for transcript segments.
type SegmentSummarizer interface {
	SummarizeSegment(
		ctx context.Context,
		seg segment.Segment,
		date string,
	) (title, summary string, skip bool, err error)
}

// Services holds all external dependencies for the recorder.
type Services struct {
	Transcriber     Transcriber
	Cleaner         TextCleaner
	Summarizer      SegmentSummarizer
	SpeakerDetector signals.SpeakerPoller
	Capture         audio.Capture
	SegmentHandler  segment.Handler
}
