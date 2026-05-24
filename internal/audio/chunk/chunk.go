package chunk

import (
	"slices"
	"time"

	"github.com/odsod/recorder/internal/audio/pcm"
)

// Dual-channel chunk accumulation tuning.
const (
	DefaultMinSecs          = 10
	DefaultMaxSecs          = 45
	DefaultSilentDiscardSec = 5
	DefaultSilenceEmitSec   = 1
)

// Action describes what the accumulator decided after ingesting a frame.
type Action int

const (
	// ActionNone means the buffer is still accumulating.
	ActionNone Action = iota
	// ActionEmit means a chunk is ready for transcription.
	ActionEmit
	// ActionDiscard means silent audio was dropped.
	ActionDiscard
)

// Config holds chunk boundary tuning parameters.
type Config struct {
	FrameBytes       int
	MinSecs          int
	MaxSecs          int
	SilentDiscardSec int
	SilenceEmitSec   int
}

// DefaultConfig returns production chunk accumulation settings.
func DefaultConfig() Config {
	return Config{
		FrameBytes:       pcm.FrameBytes,
		MinSecs:          DefaultMinSecs,
		MaxSecs:          DefaultMaxSecs,
		SilentDiscardSec: DefaultSilentDiscardSec,
		SilenceEmitSec:   DefaultSilenceEmitSec,
	}
}

// Output is the result of ingesting one frame or flushing buffered audio.
type Output struct {
	Action                Action
	SysPCM                []byte
	MicPCM                []byte
	StartTime             time.Time
	ConsecutiveSilentSecs int
	HasSpeech             bool
}

// Accumulator buffers dual-channel PCM and decides when to emit or discard chunks.
type Accumulator struct {
	cfg                   Config
	sysBuf                []byte
	micBuf                []byte
	hasSpeech             bool
	consecutiveSilentSecs int
	chunkStartTime        time.Time
}

// New creates an Accumulator with the given config.
func New(cfg Config) *Accumulator {
	return &Accumulator{cfg: cfg}
}

// Ingest appends one frame and updates speech/silence tracking.
func (a *Accumulator) Ingest(sys, mic []byte, at time.Time, hasSpeech bool) Output {
	a.ingest(sys, mic, at)
	if hasSpeech {
		a.hasSpeech = true
		a.consecutiveSilentSecs = 0
	} else {
		a.consecutiveSilentSecs++
	}

	out := Output{
		ConsecutiveSilentSecs: a.consecutiveSilentSecs,
		HasSpeech:             a.hasSpeech,
	}

	switch a.action() {
	case ActionDiscard:
		a.reset()
		out.Action = ActionDiscard
	case ActionEmit:
		out.Action = ActionEmit
		out.SysPCM, out.MicPCM = a.trimmedPCM()
		out.StartTime = a.chunkStartTime
		a.reset()
	}
	return out
}

// Flush emits a partial chunk on shutdown when speech was captured and the minimum duration is met.
func (a *Accumulator) Flush() (Output, bool) {
	if !a.hasSpeech || len(a.sysBuf) < a.cfg.MinSecs*a.cfg.FrameBytes {
		return Output{}, false
	}
	return Output{
		Action:    ActionEmit,
		SysPCM:    slices.Clone(a.sysBuf),
		MicPCM:    slices.Clone(a.micBuf),
		StartTime: a.chunkStartTime,
		HasSpeech: true,
	}, true
}

func (a *Accumulator) ingest(sys, mic []byte, at time.Time) {
	if a.chunkStartTime.IsZero() {
		a.chunkStartTime = at
	}
	a.sysBuf = append(a.sysBuf, sys...)
	a.micBuf = append(a.micBuf, mic...)
}

func (a *Accumulator) reset() {
	a.sysBuf = a.sysBuf[:0]
	a.micBuf = a.micBuf[:0]
	a.hasSpeech = false
	a.consecutiveSilentSecs = 0
	a.chunkStartTime = time.Time{}
}

func (a *Accumulator) action() Action {
	bufSecs := len(a.sysBuf) / a.cfg.FrameBytes

	if !a.hasSpeech && bufSecs >= a.cfg.SilentDiscardSec {
		return ActionDiscard
	}

	if a.hasSpeech && bufSecs >= a.cfg.MinSecs {
		if a.consecutiveSilentSecs >= a.cfg.SilenceEmitSec {
			return ActionEmit
		}
		if bufSecs >= a.cfg.MaxSecs {
			return ActionEmit
		}
	}
	return ActionNone
}

func (a *Accumulator) trimmedPCM() (sys, mic []byte) {
	trimFrames := max(0, a.consecutiveSilentSecs-a.cfg.SilenceEmitSec)
	return pcm.TrimTrailingFrames(a.sysBuf, trimFrames, a.cfg.FrameBytes),
		pcm.TrimTrailingFrames(a.micBuf, trimFrames, a.cfg.FrameBytes)
}
