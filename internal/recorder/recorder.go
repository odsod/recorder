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
	transcript      *transcript.DailyTranscript
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
	t := transcript.New(cfg.Transcript.OutputDir)
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
		silenceMonitor:  signals.NewSilenceMonitor(cfg.Signals.SilenceThresholdSecs),
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
	r.segmenter = segment.NewSegmenter(ctx, handler, r.log, func(timestamp, text string) {
		t.Append(timestamp, "✂️ seg", text, nil)
	})

	return r, nil
}

func (r *Recorder) Run(ctx context.Context) error {
	r.log(transcript.FormatMessage("🟢 rec", "started", nil))
	r.log(fmt.Sprintf("transcript: %s", r.transcript.Path()))
	r.log(fmt.Sprintf("whisper: %s", r.cfg.Whisper.URL))
	r.log(fmt.Sprintf("llm: %s", r.cfg.LLM.URL))

	ts := time.Now().Format("15:04:05")
	r.transcript.Append(ts, "🟢 rec", "started", nil)

	chunkCh := make(chan AudioChunk, audioChunkBufferSize)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		r.transcriptionWorker(ctx, chunkCh)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		signals.RunSpeakerCollector(
			ctx,
			r.speakerTimeline,
			r.participantSet,
			r.meetingState,
			r.cfg.Signals.CDPPorts,
			r.log,
		)
	}()

	r.log("signals started")

	r.captureLoop(ctx, chunkCh)
	close(chunkCh)

	wg.Wait()

	ts = time.Now().Format("15:04:05")
	r.transcript.Append(ts, "🔴 rec", "stopped", nil)
	r.log("running final segmentation...")
	r.segmenter.Flush(ctx)
	r.log("shutdown complete")
	r.lk.Release()
	r.log(transcript.FormatMessage("🔴 rec", "stopped", nil))
	return nil
}

func (r *Recorder) log(msg string) {
	ts := time.Now().Format("15:04:05")
	fmt.Printf("[%s] %s\n", ts, msg)
}
