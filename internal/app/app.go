package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/odsod/recorder/internal/cdp"
	"github.com/odsod/recorder/internal/config"
	"github.com/odsod/recorder/internal/lock"
	"github.com/odsod/recorder/internal/recorder"
	"github.com/odsod/recorder/internal/segment"
	"github.com/odsod/recorder/internal/signals"
	"github.com/odsod/recorder/internal/summarize"
	"github.com/odsod/recorder/internal/transcript"
)

type App struct {
	cfg      config.Config
	deps     Deps
	lk       *lock.RecorderLock
	lockHeld bool
}

func New(cfg config.Config) *App {
	return &App{
		cfg:  cfg,
		deps: BuildDeps(cfg),
		lk:   lock.New(cfg.Transcript.OutputDir),
	}
}

func (a *App) Close() {
	a.releaseLock()
	a.deps.Close()
}

func (a *App) Run(ctx context.Context) error {
	if err := a.lk.Acquire(); err != nil {
		return err
	}
	a.lockHeld = true
	defer a.releaseLock()

	rec, err := recorder.New(ctx, a.cfg, a.lk, a.recorderServices())
	if err != nil {
		return err
	}
	return rec.Run(ctx)
}

func (a *App) RunSegment(ctx context.Context, events []transcript.Event, write bool) error {
	boundaries := segment.DetectBoundaries(events, time.Now())
	segments := segment.SplitAtBoundaries(events, boundaries)

	for _, seg := range segments {
		speechCount := 0
		for _, e := range seg.Events {
			if e.IsSpeech() {
				speechCount++
			}
		}
		fmt.Printf("segment %s: %s–%s (%d speech events)\n",
			seg.ID, seg.Start.Format("15:04"), seg.End.Format("15:04"), speechCount)
	}

	if !write {
		return nil
	}

	date := time.Now().Format("2006-01-02")
	return a.writeSegments(ctx, segments, date)
}

func PrintBoundaries(events []transcript.Event) {
	boundaries := segment.DetectBoundaries(events, time.Now())
	for _, b := range boundaries {
		fmt.Printf("[%s] %s\n", b.Time.Format("15:04:05"), b.Reason)
	}
}

func (a *App) writeSegments(ctx context.Context, segments []segment.Segment, date string) error {
	for _, seg := range segments {
		if err := ctx.Err(); err != nil {
			return err
		}

		title, summary, skip, err := a.deps.Summarizer.SummarizeSegment(ctx, seg, date)
		if err != nil {
			fmt.Fprintf(os.Stderr, "summarize error for %s: %v\n", seg.ID, err)
			continue
		}
		if skip {
			fmt.Printf("  %s: skipped\n", seg.ID)
			continue
		}
		filename, err := summarize.WriteSegmentFile(title, summary, seg, date, a.cfg.Segments.OutputDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "write error for %s: %v\n", seg.ID, err)
			continue
		}
		fmt.Printf("  %s: wrote %s\n", seg.ID, filename)
	}
	return nil
}

func (a *App) releaseLock() {
	if !a.lockHeld {
		return
	}
	a.lk.Release()
	a.lockHeld = false
}

func (a *App) recorderServices() recorder.Services {
	return recorder.Services{
		Transcriber:     a.deps.Whisper,
		Cleaner:         a.deps.Cleaner,
		Summarizer:      a.deps.Summarizer,
		SpeakerDetector: &speakerPollerAdapter{d: a.deps.SpeakerDetector},
		Capture:         a.deps.Capture,
		SegmentHandler:  a.segmentHandler(),
	}
}

func (a *App) segmentHandler() *segment.FuncHandler {
	return &segment.FuncHandler{
		SummarizeFn: a.deps.Summarizer.SummarizeSegment,
		WriteSegmentFn: func(title, summary string, seg segment.Segment, date string) (string, error) {
			return summarize.WriteSegmentFile(title, summary, seg, date, a.cfg.Segments.OutputDir)
		},
	}
}

type speakerPollerAdapter struct {
	d *cdp.SpeakerDetector
}

func (a *speakerPollerAdapter) Poll(ctx context.Context) (signals.PollResult, error) {
	result, err := a.d.Poll(ctx)
	if err != nil {
		return signals.PollResult{}, err
	}

	var pr signals.PollResult
	if result.MeetingChange != nil {
		pr.MeetingChange = &signals.MeetingChange{Title: result.MeetingChange.Title}
	}
	if result.Participants != nil {
		pr.Participants = make([]signals.ParticipantState, len(result.Participants))
		for i, p := range result.Participants {
			pr.Participants[i] = signals.ParticipantState{Name: p.Name, Speaking: p.Speaking}
		}
	}
	return pr, nil
}
