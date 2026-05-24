package capture

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/odsod/recorder/internal/audio/frame"
	"github.com/odsod/recorder/internal/audio/pcm"
	"github.com/odsod/recorder/internal/protocol/parec"
)

// Parec implements Source using PulseAudio's parec command.
type Parec struct {
	client  *parec.Client
	monitor string
	mic     string
	stop    func()
}

// NewParec creates a Parec source using the given parec protocol client.
func NewParec(client *parec.Client) *Parec {
	return &Parec{client: client}
}

// MonitorSource returns the system audio monitor source name.
func (c *Parec) MonitorSource() string {
	return c.monitor
}

// MicSource returns the microphone source name.
func (c *Parec) MicSource() string {
	return c.mic
}

// Start begins capturing system and microphone audio, returning a channel of frames.
func (c *Parec) Start(ctx context.Context) (<-chan frame.Dual, error) {
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
		Device: c.monitor, SampleRate: pcm.SampleRate,
	})
	if err != nil {
		return nil, fmt.Errorf("start sys parec: %w", err)
	}
	micStream, err := c.client.StartCapture(ctx, parec.StartCaptureRequest{
		Device: c.mic, SampleRate: pcm.SampleRate,
	})
	if err != nil {
		_ = sysStream.Close()
		return nil, fmt.Errorf("start mic parec: %w", err)
	}

	frames := make(chan frame.Dual, 2)
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

			sysData, err := frame.Read(sysStream, pcm.FrameBytes)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					return
				}
				return
			}
			micData, err := frame.Read(micStream, pcm.FrameBytes)
			if err != nil {
				micData = frame.Silent(pcm.FrameBytes)
			}

			f := frame.Dual{Sys: sysData, Mic: micData}
			select {
			case frames <- f:
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
func (c *Parec) Stop() error {
	if c.stop != nil {
		c.stop()
	}
	return nil
}
