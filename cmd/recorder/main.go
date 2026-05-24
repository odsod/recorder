package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/odsod/recorder/internal/app"
	"github.com/odsod/recorder/internal/config"
	"github.com/odsod/recorder/internal/note"
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

	a := app.New(cfg)
	defer a.Close()

	return a.Run(ctx)
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

	if boundariesOnly {
		app.PrintBoundaries(t.Events)
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	a := app.New(cfg)
	defer a.Close()

	return a.RunSegment(ctx, t.Events, write)
}
