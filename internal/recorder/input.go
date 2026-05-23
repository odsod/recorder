package recorder

import (
	"context"
	"os"
	"time"

	"golang.org/x/term"

	"github.com/odsod/recorder/internal/transcript"
)

func (r *Recorder) inputLoop(ctx context.Context) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return
	}
	defer term.Restore(fd, oldState)

	buf := make([]byte, 1)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return
		}

		switch buf[0] {
		case 'p':
			paused := !r.paused.Load()
			r.paused.Store(paused)
			if paused {
				r.log("paused")
			} else {
				r.log("listening")
			}
		case 's':
			now := time.Now()
			ts := now.Format("15:04:05")
			r.transcript.Append(ts, "📍 pin", "", nil)
			r.log(transcript.FormatMessage("📍 pin", "", nil))
			r.segmenter.OnPin(now)
		}
	}
}
