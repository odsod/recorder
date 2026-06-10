package recorder

import (
	"context"
	"errors"

	"github.com/odsod/recorder/internal/protocol/whisper"
	"github.com/odsod/recorder/internal/speech"
	"github.com/odsod/recorder/internal/transcript"
)

// SpeechEmitter converts speech segments into transcript events.
type SpeechEmitter interface {
	Emit(
		ctx context.Context,
		source string,
		segments []speech.Segment,
		refs []transcript.Event,
	) ([]transcript.Event, error)
}

// ChunkTranscriber orchestrates transcription and speech event emission for one chunk.
type ChunkTranscriber struct {
	Transcriber   Transcriber
	SpeechEmitter SpeechEmitter

	SystemRefs speech.SystemReferenceTracker
}

// ChunkTranscription contains the speech events and errors produced for one chunk.
type ChunkTranscription struct {
	SystemEvents []transcript.Event
	MicEvents    []transcript.Event

	SystemSpeechDetected bool
	MicSpeechDetected    bool

	SystemTranscribeErr error
	MicTranscribeErr    error
	SystemEmitErr       error
	MicEmitErr          error
	Err                 error
}

// NoSpeech reports whether neither channel yielded speech segments.
func (t ChunkTranscription) NoSpeech() bool {
	return !t.SystemSpeechDetected && !t.MicSpeechDetected
}

// Transcribe transcribes both audio channels and emits speech events without writing them.
func (t *ChunkTranscriber) Transcribe(ctx context.Context, chunk AudioChunk) ChunkTranscription {
	var out ChunkTranscription

	sysResp, err := t.Transcriber.Transcribe(ctx, whisper.TranscribeRequest{
		WAVData:  chunk.SysWAV,
		Filename: "sys.wav",
	})
	out.SystemTranscribeErr = err

	micResp, err := t.Transcriber.Transcribe(ctx, whisper.TranscribeRequest{
		WAVData:  chunk.MicWAV,
		Filename: "mic.wav",
	})
	out.MicTranscribeErr = err

	sysSegments := speech.FromWhisper(sysResp, chunk.StartTime, chunk.EndTime)
	micSegments := speech.FromWhisper(micResp, chunk.StartTime, chunk.EndTime)
	out.SystemSpeechDetected = len(sysSegments) > 0
	out.MicSpeechDetected = len(micSegments) > 0

	out.SystemEvents, out.SystemEmitErr = t.SpeechEmitter.Emit(ctx, "sys", sysSegments, nil)
	micDedupEvents := t.SystemRefs.MicRefs(chunk.StartTime, out.SystemEvents)
	t.SystemRefs.Update(out.SystemEvents)
	out.MicEvents, out.MicEmitErr = t.SpeechEmitter.Emit(ctx, "mic", micSegments, micDedupEvents)

	out.Err = errors.Join(
		out.SystemTranscribeErr,
		out.MicTranscribeErr,
		out.SystemEmitErr,
		out.MicEmitErr,
	)
	return out
}
