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
	c.reconcileMics(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-sinkTicker.C:
			c.reconcileSinks(ctx)
		case <-micTicker.C:
			c.reconcileMics(ctx)
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

// reconcileMics lists all input sources and diffs them against active mic
// readers by name. Same pattern as reconcileSinks.
func (c *Parec) reconcileMics(ctx context.Context) {
	if time.Since(c.micLastTry) < c.micBackoff.cur {
		return
	}
	c.micLastTry = time.Now()

	resp, err := c.client.ListSources(ctx, parec.ListSourcesRequest{})
	if err != nil {
		slog.WarnContext(ctx, "list sources failed",
			"err", err,
			"retryIn", c.micBackoff.next(),
		)
		return
	}
	c.micBackoff.reset()

	wanted := make(map[string]parec.Source, len(resp.Sources))
	for _, s := range resp.Sources {
		wanted[s.Name] = s
	}

	c.mu.Lock()
	current := make(map[string]*reader, len(c.mics))
	maps.Copy(current, c.mics)
	c.mu.Unlock()

	for name, r := range current {
		if _, ok := wanted[name]; !ok {
			c.stopMic(name, r, "source disappeared")
			continue
		}
		select {
		case <-r.dead():
			c.stopMic(name, r, "mic reader exited")
		default:
		}
	}

	for name := range wanted {
		c.mu.Lock()
		_, active := c.mics[name]
		c.mu.Unlock()
		if active {
			continue
		}

		stream, err := c.client.StartCapture(ctx, parec.StartCaptureRequest{
			Device: name, SampleRate: pcm.SampleRate,
		})
		if err != nil {
			slog.WarnContext(ctx, "mic capture start failed",
				"source", name,
				"err", err,
			)
			continue
		}

		r := startReader(stream)
		c.mu.Lock()
		c.mics[name] = r
		c.mu.Unlock()
		slog.InfoContext(ctx, "mic capture started", "source", name)
	}
}

func (c *Parec) stopMic(name string, r *reader, reason string) {
	c.mu.Lock()
	delete(c.mics, name)
	c.mu.Unlock()
	_ = r.stop()
	slog.InfoContext(context.Background(), "mic capture stopped",
		"source", name,
		"reason", reason,
	)
}
