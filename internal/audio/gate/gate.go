package gate

import "github.com/odsod/recorder/internal/audio/pcm"

// Default RMS thresholds for speech detection and chunk filtering.
const (
	DefaultFrameThreshold = 0.002
	DefaultChunkThreshold = 0.0025
)

// Config holds per-frame and per-chunk RMS gate thresholds.
type Config struct {
	FrameThreshold float64
	ChunkThreshold float64
}

// Default returns production gate thresholds.
func Default() Config {
	return Config{
		FrameThreshold: DefaultFrameThreshold,
		ChunkThreshold: DefaultChunkThreshold,
	}
}

// Result holds RMS measurements and whether the gate passed.
type Result struct {
	SysRMS float64
	MicRMS float64
	Passes bool
}

// FrameHasSpeech reports whether either channel exceeds the frame threshold.
func (g Config) FrameHasSpeech(sys, mic []byte) Result {
	sysRMS := pcm.ComputeRMS(sys)
	micRMS := pcm.ComputeRMS(mic)
	return Result{
		SysRMS: sysRMS,
		MicRMS: micRMS,
		Passes: sysRMS >= g.FrameThreshold || micRMS >= g.FrameThreshold,
	}
}

// ChunkPasses reports whether either channel exceeds the chunk threshold.
func (g Config) ChunkPasses(sys, mic []byte) Result {
	sysRMS := pcm.ComputeRMS(sys)
	micRMS := pcm.ComputeRMS(mic)
	return Result{
		SysRMS: sysRMS,
		MicRMS: micRMS,
		Passes: sysRMS >= g.ChunkThreshold || micRMS >= g.ChunkThreshold,
	}
}
