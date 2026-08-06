package capture

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/odsod/recorder/internal/audio/pcm"
	"github.com/odsod/recorder/internal/protocol/parec"
)

// fakeRunner backs a real *parec.Client so fakeSinkClient can produce real
// *parec.CaptureStream values, matching the existing hand-rolled-fake
// convention used by internal/protocol/parec's own tests.
type fakeRunner struct {
	mu      sync.Mutex
	streams map[string]func() (io.ReadCloser, func() error, error)
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{streams: make(map[string]func() (io.ReadCloser, func() error, error))}
}

func (f *fakeRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	panic("fakeRunner.Output should not be called; capture talks to fakeSinkClient directly")
}

func (f *fakeRunner) Start(ctx context.Context, name string, args ...string) (io.ReadCloser, func() error, error) {
	device := deviceArg(args)
	f.mu.Lock()
	fn, ok := f.streams[device]
	f.mu.Unlock()
	if !ok {
		return io.NopCloser(bytes.NewReader(nil)), func() error { return nil }, nil
	}
	return fn()
}

func deviceArg(args []string) string {
	for _, a := range args {
		if len(a) > len("--device=") && a[:len("--device=")] == "--device=" {
			return a[len("--device="):]
		}
	}
	return ""
}

// setStream registers repeating frame data for a device: each StartCapture
// call for that device gets a fresh reader over data, repeated indefinitely
// so tests don't need to size data to an exact number of ticks.
func (f *fakeRunner) setStream(device string, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.streams[device] = func() (io.ReadCloser, func() error, error) {
		return io.NopCloser(&repeatingReader{data: data}), func() error { return nil }, nil
	}
}

// setStreamOnce registers one-shot frame data (EOF after data is exhausted),
// used to simulate a reader dying.
func (f *fakeRunner) setStreamOnce(device string, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.streams[device] = func() (io.ReadCloser, func() error, error) {
		return io.NopCloser(bytes.NewReader(data)), func() error { return nil }, nil
	}
}

// repeatingReader loops over data forever, so a fake capture stream never
// looks like it died from the reader's perspective.
type repeatingReader struct {
	data []byte
	pos  int
}

func (r *repeatingReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	if r.pos >= len(r.data) {
		r.pos = 0
	}
	return n, nil
}

// fakeSinkClient implements sinkClient directly (no JSON/CommandRunner
// involved), for scripting reconciliation scenarios plainly.
type fakeSinkClient struct {
	mu         sync.Mutex
	runner     *fakeRunner
	client     *parec.Client
	sinks      []parec.Sink
	sinksErr   error
	sources    []parec.Source
	sourcesErr error
	startErrFn func(device string) error
}

func newFakeSinkClient() *fakeSinkClient {
	runner := newFakeRunner()
	return &fakeSinkClient{runner: runner, client: parec.New(runner)}
}

func (f *fakeSinkClient) ListSinks(ctx context.Context, _ parec.ListSinksRequest) (parec.ListSinksResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sinksErr != nil {
		return parec.ListSinksResponse{}, f.sinksErr
	}
	return parec.ListSinksResponse{Sinks: f.sinks}, nil
}

func (f *fakeSinkClient) ListSources(
	ctx context.Context,
	_ parec.ListSourcesRequest,
) (parec.ListSourcesResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sourcesErr != nil {
		return parec.ListSourcesResponse{}, f.sourcesErr
	}
	return parec.ListSourcesResponse{Sources: f.sources}, nil
}

func (f *fakeSinkClient) StartCapture(
	ctx context.Context,
	req parec.StartCaptureRequest,
) (*parec.CaptureStream, error) {
	f.mu.Lock()
	startErrFn := f.startErrFn
	f.mu.Unlock()
	if startErrFn != nil {
		if err := startErrFn(req.Device); err != nil {
			return nil, err
		}
	}
	return f.client.StartCapture(ctx, req)
}

