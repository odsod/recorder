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
