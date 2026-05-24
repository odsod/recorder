package audio

import (
	"context"
	"io"
)

// ReadFrame reads exactly one audio frame (FrameBytes) from the reader.
func ReadFrame(r io.Reader) ([]byte, error) {
	buf := make([]byte, FrameBytes)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// SilentFrame returns a zero-filled frame representing silence.
func SilentFrame() []byte {
	return make([]byte, FrameBytes)
}

// Frame holds one system and one microphone audio frame.
type Frame struct {
	Sys []byte
	Mic []byte
}

// Capture abstracts dual-channel audio capture (system + microphone).
type Capture interface {
	Start(ctx context.Context) (<-chan Frame, error)
	Stop() error
	MonitorSource() string
	MicSource() string
}
