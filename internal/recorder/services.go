package recorder

import (
	"context"

	"github.com/odsod/recorder/internal/audio"
	"github.com/odsod/recorder/internal/protocol/whisper"
	"github.com/odsod/recorder/internal/segment"
	"github.com/odsod/recorder/internal/signals"
)

type Transcriber interface {
	Transcribe(ctx context.Context, req whisper.TranscribeRequest) (whisper.TranscribeResponse, error)
}

type TextCleaner interface {
	Cleanup(ctx context.Context, text string) (string, error)
}

type SegmentSummarizer interface {
	SummarizeSegment(
		ctx context.Context,
		seg segment.Segment,
		date string,
	) (title, summary string, skip bool, err error)
}

type Services struct {
	Transcriber     Transcriber
	Cleaner         TextCleaner
	Summarizer      SegmentSummarizer
	SpeakerDetector signals.SpeakerPoller
	Capture         audio.Capture
	SegmentHandler  segment.SegmentHandler
}