func (f *fakeSinkClient) setSinks(sinks ...parec.Sink) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sinks = sinks
}

func (f *fakeSinkClient) setSinksErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sinksErr = err
}

func (f *fakeSinkClient) setSources(sources ...parec.Source) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sources = sources
}

func (f *fakeSinkClient) setSourcesErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sourcesErr = err
}

// waitFor polls cond until it returns true or the timeout elapses.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	if !cond() {
		t.Fatal("condition not met before timeout")
	}
}

func withFastPolling(t *testing.T) {
	t.Helper()
	origSink, origMic, origTick := sinkPollInterval, micPollInterval, tickInterval
	origInitial, origMax := backoffInitial, backoffMax
	sinkPollInterval = 5 * time.Millisecond
	micPollInterval = 5 * time.Millisecond
	tickInterval = 5 * time.Millisecond
	backoffInitial = 5 * time.Millisecond
	backoffMax = 20 * time.Millisecond
	t.Cleanup(func() {
		sinkPollInterval, micPollInterval, tickInterval = origSink, origMic, origTick
		backoffInitial, backoffMax = origInitial, origMax
	})
}

func TestParec_MultiSinkMixing(t *testing.T) {
	withFastPolling(t)
	client := newFakeSinkClient()
	client.runner.setStream("a.monitor", bytes.Repeat([]byte{0x01, 0x00}, pcm.FrameBytes/2))
	client.runner.setStream("b.monitor", bytes.Repeat([]byte{0x02, 0x00}, pcm.FrameBytes/2))
	client.setSinks(
		parec.Sink{Name: "a", MonitorSource: "a.monitor"},
		parec.Sink{Name: "b", MonitorSource: "b.monitor"},
	)

	src := &Parec{client: client, sinks: make(map[string]*reader), mics: make(map[string]*reader)}
	frames, err := src.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Stop() }()

	f := <-frames
	for f.Sys[0] == 0 {
		f = <-frames
	}
	if f.Sys[0] != 0x03 {
		t.Errorf("expected mixed sys byte 0x03, got %#x", f.Sys[0])
	}
}

func TestParec_SinkAppears(t *testing.T) {
	withFastPolling(t)
	client := newFakeSinkClient()
	client.runner.setStream("a.monitor", bytes.Repeat([]byte{0x05, 0x00}, pcm.FrameBytes/2))

	src := &Parec{client: client, sinks: make(map[string]*reader), mics: make(map[string]*reader)}
	frames, err := src.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Stop() }()

	client.setSinks(parec.Sink{Name: "a", MonitorSource: "a.monitor"})

	found := false
	for range 200 {
		f := <-frames
		if f.Sys[0] == 0x05 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected sink audio to appear after discovery")
	}
}

func TestParec_SinkDisappears(t *testing.T) {
	withFastPolling(t)
	client := newFakeSinkClient()
	client.runner.setStream("a.monitor", bytes.Repeat([]byte{0x05, 0x00}, pcm.FrameBytes/2))
	client.runner.setStream("b.monitor", bytes.Repeat([]byte{0x06, 0x00}, pcm.FrameBytes/2))
	client.setSinks(
		parec.Sink{Name: "a", MonitorSource: "a.monitor"},
		parec.Sink{Name: "b", MonitorSource: "b.monitor"},
	)

	src := &Parec{client: client, sinks: make(map[string]*reader), mics: make(map[string]*reader)}
	if _, err := src.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Stop() }()

	waitFor(t, time.Second, func() bool {
		src.mu.Lock()
		defer src.mu.Unlock()
		return len(src.sinks) == 2
	})

	client.setSinks(parec.Sink{Name: "b", MonitorSource: "b.monitor"})

	waitFor(t, time.Second, func() bool {
		src.mu.Lock()
		defer src.mu.Unlock()
		_, hasA := src.sinks["a"]
		_, hasB := src.sinks["b"]
		return !hasA && hasB
	})
}

