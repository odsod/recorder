package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/odsod/recorder/internal/config"
	"github.com/odsod/recorder/internal/note"
	"github.com/odsod/recorder/internal/prompt"
	"github.com/odsod/recorder/internal/recorder"
	"github.com/odsod/recorder/internal/segment"
	"github.com/odsod/recorder/internal/transcript"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level:       slog.LevelInfo,
		ReplaceAttr: replaceConsoleAttr,
	})))

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: recorder <run|note|segment|prompts>\n")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		if err := runRecorder(); err != nil {
			slog.ErrorContext(context.Background(), "run failed",
				"err", err,
			)
			os.Exit(1)
		}
	case "note":
		if err := runNote(); err != nil {
			slog.ErrorContext(context.Background(), "note failed",
				"err", err,
			)
			os.Exit(1)
		}
	case "segment":
		if err := runSegment(); err != nil {
			slog.ErrorContext(context.Background(), "segment failed",
				"err", err,
			)
			os.Exit(1)
		}
	case "prompts":
		if err := runPrompts(); err != nil {
			slog.ErrorContext(context.Background(), "prompts failed",
				"err", err,
			)
			os.Exit(1)
		}
	default:
		slog.ErrorContext(context.Background(), "unknown command",
			"command", os.Args[1],
		)
		os.Exit(1)
	}
}

func runNote() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	closeLog, err := configureLogger(cfg, os.Stderr)
	if err != nil {
		return fmt.Errorf("configure logger: %w", err)
	}
	defer closeLog()

	return note.Run(cfg, os.Args[2:])
}

func runPrompts() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	return prompt.Run(cfg, os.Args[2:], os.Stdout)
}

func runRecorder() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	closeLog, err := configureLogger(cfg, os.Stdout)
	if err != nil {
		return fmt.Errorf("configure logger: %w", err)
	}
	defer closeLog()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	httpClient := newHTTPClient()
	defer closeHTTPClient(httpClient)

	d := buildDeps(cfg, httpClient)
	rec, err := recorder.New(ctx, cfg, recorderServices(cfg, d))
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

	if boundariesOnly {
		segment.PrintBoundaries(t.Events)
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	closeLog, err := configureLogger(cfg, os.Stderr)
	if err != nil {
		return fmt.Errorf("configure logger: %w", err)
	}
	defer closeLog()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if !write {
		return segment.RunBatch(ctx, t.Events, false, nil)
	}

	httpClient := newHTTPClient()
	defer closeHTTPClient(httpClient)

	d := buildDeps(cfg, httpClient)
	return segment.RunBatch(ctx, t.Events, true, recorderServices(cfg, d).SegmentHandler)
}

// configureLogger sets the default slog logger. Console output uses a human-readable
// text handler; when cfg.Log.File is set, a JSONL handler mirrors logs to that file.
func configureLogger(cfg config.Config, console io.Writer) (func(), error) {
	handlers := []slog.Handler{slog.NewTextHandler(console, &slog.HandlerOptions{
		Level:       slog.LevelInfo,
		ReplaceAttr: replaceConsoleAttr,
	})}

	var logFile *os.File
	if cfg.Log.File != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.Log.File), 0o755); err != nil {
			return func() {}, fmt.Errorf("log file mkdir: %w", err)
		}
		f, err := os.OpenFile(cfg.Log.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return func() {}, fmt.Errorf("log file open: %w", err)
		}
		logFile = f
		handlers = append(handlers, slog.NewJSONHandler(f, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	var handler slog.Handler
	if len(handlers) == 1 {
		handler = handlers[0]
	} else {
		handler = slog.NewMultiHandler(handlers...)
	}

	slog.SetDefault(slog.New(handler))

	closeFn := func() {}
	if logFile != nil {
		closeFn = func() { _ = logFile.Close() }
	}
	return closeFn, nil
}

func replaceConsoleAttr(_ []string, a slog.Attr) slog.Attr {
	switch a.Key {
	case slog.TimeKey:
		return slog.String("time", a.Value.Time().Format("15:04:05"))
	case slog.LevelKey:
		if a.Value.String() == slog.LevelInfo.String() {
			return slog.Attr{}
		}
		return a
	default:
		return a
	}
}
