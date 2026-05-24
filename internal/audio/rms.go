package audio

import (
	"encoding/binary"
	"math"
)

// Audio capture constants.
const (
	SampleRate         = 16000
	FrameBytes         = SampleRate * 2 // 1 second of s16le mono
	SpeechRMSThreshold = 0.002
	ChunkRMSThreshold  = 0.0025
	MinChunkSecs       = 10
	ChunkMaxSecs       = 45
)

// ComputeRMS computes the root-mean-square amplitude of 16-bit LE PCM data, normalized to [0,1].
func ComputeRMS(pcm []byte) float64 {
	if len(pcm) < 2 {
		return 0.0
	}
	n := len(pcm) / 2
	var sumSq float64
	for i := range n {
		sample := int16(binary.LittleEndian.Uint16(pcm[i*2 : i*2+2]))
		s := float64(sample)
		sumSq += s * s
	}
	return math.Sqrt(sumSq/float64(n)) / 32768.0
}
