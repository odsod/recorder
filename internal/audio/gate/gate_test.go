package gate

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/odsod/recorder/internal/audio/pcm"
)

func loudPCM(amplitude int16, samples int) []byte {
	pcmData := make([]byte, samples*2)
	for i := range samples {
		binary.LittleEndian.PutUint16(pcmData[i*2:], uint16(amplitude))
	}
	return pcmData
}

func TestFrameHasSpeech_BothSilent(t *testing.T) {
	g := Default()
	silent := make([]byte, pcm.FrameBytes)
	r := g.FrameHasSpeech(silent, silent)
	if r.Passes {
		t.Error("expected silent frames to fail frame gate")
	}
	if r.SysRMS != 0 || r.MicRMS != 0 {
		t.Errorf("RMS = sys %v mic %v, want 0", r.SysRMS, r.MicRMS)
	}
}

func TestFrameHasSpeech_SysOnly(t *testing.T) {
	g := Default()
	sys := loudPCM(500, 1000)
	silent := make([]byte, pcm.FrameBytes)
	r := g.FrameHasSpeech(sys, silent)
	if !r.Passes {
		t.Error("expected sys channel to pass frame gate")
	}
	if r.MicRMS != 0 {
		t.Errorf("mic RMS = %v, want 0", r.MicRMS)
	}
}

func TestFrameHasSpeech_MicOnly(t *testing.T) {
	g := Default()
	mic := loudPCM(500, 1000)
	silent := make([]byte, pcm.FrameBytes)
	r := g.FrameHasSpeech(silent, mic)
	if !r.Passes {
		t.Error("expected mic channel to pass frame gate")
	}
	if r.SysRMS != 0 {
		t.Errorf("sys RMS = %v, want 0", r.SysRMS)
	}
}

func TestFrameHasSpeech_AtThreshold(t *testing.T) {
	g := Config{FrameThreshold: 0.5, ChunkThreshold: 0.5}
	half := loudPCM(16384, 1000)
	r := g.FrameHasSpeech(half, nil)
	if !r.Passes {
		t.Error("expected half amplitude to pass threshold 0.5")
	}
	if math.Abs(r.SysRMS-0.5) > 0.001 {
		t.Errorf("sys RMS = %v, want ~0.5", r.SysRMS)
	}
}

func TestFrameHasSpeech_BelowThreshold(t *testing.T) {
	g := Config{FrameThreshold: 0.5, ChunkThreshold: 0.5}
	quiet := loudPCM(100, 1000)
	r := g.FrameHasSpeech(quiet, quiet)
	if r.Passes {
		t.Error("expected quiet signal to fail threshold 0.5")
	}
}

func TestChunkPasses_DifferentThreshold(t *testing.T) {
	g := Config{FrameThreshold: 0.002, ChunkThreshold: 0.01}
	signal := loudPCM(300, 1000)

	frame := g.FrameHasSpeech(signal, signal)
	chunk := g.ChunkPasses(signal, signal)

	if !frame.Passes {
		t.Error("expected frame gate to pass at lower threshold")
	}
	if chunk.Passes {
		t.Error("expected chunk gate to fail at higher threshold")
	}
}

func TestChunkPasses_EmptyPCM(t *testing.T) {
	g := Default()
	r := g.ChunkPasses(nil, nil)
	if r.Passes {
		t.Error("expected empty PCM to fail chunk gate")
	}
}
