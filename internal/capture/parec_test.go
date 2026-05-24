package capture

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/odsod/recorder/internal/audio/frame"
	"github.com/odsod/recorder/internal/audio/pcm"
	"github.com/odsod/recorder/internal/protocol/parec"
)

type fakeRunner struct {
	outputFn func(ctx context.Context, name string, args ...string) ([]byte, error)
	startFn  func(ctx context.Context, name string, args ...string) (io.ReadCloser, func() error, error)
}

func (f *fakeRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return f.outputFn(ctx, name, args...)
}

func (f *fakeRunner) Start(ctx context.Context, name string, args ...string) (io.ReadCloser, func() error, error) {
	return f.startFn(ctx, name, args...)
}

func TestParec_Start(t *testing.T) {
	sysData := bytes.Repeat([]byte{0x01}, pcm.FrameBytes*2)
	micData := bytes.Repeat([]byte{0x02}, pcm.FrameBytes*2)
	startCount := 0

	runner := &fakeRunner{
		outputFn: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if args[0] == "get-default-sink" {
				return []byte("sink\n"), nil
			}
			return []byte("mic\n"), nil
		},
		startFn: func(ctx context.Context, name string, args ...string) (io.ReadCloser, func() error, error) {
			startCount++
			if startCount == 1 {
				return io.NopCloser(bytes.NewReader(sysData)), func() error { return nil }, nil
			}
			return io.NopCloser(bytes.NewReader(micData)), func() error { return nil }, nil
		},
	}

	src := NewParec(parec.New(runner))
	frames, err := src.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	frame1, ok := <-frames
	if !ok {
		t.Fatal("expected first frame")
	}
	if !bytes.Equal(frame1.Sys, sysData[:pcm.FrameBytes]) {
		t.Error("sys frame mismatch")
	}
	if !bytes.Equal(frame1.Mic, micData[:pcm.FrameBytes]) {
		t.Error("mic frame mismatch")
	}

	frame2, ok := <-frames
	if !ok {
		t.Fatal("expected second frame")
	}
	if !bytes.Equal(frame2.Sys, sysData[pcm.FrameBytes:]) {
		t.Error("second sys frame mismatch")
	}

	if _, ok := <-frames; ok {
		t.Error("expected channel closed after streams exhausted")
	}

	if src.MonitorSource() != "sink.monitor" {
		t.Errorf("MonitorSource = %q, want sink.monitor", src.MonitorSource())
	}
	if src.MicSource() != "mic" {
		t.Errorf("MicSource = %q, want mic", src.MicSource())
	}
}

func TestParec_MicFallbackToSilence(t *testing.T) {
	sysData := bytes.Repeat([]byte{0x01}, pcm.FrameBytes)
	startCount := 0

	runner := &fakeRunner{
		outputFn: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if args[0] == "get-default-sink" {
				return []byte("sink\n"), nil
			}
			return []byte("mic\n"), nil
		},
		startFn: func(ctx context.Context, name string, args ...string) (io.ReadCloser, func() error, error) {
			startCount++
			if startCount == 1 {
				return io.NopCloser(bytes.NewReader(sysData)), func() error { return nil }, nil
			}
			return io.NopCloser(bytes.NewReader(nil)), func() error { return nil }, nil
		},
	}

	src := NewParec(parec.New(runner))
	frames, err := src.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	f, ok := <-frames
	if !ok {
		t.Fatal("expected frame")
	}
	if !bytes.Equal(f.Mic, frame.Silent(pcm.FrameBytes)) {
		t.Error("expected silent mic fallback")
	}
}

func TestParec_Stop(t *testing.T) {
	closed := false
	runner := &fakeRunner{
		outputFn: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if args[0] == "get-default-sink" {
				return []byte("sink\n"), nil
			}
			return []byte("mic\n"), nil
		},
		startFn: func(ctx context.Context, name string, args ...string) (io.ReadCloser, func() error, error) {
			return io.NopCloser(bytes.NewReader(nil)), func() error {
				closed = true
				return nil
			}, nil
		},
	}

	src := NewParec(parec.New(runner))
	_, err := src.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Stop(); err != nil {
		t.Fatal(err)
	}
	if !closed {
		t.Error("expected capture streams to close on Stop")
	}
}
