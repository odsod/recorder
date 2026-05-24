// Package parec provides a client for PulseAudio audio capture via pactl/parec.
//
// It wraps the pactl command for querying default audio devices and the parec
// command for streaming raw PCM audio. The CommandRunner interface allows
// injecting a fake for testing without real PulseAudio.
//
// Query operations (GetDefaultSink, GetDefaultSource) follow the standard
// request/response pattern. StartCapture returns a CaptureStream that
// implements io.Reader for continuous audio data and io.Closer to stop capture.
package parec

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// CommandRunner abstracts command execution for testability.
type CommandRunner interface {
	// Output runs a command and returns its stdout.
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
	// Start runs a command and returns a handle to its stdout stream.
	Start(ctx context.Context, name string, args ...string) (io.ReadCloser, func() error, error)
}

// ExecRunner is the default CommandRunner using os/exec.
type ExecRunner struct{}

// Output runs a command and returns its stdout.
func (ExecRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

// Start runs a command and returns a handle to its stdout stream.
func (ExecRunner) Start(ctx context.Context, name string, args ...string) (io.ReadCloser, func() error, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stderr = io.Discard

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start %s: %w", name, err)
	}

	stop := func() error {
		_ = cmd.Process.Kill()
		return cmd.Wait()
	}

	return stdout, stop, nil
}

// Client communicates with PulseAudio via pactl and parec commands.
type Client struct {
	runner CommandRunner
}

// New creates a Client with a custom CommandRunner.
func New(runner CommandRunner) *Client {
	return &Client{runner: runner}
}

// NewDefault creates a Client that executes real system commands.
func NewDefault() *Client {
	return &Client{runner: ExecRunner{}}
}

// GetDefaultSinkRequest is empty; the default sink is a system-global query.
type GetDefaultSinkRequest struct{}

// GetDefaultSinkResponse contains the monitor source for the default output device.
type GetDefaultSinkResponse struct {
	// MonitorSource is the PulseAudio source name for capturing system audio
	// (the default sink name with ".monitor" appended).
	MonitorSource string
}

// GetDefaultSink queries the system's default audio output device.
func (c *Client) GetDefaultSink(ctx context.Context, _ GetDefaultSinkRequest) (GetDefaultSinkResponse, error) {
	out, err := c.runner.Output(ctx, "pactl", "get-default-sink")
	if err != nil {
		return GetDefaultSinkResponse{}, fmt.Errorf("pactl get-default-sink: %w", err)
	}
	return GetDefaultSinkResponse{
		MonitorSource: strings.TrimSpace(string(out)) + ".monitor",
	}, nil
}

// GetDefaultSourceRequest is empty; the default source is a system-global query.
type GetDefaultSourceRequest struct{}

// GetDefaultSourceResponse contains the default microphone source name.
type GetDefaultSourceResponse struct {
	// Source is the PulseAudio source name for the default input device.
	Source string
}

// GetDefaultSource queries the system's default audio input device.
func (c *Client) GetDefaultSource(ctx context.Context, _ GetDefaultSourceRequest) (GetDefaultSourceResponse, error) {
	out, err := c.runner.Output(ctx, "pactl", "get-default-source")
	if err != nil {
		return GetDefaultSourceResponse{}, fmt.Errorf("pactl get-default-source: %w", err)
	}
	return GetDefaultSourceResponse{
		Source: strings.TrimSpace(string(out)),
	}, nil
}

// StartCaptureRequest specifies the audio device and format for streaming capture.
type StartCaptureRequest struct {
	// Device is the PulseAudio source name to capture from.
	Device string
	// SampleRate is the capture sample rate in Hz (e.g. 16000).
	SampleRate int
}

// CaptureStream is a long-lived audio capture session. It implements io.Reader
// for reading raw s16le mono PCM frames, and io.Closer to terminate capture.
type CaptureStream struct {
	reader io.ReadCloser
	stop   func() error
}

// Read reads raw PCM audio data from the capture stream.
func (s *CaptureStream) Read(p []byte) (int, error) {
	return s.reader.Read(p)
}

// Close terminates the capture process and releases resources.
func (s *CaptureStream) Close() error {
	return s.stop()
}

// StartCapture begins streaming audio from the specified device.
// The returned CaptureStream provides continuous PCM data until closed.
func (c *Client) StartCapture(ctx context.Context, req StartCaptureRequest) (*CaptureStream, error) {
	reader, stop, err := c.runner.Start(ctx, "parec",
		"--device="+req.Device,
		fmt.Sprintf("--rate=%d", req.SampleRate),
		"--channels=1",
		"--format=s16le",
		"--raw",
	)
	if err != nil {
		return nil, err
	}
	return &CaptureStream{reader: reader, stop: stop}, nil
}