func TestParec_SinkReappearsNewIndex(t *testing.T) {
	withFastPolling(t)
	client := newFakeSinkClient()
	client.runner.setStream("a.monitor", bytes.Repeat([]byte{0x05, 0x00}, pcm.FrameBytes/2))
	client.setSinks(parec.Sink{Name: "a", MonitorSource: "a.monitor"})

	src := &Parec{client: client, sinks: make(map[string]*reader), mics: make(map[string]*reader)}
	_, err := src.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Stop() }()

	waitFor(t, time.Second, func() bool {
		src.mu.Lock()
		defer src.mu.Unlock()
		_, ok := src.sinks["a"]
		return ok
	})

	client.setSinks()
	waitFor(t, time.Second, func() bool {
		src.mu.Lock()
		defer src.mu.Unlock()
		_, ok := src.sinks["a"]
		return !ok
	})

	client.setSinks(parec.Sink{Name: "a", MonitorSource: "a.monitor"})
	waitFor(t, time.Second, func() bool {
		src.mu.Lock()
		defer src.mu.Unlock()
		_, ok := src.sinks["a"]
		return ok
	})
}

func TestParec_MicSourceAppears(t *testing.T) {
	withFastPolling(t)
	client := newFakeSinkClient()
	client.runner.setStream("mic1", bytes.Repeat([]byte{0x07, 0x00}, pcm.FrameBytes/2))
	client.runner.setStream("mic2", bytes.Repeat([]byte{0x08, 0x00}, pcm.FrameBytes/2))
	client.setSources(parec.Source{Name: "mic1"})

	src := &Parec{client: client, sinks: make(map[string]*reader), mics: make(map[string]*reader)}
	frames, err := src.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Stop() }()

	waitFor(t, time.Second, func() bool {
		src.mu.Lock()
		defer src.mu.Unlock()
		_, ok := src.mics["mic1"]
		return ok
	})

	client.setSources(parec.Source{Name: "mic1"}, parec.Source{Name: "mic2"})
	waitFor(t, time.Second, func() bool {
		src.mu.Lock()
		defer src.mu.Unlock()
		_, ok := src.mics["mic2"]
		return ok
	})

	found := false
	for range 200 {
		f := <-frames
		if f.Mic[0] == 0x08 || f.Mic[0] == 0x0f {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected mic audio from new source after discovery")
	}
}

func TestParec_MicCaptureFailureDoesNotAffectOthers(t *testing.T) {
	withFastPolling(t)
	client := newFakeSinkClient()
	client.runner.setStream("mic1", bytes.Repeat([]byte{0x07, 0x00}, pcm.FrameBytes/2))
	client.setSources(parec.Source{Name: "mic1"})

	src := &Parec{client: client, sinks: make(map[string]*reader), mics: make(map[string]*reader)}
	_, err := src.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Stop() }()

	waitFor(t, time.Second, func() bool {
		src.mu.Lock()
		defer src.mu.Unlock()
		_, ok := src.mics["mic1"]
		return ok
	})

	client.mu.Lock()
	client.startErrFn = func(device string) error {
		if device == "mic2" {
			return io.ErrClosedPipe
		}
		return nil
	}
	client.mu.Unlock()
	client.setSources(parec.Source{Name: "mic1"}, parec.Source{Name: "mic2"})

	time.Sleep(100 * time.Millisecond)

	src.mu.Lock()
	_, hasMic1 := src.mics["mic1"]
	_, hasMic2 := src.mics["mic2"]
	src.mu.Unlock()
	if !hasMic1 {
		t.Error("expected mic1 to still be active")
	}
	if hasMic2 {
		t.Error("expected mic2 to not be active after failed start")
	}
}

