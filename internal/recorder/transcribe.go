package recorder

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/odsod/recorder/internal/protocol/whisper"
	"github.com/odsod/recorder/internal/transcribe"
	"github.com/odsod/recorder/internal/transcript"
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

	sysText := sysResp.Text
	micText := micResp.Text

	r.flushSignalEvents(ctx, chunk.StartTime, chunk.EndTime)

	speakers := r.speakerTimeline.SpeakersIn(chunk.StartTime, chunk.EndTime)
	speaker := ""
	if len(speakers) > 0 {
		speaker = speakers[0]
	}

	switch {
	case sysText != "":
		cleaned, err := r.svc.Cleaner.Cleanup(ctx, sysText)
		if err != nil {
			slog.ErrorContext(ctx, "cleanup sys failed",
				"err", err,
			)
		}
		if cleaned == "" {
			cleaned = sysText
		}
		if cleaned != "" {
			e := transcript.Event{
				Time:    chunk.StartTime,
				Type:    transcript.Speech,
				Source:  "sys",
				Text:    cleaned,
				Speaker: speaker,
			}
			r.appendEvent(ctx, e)
			r.lastSystemText = cleaned
			r.segmenter.OnSpeech(e)

			if micText != "" && !transcribe.TextsOverlap(cleaned, micText, r.cfg.Dedup.Threshold) {
				micCleaned, err := r.svc.Cleaner.Cleanup(ctx, micText)
				if err != nil {
					slog.ErrorContext(ctx, "cleanup mic failed",
						"err", err,
					)
				}
				if micCleaned == "" {
					micCleaned = micText
				}
				if micCleaned != "" {
					me := transcript.Event{
						Time:    chunk.StartTime,
						Type:    transcript.Speech,
						Source:  "mic",
						Text:    micCleaned,
						Speaker: speaker,
					}
					r.appendEvent(ctx, me)
					r.segmenter.OnSpeech(me)
				}
			}
		}
	case micText != "":
		if r.lastSystemText != "" && transcribe.TextsOverlap(r.lastSystemText, micText, r.cfg.Dedup.Threshold) {
			slog.InfoContext(ctx, "mic deduped",
				"text", truncate(micText, 60),
			)
		} else {
			cleaned, err := r.svc.Cleaner.Cleanup(ctx, micText)
			if err != nil {
				slog.ErrorContext(ctx, "cleanup mic failed",
					"err", err,
				)
			}
			if cleaned == "" {
				cleaned = micText
			}
			if cleaned != "" {
				e := transcript.Event{
					Time:    chunk.StartTime,
					Type:    transcript.Speech,
					Source:  "mic",
					Text:    cleaned,
					Speaker: speaker,
				}
				r.appendEvent(ctx, e)
				r.segmenter.OnSpeech(e)
			}
		}
	default:
		slog.InfoContext(ctx, "no speech detected")
	}
	slog.InfoContext(ctx, "listening")
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
