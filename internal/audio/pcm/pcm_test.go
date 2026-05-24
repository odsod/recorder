package pcm

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestComputeRMS_Silence(t *testing.T) {
	pcm := make([]byte, FrameBytes)
	rms := ComputeRMS(pcm)
	if rms != 0.0 {
		t.Errorf("silence RMS = %v, want 0.0", rms)
	}
}

func TestComputeRMS_MaxAmplitude(t *testing.T) {
	pcm := make([]byte, 100*2)
	for i := range 100 {
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(math.MaxInt16))
	}
	rms := ComputeRMS(pcm)
	if rms < 0.99 || rms > 1.0 {
		t.Errorf("max amplitude RMS = %v, want ~1.0", rms)
	}
}

func TestComputeRMS_KnownSignal(t *testing.T) {
	pcm := make([]byte, 1000*2)
	for i := range 1000 {
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(int16(16384)))
	}
	rms := ComputeRMS(pcm)
	if math.Abs(rms-0.5) > 0.001 {
		t.Errorf("half amplitude RMS = %v, want ~0.5", rms)
	}
}

func TestComputeRMS_TooShort(t *testing.T) {
	rms := ComputeRMS([]byte{0})
	if rms != 0.0 {
		t.Errorf("single byte RMS = %v, want 0.0", rms)
	}
}

func TestComputeRMS_Empty(t *testing.T) {
	rms := ComputeRMS(nil)
	if rms != 0.0 {
		t.Errorf("nil RMS = %v, want 0.0", rms)
	}
}

func TestFrameCount(t *testing.T) {
	tests := []struct {
		name       string
		pcm        []byte
		frameBytes int
		want       int
	}{
		{"exact", make([]byte, FrameBytes*3), FrameBytes, 3},
		{"partial", make([]byte, FrameBytes+1), FrameBytes, 1},
		{"zero frame size", make([]byte, 10), 0, 0},
		{"empty", nil, FrameBytes, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FrameCount(tt.pcm, tt.frameBytes); got != tt.want {
				t.Errorf("FrameCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestTrimTrailingFrames(t *testing.T) {
	pcm := make([]byte, FrameBytes*3)
	tests := []struct {
		name       string
		frames     int
		frameBytes int
		wantLen    int
	}{
		{"no trim", 0, FrameBytes, len(pcm)},
		{"one frame", 1, FrameBytes, FrameBytes * 2},
		{"negative frames", -1, FrameBytes, len(pcm)},
		{"trim all invalid", 3, FrameBytes, len(pcm)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TrimTrailingFrames(pcm, tt.frames, tt.frameBytes)
			if len(got) != tt.wantLen {
				t.Errorf("TrimTrailingFrames() len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}
