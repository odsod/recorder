package audio

import (
	"context"
	"io"
)

func ReadFrame(r io.Reader) ([]byte, error) {
	buf := make([]byte, FrameBytes)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func SilentFrame() []byte {
	return make([]byte, FrameBytes)
}

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
