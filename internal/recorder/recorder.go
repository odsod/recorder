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
	"github.com/odsod/recorder/internal/summarize"
	"github.com/odsod/recorder/internal/timeline"
	"github.com/odsod/recorder/internal/transcript"
)

const (
	speakerTimelineMaxAgeSecs = 600 // 10 minutes
	audioChunkBufferSize      = 8
)

type Recorder struct {
	cfg             config.Config
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

func New(ctx context.Context, cfg config.Config) (*Recorder, error) {
	t := NewTranscriptWriter(cfg.Transcript.OutputDir)
	lk := lock.New(cfg.Transcript.OutputDir)

	if err := lk.Acquire(); err != nil {
		return nil, err
	}

	r := &Recorder{
		cfg:             cfg,
		transcript:      t,
		lk:              lk,
		speakerTimeline: timeline.NewSpeakerTimeline(speakerTimelineMaxAgeSecs),
		participantSet:  timeline.NewParticipantSet(),
		meetingState:    timeline.NewMeetingState(),
		silenceMonitor:  signals.NewSilenceMonitor(cfg.Signals.SilenceThresholdS),
		lastPplSet:      make(map[string]struct{}),
	}

	handler := &segment.FuncHandler{
		SummarizeFn: func(ctx context.Context, seg segment.Segment, date string) (string, string, bool, error) {
			return summarize.SummarizeSegment(ctx, seg, cfg.LLM, date)
		},
		WriteSegmentFn: func(title, summary string, seg segment.Segment, date string) (string, error) {
			return summarize.WriteSegmentFile(title, summary, seg, date, cfg.Segments.OutputDir)
		},
	}
	r.segmenter = segment.NewSegmenter(ctx, handler, r.log, func(e transcript.Event) {
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
			r.speakerTimeline,
			r.participantSet,
			r.meetingState,
			r.cfg.Signals.CDPPorts,
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
	r.lk.Release()
	return nil
}

func (r *Recorder) log(msg string) {
	ts := time.Now().Format("15:04:05")
	fmt.Printf("[%s] %s\n", ts, msg)
}
