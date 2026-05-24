package recorder

import (
	"context"
	"fmt"
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

func New(ctx context.Context, cfg config.Config, lk *lock.RecorderLock, svc Services) (*Recorder, error) {
	t := NewTranscriptWriter(cfg.Transcript.OutputDir)

	r := &Recorder{
		cfg:             cfg,
		svc:             svc,
		transcript:      t,
		lk:              lk,
		speakerTimeline: timeline.NewSpeakerTimeline(speakerTimelineMaxAgeSecs),
		participantSet:  timeline.NewParticipantSet(),
		meetingState:    timeline.NewMeetingState(),
		silenceMonitor:  signals.NewSilenceMonitor(cfg.Signals.SilenceThresholdS),
		lastPplSet:      make(map[string]struct{}),
	}

	r.segmenter = segment.NewSegmenter(ctx, svc.SegmentHandler, r.log, func(e transcript.Event) {
		t.AppendEvent(e)
	})

	return r, nil
}

func (r *Recorder) Run(ctx context.Context) error {
	r.log("transcript: " + r.transcript.Path())
	r.log("whisper: " + r.cfg.Whisper.URL)
	r.log("llm: " + r.cfg.LLM.URL)

	r.appendEvent(transcript.Event{Time: time.Now(), Type: transcript.Recorder, Text: "started"})

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
			r.log,
		)
	})

	r.log("signals started")

	r.captureLoop(ctx, chunkCh)
	close(chunkCh)

	wg.Wait()

	r.appendEvent(transcript.Event{Time: time.Now(), Type: transcript.Recorder, Text: "stopped"})
	r.log("running final segmentation...")
	r.segmenter.Flush(ctx)
	r.log("shutdown complete")
	return nil
}

func (r *Recorder) log(msg string) {
	ts := time.Now().Format("15:04:05")
	fmt.Printf("[%s] %s\n", ts, msg)
}
