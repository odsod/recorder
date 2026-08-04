package capture

import (
	"context"
	"sync"
	"time"

	"github.com/odsod/recorder/internal/audio/frame"
	"github.com/odsod/recorder/internal/audio/pcm"
	"github.com/odsod/recorder/internal/protocol/parec"
)

// tickInterval is the mix loop's emission period, matching pcm.FrameBytes'
// one second of audio. A package-level var so tests can shrink it.
var tickInterval = time.Second

// Parec implements Source by dynamically monitoring every PulseAudio sink
// plus the default microphone, mixing simultaneous sink audio together.
type Parec struct {
	client sinkClient

	mu      sync.Mutex
	sinks   map[string]*reader // keyed by sink name
	mic     *reader
	micName string

	sinkBackoff backoff
	sinkLastTry time.Time
	micBackoff  backoff
	micLastTry  time.Time

	cancel   context.CancelFunc
	wg       sync.WaitGroup
	stopOnce sync.Once
}

// NewParec creates a Parec source using the given parec protocol client.
func NewParec(client *parec.Client) *Parec {
	return &Parec{client: client, sinks: make(map[string]*reader)}
}

// Start begins dynamic sink and microphone capture, returning a channel of
// mixed frames. Discovery and stream startup happen in the background;
// outages (no sinks, unreachable PulseAudio, a dead parec process) surface
// as silent frames rather than failing Start or closing the channel — the
// channel only closes once ctx is done.
func (c *Parec) Start(ctx context.Context) (<-chan frame.Dual, error) {
	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	frames := make(chan frame.Dual, 2)

	c.wg.Go(func() { c.reconcileLoop(runCtx) })
	c.wg.Go(func() { c.mixLoop(runCtx, frames) })

	return frames, nil
}

func (c *Parec) mixLoop(ctx context.Context, out chan<- frame.Dual) {
	defer close(out)
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f := c.mixTick()
			select {
			case out <- f:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (c *Parec) mixTick() frame.Dual {
	c.mu.Lock()
	sinkFrames := make([][]byte, 0, len(c.sinks))
	for _, r := range c.sinks {
		if data, ok := r.take(); ok {
			sinkFrames = append(sinkFrames, data)
		}
	}
	micReader := c.mic
	c.mu.Unlock()

	mic := frame.Silent(pcm.FrameBytes)
	if micReader != nil {
		if data, ok := micReader.take(); ok {
			mic = data
		}
	}

	return frame.Dual{Sys: pcm.Mix(sinkFrames...), Mic: mic}
}

// Stop terminates all capture streams. Idempotent.
func (c *Parec) Stop() error {
	c.stopOnce.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		c.wg.Wait()

		c.mu.Lock()
		for _, r := range c.sinks {
			_ = r.stop()
		}
		if c.mic != nil {
			_ = c.mic.stop()
		}
		c.mu.Unlock()
	})
	return nil
}
