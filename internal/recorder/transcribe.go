package recorder

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/odsod/recorder/internal/protocol/whisper"
	"github.com/odsod/recorder/internal/speech"
	"github.com/odsod/recorder/internal/transcript"
)

const (
	minSpeakerCandidatePct      = 0.05
	minSpeakerCandidateDuration = 250 * time.Millisecond
)

func (r *Recorder) transcriptionWorker(ctx context.Context, chunkCh <-chan AudioChunk) {
	for chunk := range chunkCh {
		slog.InfoContext(ctx, "transcribing")
		t0 := time.Now()
		r.transcribeChunk(ctx, chunk)
		slog.InfoContext(ctx, "transcribed",
			"durationSec", time.Since(t0).Seconds(),
		)
	}
}

func (r *Recorder) transcribeChunk(ctx context.Context, chunk AudioChunk) {
	sysResp, err := r.svc.Transcriber.Transcribe(ctx, whisper.TranscribeRequest{
		WAVData: chunk.SysWAV, Filename: "sys.wav",
	})
	if err != nil {
		slog.ErrorContext(ctx, "transcribe sys failed",
			"err", err,
		)
	}
	micResp, err := r.svc.Transcriber.Transcribe(ctx, whisper.TranscribeRequest{
		WAVData: chunk.MicWAV, Filename: "mic.wav",
	})
	if err != nil {
		slog.ErrorContext(ctx, "transcribe mic failed",
			"err", err,
		)
	}

	r.flushSignalEvents(ctx, chunk.StartTime, chunk.EndTime)

	sysSegments := speech.FromWhisper(sysResp, chunk.StartTime, chunk.EndTime)
	micSegments := speech.FromWhisper(micResp, chunk.StartTime, chunk.EndTime)

	priorSystemText := r.lastSystemText
	sysEvents, err := r.speechEmitter.Emit(ctx, "sys", sysSegments, nil)
	if err != nil {
		slog.ErrorContext(ctx, "emit sys speech failed", "err", err)
	}
	r.appendSpeechEvents(ctx, sysEvents)
	if len(sysEvents) > 0 {
		r.lastSystemText = joinEventText(sysEvents)
	}

	micDedupEvents := sysEvents
	if len(micDedupEvents) == 0 && priorSystemText != "" {
		micDedupEvents = []transcript.Event{{Time: chunk.StartTime, Text: priorSystemText}}
	}
	micEvents, err := r.speechEmitter.Emit(ctx, "mic", micSegments, micDedupEvents)
	if err != nil {
		slog.ErrorContext(ctx, "emit mic speech failed", "err", err)
	}
	r.appendSpeechEvents(ctx, micEvents)

	if len(sysSegments) == 0 && len(micSegments) == 0 {
		slog.InfoContext(ctx, "no speech detected")
	}
	slog.InfoContext(ctx, "listening")
}

func (r *Recorder) currentParticipants() []string {
	all := r.participantSet.GetAll()
	if len(all) == 0 {
		return nil
	}
	return slices.Sorted(maps.Keys(all))
}

func joinEventText(events []transcript.Event) string {
	parts := make([]string, 0, len(events))
	for _, e := range events {
		if e.Text != "" {
			parts = append(parts, e.Text)
		}
	}
	return strings.Join(parts, " ")
}

func (r *Recorder) appendSpeechEvents(ctx context.Context, events []transcript.Event) {
	for _, e := range events {
		r.appendEvent(ctx, e)
		r.segmenter.OnSpeech(e)
	}
}

func (r *Recorder) flushSignalEvents(ctx context.Context, start, end time.Time) {
	r.lastFlushedTime = end

	if title, changedAt, ok := r.meetingState.Consume(); ok {
		e := transcript.Event{
			Time:  changedAt,
			Type:  transcript.Meeting,
			Title: title,
		}
		r.appendEvent(ctx, e)
		r.segmenter.OnEvent(e)
		if title != "" {
			r.segmenter.OnMeetingChange(title, changedAt)
		}
	}

	allParticipants := r.participantSet.GetAll()
	if len(allParticipants) > 0 && !setsEqual(allParticipants, r.lastPplSet) {
		r.lastPplSet = allParticipants
		e := transcript.Event{
			Time:   start,
			Type:   transcript.Participants,
			People: slices.Sorted(maps.Keys(allParticipants)),
		}
		r.appendEvent(ctx, e)
		r.segmenter.OnEvent(e)
	}
}
