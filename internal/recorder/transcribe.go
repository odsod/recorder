package recorder

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"time"

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
	result := r.chunkTranscriber.Transcribe(ctx, chunk)
	if result.SystemTranscribeErr != nil {
		slog.ErrorContext(ctx, "transcribe sys failed",
			"err", result.SystemTranscribeErr,
		)
	}
	if result.MicTranscribeErr != nil {
		slog.ErrorContext(ctx, "transcribe mic failed",
			"err", result.MicTranscribeErr,
		)
	}

	r.flushSignalEvents(ctx, chunk.StartTime, chunk.EndTime)

	if result.SystemEmitErr != nil {
		slog.ErrorContext(ctx, "emit sys speech failed", "err", result.SystemEmitErr)
	}
	r.appendSpeechEvents(ctx, result.SystemEvents)
	if result.MicEmitErr != nil {
		slog.ErrorContext(ctx, "emit mic speech failed", "err", result.MicEmitErr)
	}
	r.appendSpeechEvents(ctx, result.MicEvents)

	if result.NoSpeech() {
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