func TestParec_ReaderExitTriggersRestartWithoutKillingOthers(t *testing.T) {
	withFastPolling(t)
	client := newFakeSinkClient()
	client.runner.setStreamOnce("a.monitor", bytes.Repeat([]byte{0x05, 0x00}, pcm.FrameBytes/2))
	client.runner.setStream("b.monitor", bytes.Repeat([]byte{0x06, 0x00}, pcm.FrameBytes/2))
	client.setSinks(
		parec.Sink{Name: "a", MonitorSource: "a.monitor"},
		parec.Sink{Name: "b", MonitorSource: "b.monitor"},
	)

	src := &Parec{client: client, sinks: make(map[string]*reader), mics: make(map[string]*reader)}
	_, err := src.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Stop() }()

	waitFor(t, time.Second, func() bool {
		src.mu.Lock()
		defer src.mu.Unlock()
		_, hasA := src.sinks["a"]
		_, hasB := src.sinks["b"]
		return hasA && hasB
	})

	src.mu.Lock()
	bReaderBefore := src.sinks["b"]
	src.mu.Unlock()

	// Once "a"'s one-shot stream is exhausted, its reader dies; the next
	// sink reconciliation pass must restart it in place without touching "b".
	waitFor(t, time.Second, func() bool {
		src.mu.Lock()
		defer src.mu.Unlock()
		a, ok := src.sinks["a"]
		if !ok {
			return false
		}
		select {
		case <-a.dead():
			return false // not yet restarted
		default:
			return true
		}
	})

	src.mu.Lock()
	bReaderAfter := src.sinks["b"]
	src.mu.Unlock()
	if bReaderBefore != bReaderAfter {
		t.Error("sink b's reader should not have been restarted")
	}
}

func TestParec_PulseAudioUnreachableThenRecovers(t *testing.T) {
	withFastPolling(t)
	client := newFakeSinkClient()
	client.setSinksErr(io.ErrClosedPipe)
	client.setSourcesErr(io.ErrClosedPipe)

	src := &Parec{client: client, sinks: make(map[string]*reader), mics: make(map[string]*reader)}
	frames, err := src.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Stop() }()

	for range 3 {
		f := <-frames
		if !isSilent(f.Sys) || !isSilent(f.Mic) {
			t.Error("expected silence while PulseAudio is unreachable")
		}
	}

	client.runner.setStream("a.monitor", bytes.Repeat([]byte{0x09, 0x00}, pcm.FrameBytes/2))
	client.setSinksErr(nil)
	client.setSinks(parec.Sink{Name: "a", MonitorSource: "a.monitor"})
	client.setSourcesErr(nil)
	client.setSources(parec.Source{Name: "mic1"})
	client.runner.setStream("mic1", bytes.Repeat([]byte{0x0a, 0x00}, pcm.FrameBytes/2))

	waitFor(t, 2*time.Second, func() bool {
		src.mu.Lock()
		defer src.mu.Unlock()
		_, hasSink := src.sinks["a"]
		_, hasMic := src.mics["mic1"]
		return hasSink && hasMic
	})
}

func isSilent(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return false
		}
	}
	return true
}

func TestParec_Stop_Idempotent(t *testing.T) {
	withFastPolling(t)
	client := newFakeSinkClient()
	src := &Parec{client: client, sinks: make(map[string]*reader), mics: make(map[string]*reader)}
	if _, err := src.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := src.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := src.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestParec_Backpressure_SlowConsumerDoesNotDropFrames(t *testing.T) {
	withFastPolling(t)
	client := newFakeSinkClient()
	src := &Parec{client: client, sinks: make(map[string]*reader), mics: make(map[string]*reader)}
	frames, err := src.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Stop() }()

	// Don't read frames for a while; mixLoop must block on send rather than
	// panic or busy-loop, and Stop() must still terminate cleanly afterward.
	time.Sleep(50 * time.Millisecond)

	drained := 0
	timeout := time.After(time.Second)
loop:
	for drained < 3 {
		select {
		case <-frames:
			drained++
		case <-timeout:
			break loop
		}
	}
	if drained < 3 {
		t.Errorf("expected to drain buffered/backlogged frames, got %d", drained)
	}
}
