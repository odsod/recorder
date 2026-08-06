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

func TestListSinks(t *testing.T) {
	runner := &fakeRunner{
		outputFn: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name != "pactl" {
				t.Errorf("expected pactl, got %s", name)
			}
			expectedArgs := []string{"--format=json", "list", "sinks"}
			if len(args) != len(expectedArgs) {
				t.Fatalf("expected %d args, got %d: %v", len(expectedArgs), len(args), args)
			}
			for i, exp := range expectedArgs {
				if args[i] != exp {
					t.Errorf("arg %d: expected %q, got %q", i, exp, args[i])
				}
			}
			return []byte(`[
				{"name": "alsa_output.pci.hdmi"},
				{"name": "bluez_output.usb"}
			]`), nil
		},
	}

	client := parec.New(runner)
	resp, err := client.ListSinks(context.Background(), parec.ListSinksRequest{})
	if err != nil {
		t.Fatal(err)
	}
	want := []parec.Sink{
		{Name: "alsa_output.pci.hdmi", MonitorSource: "alsa_output.pci.hdmi.monitor"},
		{Name: "bluez_output.usb", MonitorSource: "bluez_output.usb.monitor"},
	}
	if len(resp.Sinks) != len(want) {
		t.Fatalf("expected %d sinks, got %d: %v", len(want), len(resp.Sinks), resp.Sinks)
	}
	for i, w := range want {
		if resp.Sinks[i] != w {
			t.Errorf("sink %d: expected %+v, got %+v", i, w, resp.Sinks[i])
		}
	}
}

func TestListSinks_Empty(t *testing.T) {
	runner := &fakeRunner{
		outputFn: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(`[]`), nil
		},
	}

	client := parec.New(runner)
	resp, err := client.ListSinks(context.Background(), parec.ListSinksRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Sinks) != 0 {
		t.Errorf("expected no sinks, got %v", resp.Sinks)
	}
}

func TestListSinks_InvalidJSON(t *testing.T) {
	runner := &fakeRunner{
		outputFn: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(`not json`), nil
		},
	}

	client := parec.New(runner)
	_, err := client.ListSinks(context.Background(), parec.ListSinksRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "parse pactl sinks json") {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

func TestListSinks_CommandError(t *testing.T) {
	runner := &fakeRunner{
		outputFn: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, errors.New("command not found")
		},
	}

	client := parec.New(runner)
	_, err := client.ListSinks(context.Background(), parec.ListSinksRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "pactl list sinks") {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

func TestListSources(t *testing.T) {
	runner := &fakeRunner{
		outputFn: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			expectedArgs := []string{"--format=json", "list", "sources"}
			if len(args) != len(expectedArgs) {
				t.Fatalf("expected %d args, got %d: %v", len(expectedArgs), len(args), args)
			}
			for i, exp := range expectedArgs {
				if args[i] != exp {
					t.Errorf("arg %d: expected %q, got %q", i, exp, args[i])
				}
			}
			return []byte(`[
				{"name": "alsa_output.hdmi.monitor", "monitor_source": "alsa_output.hdmi"},
				{"name": "alsa_input.usb-mic", "monitor_source": ""},
				{"name": "bluez_input.headset", "monitor_source": ""}
			]`), nil
		},
	}

	client := parec.New(runner)
	resp, err := client.ListSources(context.Background(), parec.ListSourcesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	want := []parec.Source{
		{Name: "alsa_input.usb-mic"},
		{Name: "bluez_input.headset"},
	}
	if len(resp.Sources) != len(want) {
		t.Fatalf("expected %d sources, got %d: %v", len(want), len(resp.Sources), resp.Sources)
	}
	for i, w := range want {
		if resp.Sources[i] != w {
			t.Errorf("source %d: expected %+v, got %+v", i, w, resp.Sources[i])
		}
	}
}

func TestListSources_CommandError(t *testing.T) {
	runner := &fakeRunner{
		outputFn: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, errors.New("command not found")
		},
	}

	client := parec.New(runner)
	_, err := client.ListSources(context.Background(), parec.ListSourcesRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "pactl list sources") {
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
