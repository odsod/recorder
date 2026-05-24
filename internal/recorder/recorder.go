package recorder

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/odsod/recorder/internal/config"
	"github.com/odsod/recorder/internal/lock"
	"github.com/odsod/recorder/internal/segment"
	"github.com/odsod/recorder/internal/signals"
	"github.com/odsod/recorder/internal/timeline"
	"github.com/odsod/recorder/internal/transcript"
)

const (
	speakerTimelineMaxAgeSecs = 600 // 10 minutes
	audioChunkBufferSize      = 8
)

// Recorder orchestrates audio capture, transcription, and segmentation.
type Recorder struct {
	cfg             config.Config
	svc             Services
	transcript      *TranscriptWriter
	lk              *lock.RecorderLock
	speakerTimeline *timeline.SpeakerTimeline
	participantSet  *timeline.ParticipantSet
	meetingState    *timeline.MeetingState
	silenceMonitor  *signals.SilenceMonitor
	segmenter       *segment.IncrementalSegmenter
	lastSystemText  string
	chunkNum        int
	lastFlushedTime time.Time
	lastPplSet      map[string]struct{}
}

// New creates a Recorder with the given config and services.
func New(ctx context.Context, cfg config.Config, svc Services) (*Recorder, error) {
	t := NewTranscriptWriter(cfg.Transcript.OutputDir)

	r := &Recorder{
		cfg:             cfg,
		svc:             svc,
		transcript:      t,
		lk:              lock.New(cfg.Transcript.OutputDir),
		speakerTimeline: timeline.NewSpeakerTimeline(speakerTimelineMaxAgeSecs),
		participantSet:  timeline.NewParticipantSet(),
		meetingState:    timeline.NewMeetingState(),
		silenceMonitor:  signals.NewSilenceMonitor(cfg.Signals.SilenceThresholdS),
		lastPplSet:      make(map[string]struct{}),
	}

	r.segmenter = segment.NewSegmenter(ctx, svc.SegmentHandler, func(e transcript.Event) {
		t.AppendEvent(e)
	})

	return r, nil
}

// Run starts capture, transcription, and speaker detection until ctx is cancelled.
func (r *Recorder) Run(ctx context.Context) error {
	if err := r.lk.Acquire(); err != nil {
		return err
	}
	defer r.lk.Release()

	slog.InfoContext(ctx, "transcript configured",
		"path", r.transcript.Path(),
	)
	slog.InfoContext(ctx, "whisper configured",
		"url", r.cfg.Whisper.URL,
	)
	slog.InfoContext(ctx, "llm configured",
		"url", r.cfg.LLM.URL,
	)

	r.appendEvent(ctx, transcript.Event{Time: time.Now(), Type: transcript.Recorder, Text: "started"})

	chunkCh := make(chan AudioChunk, audioChunkBufferSize)

	var wg sync.WaitGroup

	wg.Go(func() {
		r.transcriptionWorker(ctx, chunkCh)
	})

	wg.Go(func() {
		signals.RunSpeakerCollector(
			ctx,
			r.svc.SpeakerDetector,
			r.speakerTimeline,
			r.participantSet,
			r.meetingState,
		)
	})

	slog.InfoContext(ctx, "signals started")

	r.captureLoop(ctx, chunkCh)
	close(chunkCh)

	wg.Wait()

	r.appendEvent(ctx, transcript.Event{Time: time.Now(), Type: transcript.Recorder, Text: "stopped"})
	slog.InfoContext(ctx, "running final segmentation")
	r.segmenter.Flush(ctx)
	slog.InfoContext(ctx, "shutdown complete")
	return nil
}
