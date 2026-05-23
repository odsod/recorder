package audio

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
	// Max amplitude int16 = 32767, normalized by 32768 → ~1.0
	if rms < 0.99 || rms > 1.0 {
		t.Errorf("max amplitude RMS = %v, want ~1.0", rms)
	}
}

func TestComputeRMS_KnownSignal(t *testing.T) {
	// 1000 samples at half amplitude (16384)
	pcm := make([]byte, 1000*2)
	for i := range 1000 {
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(int16(16384)))
	}
	rms := ComputeRMS(pcm)
	// RMS of constant 16384 = 16384/32768 = 0.5
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
