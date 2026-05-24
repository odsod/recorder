package audio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/odsod/recorder/internal/protocol/parec"
)

// ParecCapture implements Capture using PulseAudio's parec command.
type ParecCapture struct {
	client  *parec.Client
	monitor string
	mic     string
	stop    func()
}

// NewParecCapture creates a ParecCapture using the given parec protocol client.
func NewParecCapture(client *parec.Client) *ParecCapture {
	return &ParecCapture{client: client}
}

// MonitorSource returns the system audio monitor source name.
func (c *ParecCapture) MonitorSource() string {
	return c.monitor
}

// MicSource returns the microphone source name.
func (c *ParecCapture) MicSource() string {
	return c.mic
}

// Start begins capturing system and microphone audio, returning a channel of frames.
func (c *ParecCapture) Start(ctx context.Context) (<-chan Frame, error) {
	sinkResp, err := c.client.GetDefaultSink(ctx, parec.GetDefaultSinkRequest{})
	if err != nil {
		return nil, err
	}
	sourceResp, err := c.client.GetDefaultSource(ctx, parec.GetDefaultSourceRequest{})
	if err != nil {
		return nil, err
	}
	c.monitor = sinkResp.MonitorSource
	c.mic = sourceResp.Source

	sysStream, err := c.client.StartCapture(ctx, parec.StartCaptureRequest{
		Device: c.monitor, SampleRate: SampleRate,
	})
	if err != nil {
		return nil, fmt.Errorf("start sys parec: %w", err)
	}
	micStream, err := c.client.StartCapture(ctx, parec.StartCaptureRequest{
		Device: c.mic, SampleRate: SampleRate,
	})
	if err != nil {
		_ = sysStream.Close()
		return nil, fmt.Errorf("start mic parec: %w", err)
	}

	frames := make(chan Frame, 2)
	done := make(chan struct{})
	var once sync.Once
	c.stop = func() {
		once.Do(func() {
			close(done)
			_ = sysStream.Close()
			_ = micStream.Close()
		})
	}

	go func() {
		defer close(frames)
		defer c.stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			default:
			}

			sysData, err := ReadFrame(sysStream)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					return
				}
				return
			}
			micData, err := ReadFrame(micStream)
			if err != nil {
				micData = SilentFrame()
			}

			frame := Frame{Sys: sysData, Mic: micData}
			select {
			case frames <- frame:
			case <-ctx.Done():
				return
			case <-done:
				return
			}
		}
	}()

	return frames, nil
}

// Stop terminates the capture processes.
func (c *ParecCapture) Stop() error {
	if c.stop != nil {
		c.stop()
	}
	return nil
}
