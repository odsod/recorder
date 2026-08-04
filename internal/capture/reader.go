package capture

import (
	"sync"

	"github.com/odsod/recorder/internal/audio/frame"
	"github.com/odsod/recorder/internal/audio/pcm"
	"github.com/odsod/recorder/internal/protocol/parec"
)

// reader continuously reads frames from a parec.CaptureStream and makes the
// most recent frame available without blocking capture. A slow or absent
// consumer never backs up the underlying parec process.
type reader struct {
	stream *parec.CaptureStream

	frameCh chan []byte
	done    chan struct{}

	stopOnce sync.Once
}

// startReader begins reading 1-second frames from stream in the background.
func startReader(stream *parec.CaptureStream) *reader {
	r := &reader{
		stream:  stream,
		frameCh: make(chan []byte, 1),
		done:    make(chan struct{}),
	}
	go r.run()
	return r
}

func (r *reader) run() {
	defer close(r.done)
	for {
		data, err := frame.Read(r.stream, pcm.FrameBytes)
		if err != nil {
			return
		}
		select {
		case r.frameCh <- data:
		default:
			<-r.frameCh
			r.frameCh <- data
		}
	}
}

// take returns the most recently read frame if one is available. It never
// blocks: if no frame has arrived since the last take, ok is false.
func (r *reader) take() (data []byte, ok bool) {
	select {
	case data := <-r.frameCh:
		return data, true
	default:
		return nil, false
	}
}

// dead returns a channel that is closed when the read loop has exited
// (stream EOF or error).
func (r *reader) dead() <-chan struct{} {
	return r.done
}

// stop terminates the underlying capture stream. Idempotent.
func (r *reader) stop() error {
	var err error
	r.stopOnce.Do(func() {
		err = r.stream.Close()
	})
	return err
}
