package capture

import (
	"context"
	"log/slog"
	"maps"
	"time"

	"github.com/odsod/recorder/internal/audio/pcm"
	"github.com/odsod/recorder/internal/protocol/parec"
)

// Poll intervals for sink and microphone discovery. Sinks and default
// sources change on human timescales (plugging in headphones, changing
// output device in a settings applet); periodic polling is sufficient and
// avoids a pactl subscribe event subsystem.
var (
	sinkPollInterval = 2 * time.Second
	micPollInterval  = 2 * time.Second
)

// backoff tracks a reset-on-success retry delay for one recovering
// operation. Not goroutine-safe by design: each failure axis (sinks, mic)
// owns its own instance, so failures on one never throttle the other.
type backoff struct {
	cur time.Duration
}

var (
	backoffInitial = 500 * time.Millisecond
	backoffMax     = 30 * time.Second
)

func (b *backoff) next() time.Duration {
	if b.cur == 0 {
		b.cur = backoffInitial
	} else {
		b.cur = min(b.cur*2, backoffMax)
	}
	return b.cur
}

func (b *backoff) reset() {
	b.cur = 0
}

func (c *Parec) reconcileLoop(ctx context.Context) {
	sinkTicker := time.NewTicker(sinkPollInterval)
	defer sinkTicker.Stop()
	micTicker := time.NewTicker(micPollInterval)
	defer micTicker.Stop()

	c.reconcileSinks(ctx)
	c.reconcileMic(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-sinkTicker.C:
			c.reconcileSinks(ctx)
		case <-micTicker.C:
			c.reconcileMic(ctx)
		}
	}
}

// reconcileSinks lists the current sinks and diffs them against active
// readers by name. A transient ListSinks failure leaves existing readers
// running untouched.
//
// Known limitation: virtual/loopback sink chains (e.g. an EasyEffects
// post-processing sink chained after a raw device sink) will be monitored
// as separate sinks and double-counted when mixed. Not solved here.
func (c *Parec) reconcileSinks(ctx context.Context) {
	if time.Since(c.sinkLastTry) < c.sinkBackoff.cur {
		return
	}
	c.sinkLastTry = time.Now()

	resp, err := c.client.ListSinks(ctx, parec.ListSinksRequest{})
	if err != nil {
		slog.WarnContext(ctx, "list sinks failed",
			"err", err,
			"retryIn", c.sinkBackoff.next(),
		)
		return
	}
	c.sinkBackoff.reset()

	wanted := make(map[string]parec.Sink, len(resp.Sinks))
	for _, s := range resp.Sinks {
		wanted[s.Name] = s
	}

	c.mu.Lock()
	current := make(map[string]*reader, len(c.sinks))
	maps.Copy(current, c.sinks)
	c.mu.Unlock()

	for name, r := range current {
		if _, ok := wanted[name]; !ok {
			c.stopSink(name, r, "sink disappeared")
			continue
		}
		select {
		case <-r.dead():
			c.stopSink(name, r, "sink reader exited")
		default:
		}
	}

	for name, sink := range wanted {
		c.mu.Lock()
		_, active := c.sinks[name]
		c.mu.Unlock()
		if active {
			continue
		}

		stream, err := c.client.StartCapture(ctx, parec.StartCaptureRequest{
			Device: sink.MonitorSource, SampleRate: pcm.SampleRate,
		})
		if err != nil {
			slog.WarnContext(ctx, "sink capture start failed",
				"sink", name,
				"err", err,
			)
			continue
		}

		r := startReader(stream)
		c.mu.Lock()
		c.sinks[name] = r
		c.mu.Unlock()
		slog.InfoContext(ctx, "sink capture started", "sink", name)
	}
}

func (c *Parec) stopSink(name string, r *reader, reason string) {
	c.mu.Lock()
	delete(c.sinks, name)
	c.mu.Unlock()
	_ = r.stop()
	slog.InfoContext(context.Background(), "sink capture stopped",
		"sink", name,
		"reason", reason,
	)
}

// reconcileMic resolves the default microphone source and swaps to it only
// after the replacement capture starts successfully, so a failed swap never
// drops below one working mic reader.
func (c *Parec) reconcileMic(ctx context.Context) {
	if time.Since(c.micLastTry) < c.micBackoff.cur {
		return
	}
	c.micLastTry = time.Now()

	resp, err := c.client.GetDefaultSource(ctx, parec.GetDefaultSourceRequest{})
	if err != nil {
		slog.WarnContext(ctx, "get default source failed",
			"err", err,
			"retryIn", c.micBackoff.next(),
		)
		return
	}
	c.micBackoff.reset()

	c.mu.Lock()
	name := resp.Source
	current := c.mic
	currentName := c.micName
	c.mu.Unlock()

	dead := false
	if current != nil {
		select {
		case <-current.dead():
			dead = true
		default:
		}
	}

	if current != nil && !dead && name == currentName {
		return
	}

	stream, err := c.client.StartCapture(ctx, parec.StartCaptureRequest{
		Device: name, SampleRate: pcm.SampleRate,
	})
	if err != nil {
		slog.WarnContext(ctx, "mic capture start failed",
			"source", name,
			"err", err,
		)
		return
	}

	r := startReader(stream)
	c.mu.Lock()
	c.mic = r
	c.micName = name
	c.mu.Unlock()

	if current != nil {
		_ = current.stop()
	}

	if currentName == "" {
		slog.InfoContext(ctx, "mic capture started", "source", name)
	} else {
		slog.InfoContext(ctx, "default microphone changed",
			"old", currentName,
			"new", name,
		)
	}
}
