package recorder

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/odsod/recorder/internal/transcribe"
	"github.com/odsod/recorder/internal/transcript"
)

func (r *Recorder) transcriptionWorker(ctx context.Context, chunkCh <-chan AudioChunk) {
	for chunk := range chunkCh {
		r.log("transcribing")
		t0 := time.Now()
		r.transcribeChunk(ctx, chunk)
		r.log(fmt.Sprintf("transcribed in %.1fs", time.Since(t0).Seconds()))
	}
}

func (r *Recorder) transcribeChunk(ctx context.Context, chunk AudioChunk) {
	sysText, err := transcribe.TranscribeChunk(ctx, chunk.SysWAV, "sys.wav", r.cfg.Whisper)
	if err != nil {
		r.log(fmt.Sprintf("transcribe sys: %v", err))
	}
	micText, err := transcribe.TranscribeChunk(ctx, chunk.MicWAV, "mic.wav", r.cfg.Whisper)
	if err != nil {
		r.log(fmt.Sprintf("transcribe mic: %v", err))
	}

	timestamp := chunk.StartTime.Format("15:04:05")
	r.flushSignalEvents(chunk.StartTime, chunk.EndTime)

	speakers := r.speakerTimeline.SpeakersIn(chunk.StartTime, chunk.EndTime)

	if sysText != "" {
		cleaned, err := transcribe.CleanupText(ctx, sysText, r.cfg.LLM)
		if err != nil {
			r.log(fmt.Sprintf("cleanup sys: %v", err))
		}
		if cleaned == "" {
			cleaned = sysText
		}
		if cleaned != "" {
			r.transcript.Append(timestamp, "\U0001f50a sys", cleaned, speakers)
			r.lastSystemText = cleaned
			r.log(transcript.FormatMessage("\U0001f50a sys", truncate(cleaned, 80), speakers))
			r.segmenter.OnSpeech(chunk.StartTime, "sys", cleaned)

			if micText != "" && !transcribe.TextsOverlap(cleaned, micText, r.cfg.Dedup.Threshold) {
				micCleaned, err := transcribe.CleanupText(ctx, micText, r.cfg.LLM)
				if err != nil {
					r.log(fmt.Sprintf("cleanup mic: %v", err))
				}
				if micCleaned == "" {
					micCleaned = micText
				}
				if micCleaned != "" {
					r.transcript.Append(timestamp, "\U0001f3a4 mic", micCleaned, speakers)
					r.log(transcript.FormatMessage("\U0001f3a4 mic", truncate(micCleaned, 80), nil))
					r.segmenter.OnSpeech(chunk.StartTime, "mic", micCleaned)
				}
			}
		}
	} else if micText != "" {
		if r.lastSystemText != "" && transcribe.TextsOverlap(r.lastSystemText, micText, r.cfg.Dedup.Threshold) {
			r.log(fmt.Sprintf("mic: (dedup) %s", truncate(micText, 60)))
		} else {
			cleaned, err := transcribe.CleanupText(ctx, micText, r.cfg.LLM)
			if err != nil {
				r.log(fmt.Sprintf("cleanup mic: %v", err))
			}
			if cleaned == "" {
				cleaned = micText
			}
			if cleaned != "" {
				r.transcript.Append(timestamp, "\U0001f3a4 mic", cleaned, speakers)
				r.log(transcript.FormatMessage("\U0001f3a4 mic", truncate(cleaned, 80), nil))
				r.segmenter.OnSpeech(chunk.StartTime, "mic", cleaned)
			}
		}
	} else {
		r.log("(no speech detected)")
	}
	r.log("listening")
}

func (r *Recorder) flushSignalEvents(start, end time.Time) {
	r.lastFlushedTime = end

	if title, changedAt, ok := r.meetingState.Consume(); ok {
		ts := changedAt.Format("15:04:05")
		if title != "" {
			msg := fmt.Sprintf("joined: %s", title)
			r.transcript.Append(ts, "\U0001fa9f mtg", msg, nil)
			r.log(transcript.FormatMessage("\U0001fa9f mtg", msg, nil))
			r.segmenter.OnSignal(changedAt, "mtg", "\U0001fa9f", msg)
			r.segmenter.OnMeetingChange(title, changedAt)
		} else {
			r.transcript.Append(ts, "\U0001fa9f mtg", "ended", nil)
			r.log(transcript.FormatMessage("\U0001fa9f mtg", "ended", nil))
			r.segmenter.OnSignal(changedAt, "mtg", "\U0001fa9f", "ended")
		}
	}

	allParticipants := r.participantSet.GetAll()
	if len(allParticipants) > 0 && !setsEqual(allParticipants, r.lastPplSet) {
		r.lastPplSet = allParticipants
		ts := start.Format("15:04:05")
		names := strings.Join(slices.Sorted(maps.Keys(allParticipants)), ", ")
		r.transcript.Append(ts, "\U0001f465 ppl", names, nil)
		r.log(transcript.FormatMessage("\U0001f465 ppl", names, nil))
		r.segmenter.OnSignal(start, "ppl", "\U0001f465", names)
	}
}
