package pcm

import (
	"encoding/binary"
	"math"
)

// s16le mono capture format.
const (
	SampleRate = 16000
	FrameBytes = SampleRate * 2 // 1 second of s16le mono
)

// ComputeRMS computes the root-mean-square amplitude of 16-bit LE PCM data, normalized to [0,1].
func ComputeRMS(data []byte) float64 {
	if len(data) < 2 {
		return 0.0
	}
	n := len(data) / 2
	var sumSq float64
	for i := range n {
		sample := int16(binary.LittleEndian.Uint16(data[i*2 : i*2+2]))
		s := float64(sample)
		sumSq += s * s
	}
	return math.Sqrt(sumSq/float64(n)) / 32768.0
}

// FrameCount returns the number of complete frames in pcm.
func FrameCount(pcm []byte, frameBytes int) int {
	if frameBytes <= 0 {
		return 0
	}
	return len(pcm) / frameBytes
}

// TrimTrailingFrames removes trailing silent frames from pcm.
func TrimTrailingFrames(pcm []byte, frames, frameBytes int) []byte {
	if frames <= 0 || frameBytes <= 0 {
		return pcm
	}
	trimBytes := frames * frameBytes
	if trimBytes > 0 && trimBytes < len(pcm) {
		return pcm[:len(pcm)-trimBytes]
	}
	return pcm
}

// Mix combines s16le mono PCM frames via saturating addition. Silent or idle
// inputs (all-zero) don't attenuate active ones the way averaging would.
// Frames are expected to be FrameBytes long; shorter frames contribute only
// their available samples. Mixing zero frames returns FrameBytes of silence.
func Mix(frames ...[]byte) []byte {
	out := make([]byte, FrameBytes)
	for _, f := range frames {
		n := min(len(f), FrameBytes) / 2
		for i := range n {
			a := int16(binary.LittleEndian.Uint16(out[i*2 : i*2+2]))
			b := int16(binary.LittleEndian.Uint16(f[i*2 : i*2+2]))
			binary.LittleEndian.PutUint16(out[i*2:i*2+2], uint16(saturatingAdd(a, b)))
		}
	}
	return out
}

func saturatingAdd(a, b int16) int16 {
	sum := int32(a) + int32(b)
	switch {
	case sum > math.MaxInt16:
		return math.MaxInt16
	case sum < math.MinInt16:
		return math.MinInt16
	default:
		return int16(sum)
	}
}
