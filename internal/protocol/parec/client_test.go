package parec_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

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

func TestGetDefaultSink(t *testing.T) {
	runner := &fakeRunner{
		outputFn: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name != "pactl" {
				t.Errorf("expected pactl, got %s", name)
			}
			if len(args) != 1 || args[0] != "get-default-sink" {
				t.Errorf("unexpected args: %v", args)
			}
			return []byte("alsa_output.pci\n"), nil
		},
	}

	client := parec.New(runner)
	resp, err := client.GetDefaultSink(context.Background(), parec.GetDefaultSinkRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.MonitorSource != "alsa_output.pci.monitor" {
		t.Errorf("expected 'alsa_output.pci.monitor', got %q", resp.MonitorSource)
	}
}

func TestGetDefaultSource(t *testing.T) {
	runner := &fakeRunner{
		outputFn: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name != "pactl" {
				t.Errorf("expected pactl, got %s", name)
			}
			if len(args) != 1 || args[0] != "get-default-source" {
				t.Errorf("unexpected args: %v", args)
			}
			return []byte("alsa_input.usb\n"), nil
		},
	}

	client := parec.New(runner)
	resp, err := client.GetDefaultSource(context.Background(), parec.GetDefaultSourceRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Source != "alsa_input.usb" {
		t.Errorf("expected 'alsa_input.usb', got %q", resp.Source)
	}
}

func TestGetDefaultSink_Error(t *testing.T) {
	runner := &fakeRunner{
		outputFn: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, errors.New("command not found")
		},
	}

	client := parec.New(runner)
	_, err := client.GetDefaultSink(context.Background(), parec.GetDefaultSinkRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "pactl get-default-sink") {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

func TestGetDefaultSource_Error(t *testing.T) {
	runner := &fakeRunner{
		outputFn: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, errors.New("command not found")
		},
	}

	client := parec.New(runner)
	_, err := client.GetDefaultSource(context.Background(), parec.GetDefaultSourceRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "pactl get-default-source") {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

func TestStartCapture(t *testing.T) {
	pcmData := bytes.Repeat([]byte{0x01, 0x02}, 100)
	closed := false

	runner := &fakeRunner{
		startFn: func(ctx context.Context, name string, args ...string) (io.ReadCloser, func() error, error) {
			if name != "parec" {
				t.Errorf("expected parec, got %s", name)
			}

			expectedArgs := []string{
				"--device=test-device",
				"--rate=16000",
				"--channels=1",
				"--format=s16le",
				"--raw",
			}
			if len(args) != len(expectedArgs) {
				t.Fatalf("expected %d args, got %d: %v", len(expectedArgs), len(args), args)
			}
			for i, exp := range expectedArgs {
				if args[i] != exp {
					t.Errorf("arg %d: expected %q, got %q", i, exp, args[i])
				}
			}

			reader := io.NopCloser(bytes.NewReader(pcmData))
			stop := func() error {
				closed = true
				return nil
			}
			return reader, stop, nil
		},
	}

	client := parec.New(runner)
	stream, err := client.StartCapture(context.Background(), parec.StartCaptureRequest{
		Device: "test-device", SampleRate: 16000,
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := io.ReadAll(stream)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, pcmData) {
		t.Error("read data doesn't match expected PCM data")
	}

	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
	if !closed {
		t.Error("stream was not closed")
	}
}

func TestStartCapture_Error(t *testing.T) {
	runner := &fakeRunner{
		startFn: func(ctx context.Context, name string, args ...string) (io.ReadCloser, func() error, error) {
			return nil, nil, errors.New("device not found")
		},
	}

	client := parec.New(runner)
	_, err := client.StartCapture(context.Background(), parec.StartCaptureRequest{
		Device: "bad-device", SampleRate: 16000,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
