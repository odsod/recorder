package audio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/odsod/recorder/internal/protocol/parec"
)

type ParecCapture struct {
	client  *parec.Client
	monitor string
	mic     string
	stop    func()
}

func NewParecCapture(client *parec.Client) *ParecCapture {
	return &ParecCapture{client: client}
}

func (c *ParecCapture) MonitorSource() string {
	return c.monitor
}

func (c *ParecCapture) MicSource() string {
	return c.mic
}

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

func (c *ParecCapture) Stop() error {
	if c.stop != nil {
		c.stop()
	}
	return nil
}
