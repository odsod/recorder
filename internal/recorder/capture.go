package recorder

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/odsod/recorder/internal/audio"
	"github.com/odsod/recorder/internal/transcript"
)

type captureAction int

const (
	actionNone captureAction = iota
	actionEmit
	actionDiscard
)

type captureState struct {
	sysBuf                []byte
	micBuf                []byte
	hasSpeech             bool
	consecutiveSilentSecs int
	wasSpeech             bool
	chunkStartTime        time.Time
}

func (s *captureState) reset() {
	s.sysBuf = s.sysBuf[:0]
	s.micBuf = s.micBuf[:0]
	s.hasSpeech = false
	s.consecutiveSilentSecs = 0
	s.wasSpeech = false
	s.chunkStartTime = time.Time{}
}

func (s *captureState) ingest(sysData, micData []byte) {
	if s.chunkStartTime.IsZero() {
		s.chunkStartTime = time.Now()
	}
	s.sysBuf = append(s.sysBuf, sysData...)
	s.micBuf = append(s.micBuf, micData...)
}

func (s *captureState) recordSpeech() {
	s.hasSpeech = true
	s.consecutiveSilentSecs = 0
}

func (s *captureState) recordSilence() {
	s.consecutiveSilentSecs++
}

func (s *captureState) action() captureAction {
	bufSecs := len(s.sysBuf) / audio.FrameBytes

	if !s.hasSpeech && bufSecs >= 5 {
		return actionDiscard
	}

	if s.hasSpeech && bufSecs >= audio.MinChunkSecs {
		if s.consecutiveSilentSecs >= 1 {
			return actionEmit
		}
		if bufSecs >= audio.ChunkMaxSecs {
			return actionEmit
		}
	}
	return actionNone
}

func (s *captureState) trimmedPCM() (sys, mic []byte) {
	trimBytes := max(0, s.consecutiveSilentSecs-1) * audio.FrameBytes
	if trimBytes > 0 && trimBytes < len(s.sysBuf) {
		return s.sysBuf[:len(s.sysBuf)-trimBytes], s.micBuf[:len(s.micBuf)-trimBytes]
	}
	return s.sysBuf, s.micBuf
}

func (r *Recorder) captureLoop(ctx context.Context, chunkCh chan<- AudioChunk) {
	frames, err := r.svc.Capture.Start(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "capture start failed",
			"err", err,
		)
		return
	}
	defer func() { _ = r.svc.Capture.Stop() }()

	slog.InfoContext(ctx, "system source configured",
		"source", r.svc.Capture.MonitorSource(),
	)
	slog.InfoContext(ctx, "mic source configured",
		"source", r.svc.Capture.MicSource(),
	)

	var state captureState
	slog.InfoContext(ctx, "listening")

	for {
		select {
		case <-ctx.Done():
			if state.hasSpeech && len(state.sysBuf) >= audio.MinChunkSecs*audio.FrameBytes {
				r.emitChunk(ctx, state.sysBuf, state.micBuf, state.chunkStartTime, chunkCh)
			}
			return
		case frame, ok := <-frames:
			if !ok {
				if state.hasSpeech && len(state.sysBuf) >= audio.MinChunkSecs*audio.FrameBytes {
					r.emitChunk(ctx, state.sysBuf, state.micBuf, state.chunkStartTime, chunkCh)
				}
				slog.InfoContext(ctx, "audio capture ended")
				return
			}
			r.processFrame(ctx, &state, frame, chunkCh)
		}
	}
}

func (r *Recorder) processFrame(
	ctx context.Context,
	state *captureState,
	frame audio.Frame,
	chunkCh chan<- AudioChunk,
) {
	if err := r.lk.Heartbeat(); err != nil {
		slog.ErrorContext(ctx, "heartbeat failed",
			"err", err,
		)
	}

	state.ingest(frame.Sys, frame.Mic)

	sysRMS := audio.ComputeRMS(frame.Sys)
	micRMS := audio.ComputeRMS(frame.Mic)
	secondHasSpeech := sysRMS >= audio.SpeechRMSThreshold || micRMS >= audio.SpeechRMSThreshold

	if secondHasSpeech {
		if !state.wasSpeech {
			var src []string
			if sysRMS >= audio.SpeechRMSThreshold {
				src = append(src, fmt.Sprintf("sys=%.4f", sysRMS))
			}
			if micRMS >= audio.SpeechRMSThreshold {
				src = append(src, fmt.Sprintf("mic=%.4f", micRMS))
			}
			slog.InfoContext(ctx, "speech detected",
				"sources", strings.Join(src, ", "),
			)
			state.wasSpeech = true
		}
		state.recordSpeech()
		r.silenceMonitor.Reset()
	} else {
		if state.wasSpeech && state.consecutiveSilentSecs == 1 {
			slog.InfoContext(ctx, "silence")
			state.wasSpeech = false
		}
		state.recordSilence()
		if r.silenceMonitor.Tick(state.consecutiveSilentSecs) {
			mins := state.consecutiveSilentSecs / 60
			e := transcript.Event{
				Time: time.Now(),
				Type: transcript.Idle,
				Text: fmt.Sprintf("%d min", mins),
			}
			r.appendEvent(ctx, e)
		}
		r.segmenter.OnSilence(state.consecutiveSilentSecs)
	}

	switch state.action() {
	case actionDiscard:
		state.reset()
	case actionEmit:
		sys, mic := state.trimmedPCM()
		r.emitChunk(ctx, sys, mic, state.chunkStartTime, chunkCh)
		state.reset()
	}
}

func (r *Recorder) emitChunk(
	ctx context.Context,
	sysPCM, micPCM []byte,
	startTime time.Time,
	chunkCh chan<- AudioChunk,
) {
	r.chunkNum++
	endTime := time.Now()
	duration := len(sysPCM) / audio.FrameBytes
	sysRMS := audio.ComputeRMS(sysPCM)
	micRMS := audio.ComputeRMS(micPCM)

	if sysRMS < audio.ChunkRMSThreshold && micRMS < audio.ChunkRMSThreshold {
		slog.InfoContext(ctx, "chunk skipped",
			"chunkNum", r.chunkNum,
			"durationSec", duration,
			"sysRms", sysRMS,
			"micRms", micRMS,
			"reason", "below threshold",
		)
		return
	}

	slog.InfoContext(ctx, "chunk emitted",
		"chunkNum", r.chunkNum,
		"durationSec", duration,
		"sysRms", sysRMS,
		"micRms", micRMS,
	)

	chunkCh <- AudioChunk{
		SysWAV:    audio.MakeWAV(sysPCM, audio.SampleRate),
		MicWAV:    audio.MakeWAV(micPCM, audio.SampleRate),
		StartTime: startTime,
		EndTime:   endTime,
	}
}
