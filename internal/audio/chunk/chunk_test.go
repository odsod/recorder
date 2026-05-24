package chunk

import (
	"testing"
	"time"

	"github.com/odsod/recorder/internal/audio/pcm"
)

func testConfig() Config {
	return Config{
		FrameBytes:       pcm.FrameBytes,
		MinSecs:          3,
		MaxSecs:          5,
		SilentDiscardSec: 2,
		SilenceEmitSec:   1,
	}
}

func frame() []byte {
	return make([]byte, pcm.FrameBytes)
}

func ingestFrames(a *Accumulator, n int, hasSpeech bool, start time.Time) Output {
	var out Output
	for i := range n {
		out = a.Ingest(frame(), frame(), start.Add(time.Duration(i)*time.Second), hasSpeech)
	}
	return out
}

func TestAccumulator_DiscardSilentBuffer(t *testing.T) {
	a := New(testConfig())
	start := time.Now()

	out := ingestFrames(a, 2, false, start)
	if out.Action != ActionDiscard {
		t.Errorf("Action = %v, want ActionDiscard", out.Action)
	}
}

func TestAccumulator_NoEmitBeforeMinDuration(t *testing.T) {
	a := New(testConfig())
	start := time.Now()

	out := ingestFrames(a, 2, true, start)
	if out.Action != ActionNone {
		t.Errorf("Action = %v, want ActionNone", out.Action)
	}
}

func TestAccumulator_EmitAfterMinAndSilence(t *testing.T) {
	a := New(testConfig())
	start := time.Now()

	ingestFrames(a, 3, true, start)
	out := a.Ingest(frame(), frame(), start.Add(3*time.Second), false)
	if out.Action != ActionEmit {
		t.Fatalf("Action = %v, want ActionEmit", out.Action)
	}
	if len(out.SysPCM) != 4*pcm.FrameBytes {
		t.Errorf("emitted len = %d, want %d", len(out.SysPCM), 4*pcm.FrameBytes)
	}
	if !out.StartTime.Equal(start) {
		t.Errorf("StartTime = %v, want %v", out.StartTime, start)
	}
}

func TestAccumulator_EmitAtMaxDuration(t *testing.T) {
	a := New(testConfig())
	start := time.Now()

	out := ingestFrames(a, 5, true, start)
	if out.Action != ActionEmit {
		t.Fatalf("Action = %v, want ActionEmit", out.Action)
	}
	if len(out.SysPCM) != 5*pcm.FrameBytes {
		t.Errorf("emitted len = %d, want %d", len(out.SysPCM), 5*pcm.FrameBytes)
	}
}

func TestAccumulator_NoEmitWithPartialSilence(t *testing.T) {
	cfg := testConfig()
	cfg.MinSecs = 10
	a := New(cfg)
	start := time.Now()

	ingestFrames(a, 2, true, start)
	out := a.Ingest(frame(), frame(), start.Add(2*time.Second), false)
	if out.Action != ActionNone {
		t.Errorf("Action = %v, want ActionNone (below min duration)", out.Action)
	}
	if out.ConsecutiveSilentSecs != 1 {
		t.Errorf("ConsecutiveSilentSecs = %d, want 1", out.ConsecutiveSilentSecs)
	}
}

func TestAccumulator_Flush(t *testing.T) {
	a := New(testConfig())
	start := time.Now()

	ingestFrames(a, 3, true, start)
	out, ok := a.Flush()
	if !ok {
		t.Fatal("Flush() = false, want true")
	}
	if out.Action != ActionEmit {
		t.Errorf("Action = %v, want ActionEmit", out.Action)
	}
	if len(out.SysPCM) != 3*pcm.FrameBytes {
		t.Errorf("emitted len = %d, want %d", len(out.SysPCM), 3*pcm.FrameBytes)
	}
}

func TestAccumulator_FlushTooShort(t *testing.T) {
	a := New(testConfig())
	start := time.Now()

	ingestFrames(a, 2, true, start)
	if _, ok := a.Flush(); ok {
		t.Error("Flush() = true, want false for buffer below minimum")
	}
}

func TestAccumulator_FlushNoSpeech(t *testing.T) {
	a := New(testConfig())
	start := time.Now()

	ingestFrames(a, 3, false, start)
	if _, ok := a.Flush(); ok {
		t.Error("Flush() = true, want false without speech")
	}
}

func TestAccumulator_ResetAfterEmit(t *testing.T) {
	a := New(testConfig())
	start := time.Now()

	ingestFrames(a, 3, true, start)
	a.Ingest(frame(), frame(), start.Add(3*time.Second), false)

	out := a.Ingest(frame(), frame(), start.Add(4*time.Second), true)
	if out.Action != ActionNone {
		t.Errorf("Action after reset = %v, want ActionNone", out.Action)
	}
	if !out.HasSpeech {
		t.Error("HasSpeech = false after speech frame, want true")
	}
	if out.ConsecutiveSilentSecs != 0 {
		t.Errorf("ConsecutiveSilentSecs = %d, want 0", out.ConsecutiveSilentSecs)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.FrameBytes != pcm.FrameBytes {
		t.Errorf("FrameBytes = %d, want %d", cfg.FrameBytes, pcm.FrameBytes)
	}
	if cfg.MinSecs != DefaultMinSecs {
		t.Errorf("MinSecs = %d, want %d", cfg.MinSecs, DefaultMinSecs)
	}
}
