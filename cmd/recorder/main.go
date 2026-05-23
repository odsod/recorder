package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/odsod/recorder/internal/config"
	"github.com/odsod/recorder/internal/note"
	"github.com/odsod/recorder/internal/recorder"
	"github.com/odsod/recorder/internal/segment"
	"github.com/odsod/recorder/internal/summarize"
	"github.com/odsod/recorder/internal/transcript"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: recorder <run|note|segment>\n")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		if err := runRecorder(); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
	case "note":
		if err := note.Run(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
	case "segment":
		if err := runSegment(); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "recorder: unknown command %q\n", os.Args[1])
		os.Exit(1)
	}
}

func runRecorder() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	rec, err := recorder.New(ctx, cfg)
	if err != nil {
		return err
	}
	return rec.Run(ctx)
}

func runSegment() error {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: recorder segment <transcript> [--boundaries] [--write]\n")
		os.Exit(1)
	}

	path := filepath.Clean(os.Args[2])
	boundariesOnly := false
	write := false
	for _, arg := range os.Args[3:] {
		switch arg {
		case "--boundaries":
			boundariesOnly = true
		case "--write":
			write = true
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	t, err := transcript.Parse(data)
	if err != nil {
		return err
	}

	boundaries := segment.DetectBoundaries(t.Events, time.Now())

	if boundariesOnly {
		for _, b := range boundaries {
			fmt.Printf("[%s] %s\n", b.Time.Format("15:04:05"), b.Reason)
		}
		return nil
	}

	segments := segment.SplitAtBoundaries(t.Events, boundaries)
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

	if write {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		date := time.Now().Format("2006-01-02")

		for _, seg := range segments {
			ctx := context.Background()
			title, summary, skip, err := summarize.SummarizeSegment(ctx, seg, cfg.LLM, date)
			if err != nil {
				fmt.Fprintf(os.Stderr, "summarize error for %s: %v\n", seg.ID, err)
				continue
			}
			if skip {
				fmt.Printf("  %s: skipped\n", seg.ID)
				continue
			}
			filename, err := summarize.WriteSegmentFile(title, summary, seg, date, cfg.Segments.OutputDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "write error for %s: %v\n", seg.ID, err)
				continue
			}
			fmt.Printf("  %s: wrote %s\n", seg.ID, filename)
		}
	}
	return nil
}
