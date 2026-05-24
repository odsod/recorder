package recorder

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/odsod/recorder/internal/audio/chunk"
	"github.com/odsod/recorder/internal/audio/frame"
	"github.com/odsod/recorder/internal/audio/gate"
	"github.com/odsod/recorder/internal/audio/pcm"
	"github.com/odsod/recorder/internal/audio/wav"
	"github.com/odsod/recorder/internal/transcript"
)

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

	accum := chunk.New(chunk.DefaultConfig())
	audioGate := gate.Default()
	var wasSpeech bool

	slog.InfoContext(ctx, "listening")

	for {
		select {
		case <-ctx.Done():
			if out, ok := accum.Flush(); ok {
				r.emitChunk(ctx, out.SysPCM, out.MicPCM, out.StartTime, audioGate, chunkCh)
			}
			return
		case frame, ok := <-frames:
			if !ok {
				if out, ok := accum.Flush(); ok {
					r.emitChunk(ctx, out.SysPCM, out.MicPCM, out.StartTime, audioGate, chunkCh)
				}
				slog.InfoContext(ctx, "audio capture ended")
				return
			}
			wasSpeech = r.processFrame(ctx, accum, audioGate, frame, wasSpeech, chunkCh)
		}
	}
}

func (r *Recorder) processFrame(
	ctx context.Context,
	accum *chunk.Accumulator,
	audioGate gate.Config,
	frame frame.Dual,
	wasSpeech bool,
	chunkCh chan<- AudioChunk,
) bool {
	if err := r.lk.Heartbeat(); err != nil {
		slog.ErrorContext(ctx, "heartbeat failed",
			"err", err,
		)
	}

	speech := audioGate.FrameHasSpeech(frame.Sys, frame.Mic)
	out := accum.Ingest(frame.Sys, frame.Mic, time.Now(), speech.Passes)

	if speech.Passes {
		if !wasSpeech {
			var src []string
			if speech.SysRMS >= audioGate.FrameThreshold {
				src = append(src, fmt.Sprintf("sys=%.4f", speech.SysRMS))
			}
			if speech.MicRMS >= audioGate.FrameThreshold {
				src = append(src, fmt.Sprintf("mic=%.4f", speech.MicRMS))
			}
			slog.InfoContext(ctx, "speech detected",
				"sources", strings.Join(src, ", "),
			)
			wasSpeech = true
		}
		r.silenceMonitor.Reset()
	} else {
		if wasSpeech && out.ConsecutiveSilentSecs == 1 {
			slog.InfoContext(ctx, "silence")
			wasSpeech = false
		}
		if r.silenceMonitor.Tick(out.ConsecutiveSilentSecs) {
			mins := out.ConsecutiveSilentSecs / 60
			e := transcript.Event{
				Time: time.Now(),
				Type: transcript.Idle,
				Text: fmt.Sprintf("%d min", mins),
			}
			r.appendEvent(ctx, e)
		}
		r.segmenter.OnSilence(out.ConsecutiveSilentSecs)
	}

	switch out.Action {
	case chunk.ActionDiscard:
		// buffer cleared by accumulator
	case chunk.ActionEmit:
		r.emitChunk(ctx, out.SysPCM, out.MicPCM, out.StartTime, audioGate, chunkCh)
	}
	return wasSpeech
}

func (r *Recorder) emitChunk(
	ctx context.Context,
	sysPCM, micPCM []byte,
	startTime time.Time,
	audioGate gate.Config,
	chunkCh chan<- AudioChunk,
) {
	r.chunkNum++
	endTime := time.Now()
	duration := pcm.FrameCount(sysPCM, pcm.FrameBytes)
	filter := audioGate.ChunkPasses(sysPCM, micPCM)

	if !filter.Passes {
		slog.InfoContext(ctx, "chunk skipped",
			"chunkNum", r.chunkNum,
			"durationSec", duration,
			"sysRms", filter.SysRMS,
			"micRms", filter.MicRMS,
			"reason", "below threshold",
		)
		return
	}

	slog.InfoContext(ctx, "chunk emitted",
		"chunkNum", r.chunkNum,
		"durationSec", duration,
		"sysRms", filter.SysRMS,
		"micRms", filter.MicRMS,
	)

	chunkCh <- AudioChunk{
		SysWAV:    wav.MakeWAV(sysPCM, pcm.SampleRate),
		MicWAV:    wav.MakeWAV(micPCM, pcm.SampleRate),
		StartTime: startTime,
		EndTime:   endTime,
	}
}
