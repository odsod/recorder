package audio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

type Frame struct {
	Sys []byte
	Mic []byte
}

type Capture interface {
	Start(ctx context.Context) (<-chan Frame, error)
	Stop() error
	MonitorSource() string
	MicSource() string
}

type ParecCapture struct {
	monitor string
	mic     string
	sysCmd  *exec.Cmd
	micCmd  *exec.Cmd
	stop    func()
}

func NewParecCapture() *ParecCapture {
	return &ParecCapture{}
}

func (c *ParecCapture) MonitorSource() string {
	return c.monitor
}

func (c *ParecCapture) MicSource() string {
	return c.mic
}

func (c *ParecCapture) Start(ctx context.Context) (<-chan Frame, error) {
	monitor, err := GetMonitorSource(ctx)
	if err != nil {
		return nil, err
	}
	micSource, err := GetMicSource(ctx)
	if err != nil {
		return nil, err
	}
	c.monitor = monitor
	c.mic = micSource

	sysCmd, sysReader, err := StartParec(ctx, monitor, SampleRate)
	if err != nil {
		return nil, fmt.Errorf("start sys parec: %w", err)
	}
	micCmd, micReader, err := StartParec(ctx, micSource, SampleRate)
	if err != nil {
		_ = sysCmd.Process.Kill()
		_ = sysCmd.Wait()
		return nil, fmt.Errorf("start mic parec: %w", err)
	}
	c.sysCmd = sysCmd
	c.micCmd = micCmd

	frames := make(chan Frame, 2)
	done := make(chan struct{})
	var once sync.Once
	c.stop = func() {
		once.Do(func() {
			close(done)
			_ = sysCmd.Process.Kill()
			_ = sysCmd.Wait()
			_ = micCmd.Process.Kill()
			_ = micCmd.Wait()
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

			sysData, err := ReadFrame(sysReader)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					return
				}
				return
			}
			micData, err := ReadFrame(micReader)
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
