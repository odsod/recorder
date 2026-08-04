package pcm

import (
	"bytes"
	"encoding/binary"
	"math"
	"slices"
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

func sampleFrame(t *testing.T, samples ...int16) []byte {
	t.Helper()
	buf := make([]byte, FrameBytes)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(s))
	}
	return buf
}

func readSamples(t *testing.T, data []byte, n int) []int16 {
	t.Helper()
	out := make([]int16, n)
	for i := range n {
		out[i] = int16(binary.LittleEndian.Uint16(data[i*2:]))
	}
	return out
}

func TestMix_NoInputs(t *testing.T) {
	got := Mix()
	if len(got) != FrameBytes {
		t.Fatalf("len = %d, want %d", len(got), FrameBytes)
	}
	for _, b := range got {
		if b != 0 {
			t.Fatalf("expected silence, got non-zero byte")
		}
	}
}

func TestMix_SingleInput(t *testing.T) {
	f := sampleFrame(t, 100, -200, 300)
	got := Mix(f)
	if !bytes.Equal(got, f) {
		t.Errorf("single input should be unchanged")
	}
}

func TestMix_TwoInputsAdd(t *testing.T) {
	a := sampleFrame(t, 100, -200, 300)
	b := sampleFrame(t, 50, -50, -100)
	got := Mix(a, b)
	want := []int16{150, -250, 200}
	if s := readSamples(t, got, len(want)); !slices.Equal(s, want) {
		t.Errorf("Mix() = %v, want %v", s, want)
	}
}

func TestMix_SilentDoesNotAttenuate(t *testing.T) {
	active := sampleFrame(t, 1000, -1000, 500)
	silent := make([]byte, FrameBytes)
	got := Mix(active, silent)
	if !bytes.Equal(got, active) {
		t.Errorf("mixing with silence should leave active frame unchanged")
	}
}

func TestMix_PositiveOverflowClamps(t *testing.T) {
	a := sampleFrame(t, math.MaxInt16)
	b := sampleFrame(t, math.MaxInt16)
	got := Mix(a, b)
	want := []int16{math.MaxInt16}
	if s := readSamples(t, got, 1); !slices.Equal(s, want) {
		t.Errorf("Mix() = %v, want %v", s, want)
	}
}

func TestMix_NegativeOverflowClamps(t *testing.T) {
	a := sampleFrame(t, math.MinInt16)
	b := sampleFrame(t, math.MinInt16)
	got := Mix(a, b)
	want := []int16{math.MinInt16}
	if s := readSamples(t, got, 1); !slices.Equal(s, want) {
		t.Errorf("Mix() = %v, want %v", s, want)
	}
}

func TestMix_InputBuffersUnchanged(t *testing.T) {
	a := sampleFrame(t, 100, -200)
	b := sampleFrame(t, 50, -50)
	aCopy := append([]byte(nil), a...)
	bCopy := append([]byte(nil), b...)
	Mix(a, b)
	if !bytes.Equal(a, aCopy) || !bytes.Equal(b, bCopy) {
		t.Errorf("Mix() must not mutate its input buffers")
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
