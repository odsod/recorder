package audio

import (
	"encoding/binary"
	"math"
)

const (
	SampleRate        = 16000
	FrameBytes        = SampleRate * 2 // 1 second of s16le mono
	SpeechRMSThreshold = 0.002
	ChunkRMSThreshold  = 0.0025
	MinChunkSecs      = 10
	ChunkMaxSecs      = 45
)

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
