package recorder

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/odsod/recorder/internal/protocol/whisper"
	"github.com/odsod/recorder/internal/speech"
	"github.com/odsod/recorder/internal/transcript"
)

func TestChunkTranscriber_MultipleSystemSegmentsReturnMultipleEvents(t *testing.T) {
	start := chunkStart()
	transcriber := &fakeChunkTranscriber{
		responses: map[string]whisper.TranscribeResponse{
			"sys.wav": {Segments: []whisper.Segment{
				{StartSec: 0, EndSec: 1, Text: "first"},
				{StartSec: 2, EndSec: 3, Text: "second"},
			}},
		},
	}
	emitter := &fakeChunkSpeechEmitter{}
	chunkTranscriber := ChunkTranscriber{Transcriber: transcriber, SpeechEmitter: emitter}

	got := chunkTranscriber.Transcribe(context.Background(), testAudioChunk(start))

	if len(got.SystemEvents) != 2 {
		t.Fatalf("got %d system events, want 2", len(got.SystemEvents))
	}
	if got.SystemEvents[0].Text != "first" || got.SystemEvents[1].Text != "second" {
		t.Fatalf("system events = %+v", got.SystemEvents)
	}
}

func TestChunkTranscriber_MicReceivesCurrentSystemEventsAsDedupRefs(t *testing.T) {
	start := chunkStart()
	transcriber := &fakeChunkTranscriber{
		responses: map[string]whisper.TranscribeResponse{
			"sys.wav": {Text: "system text"},
			"mic.wav": {Text: "mic text"},
		},
	}
	emitter := &fakeChunkSpeechEmitter{}
	chunkTranscriber := ChunkTranscriber{Transcriber: transcriber, SpeechEmitter: emitter}

	got := chunkTranscriber.Transcribe(context.Background(), testAudioChunk(start))

	if len(got.SystemEvents) != 1 {
		t.Fatalf("system events = %+v", got.SystemEvents)
	}
	micCall := emitter.calls[1]
	if micCall.source != "mic" {
		t.Fatalf("second emit source = %q, want mic", micCall.source)
	}
	if !reflect.DeepEqual(micCall.refs, got.SystemEvents) {
		t.Fatalf("mic refs = %+v, want %+v", micCall.refs, got.SystemEvents)
	}
}

func TestChunkTranscriber_MicReceivesPriorSystemTextFallback(t *testing.T) {
	start := chunkStart()
	transcriber := &fakeChunkTranscriber{
		responses: map[string]whisper.TranscribeResponse{
			"sys.wav": {Text: "previous system"},
		},
	}
	emitter := &fakeChunkSpeechEmitter{}
	chunkTranscriber := ChunkTranscriber{Transcriber: transcriber, SpeechEmitter: emitter}

	chunkTranscriber.Transcribe(context.Background(), testAudioChunk(start))

	nextStart := start.Add(time.Minute)
	transcriber.responses = map[string]whisper.TranscribeResponse{
		"mic.wav": {Text: "mic text"},
	}
	chunkTranscriber.Transcribe(context.Background(), testAudioChunk(nextStart))

	micCall := emitter.calls[3]
	wantRefs := []transcript.Event{{Time: nextStart, Text: "previous system"}}
	if !reflect.DeepEqual(micCall.refs, wantRefs) {
		t.Fatalf("mic refs = %+v, want %+v", micCall.refs, wantRefs)
	}
}

func TestChunkTranscriber_PriorSystemTextUpdatesOnlyWhenSystemEventsEmit(t *testing.T) {
	start := chunkStart()
	transcriber := &fakeChunkTranscriber{
		responses: map[string]whisper.TranscribeResponse{
			"sys.wav": {Text: "previous system"},
		},
	}
	emitter := &fakeChunkSpeechEmitter{}
	chunkTranscriber := ChunkTranscriber{Transcriber: transcriber, SpeechEmitter: emitter}

	chunkTranscriber.Transcribe(context.Background(), testAudioChunk(start))

	nextStart := start.Add(time.Minute)
	transcriber.responses = map[string]whisper.TranscribeResponse{
		"sys.wav": {Text: "current system"},
		"mic.wav": {Text: "mic text"},
	}
	emitter.suppressSource = map[string]bool{"sys": true}
	chunkTranscriber.Transcribe(context.Background(), testAudioChunk(nextStart))

	micCall := emitter.calls[3]
	wantRefs := []transcript.Event{{Time: nextStart, Text: "previous system"}}
	if !reflect.DeepEqual(micCall.refs, wantRefs) {
		t.Fatalf("mic refs = %+v, want %+v", micCall.refs, wantRefs)
	}
}

func TestChunkTranscriber_SystemTranscribeErrorDoesNotPreventMicTranscription(t *testing.T) {
	start := chunkStart()
	sysErr := errors.New("sys failed")
	transcriber := &fakeChunkTranscriber{
		responses: map[string]whisper.TranscribeResponse{
			"mic.wav": {Text: "mic text"},
		},
		errors: map[string]error{"sys.wav": sysErr},
	}
	emitter := &fakeChunkSpeechEmitter{}
	chunkTranscriber := ChunkTranscriber{Transcriber: transcriber, SpeechEmitter: emitter}

	got := chunkTranscriber.Transcribe(context.Background(), testAudioChunk(start))

	if !reflect.DeepEqual(transcriber.calls, []string{"sys.wav", "mic.wav"}) {
		t.Fatalf("transcriber calls = %+v", transcriber.calls)
	}
	if len(got.MicEvents) != 1 || got.MicEvents[0].Text != "mic text" {
		t.Fatalf("mic events = %+v", got.MicEvents)
	}
	if !errors.Is(got.Err, sysErr) {
		t.Fatalf("err = %v, want %v", got.Err, sysErr)
	}
}

func TestChunkTranscriber_EmitterErrorPreservesReturnedEvents(t *testing.T) {
	start := chunkStart()
	emitErr := errors.New("cleanup failed")
	transcriber := &fakeChunkTranscriber{
		responses: map[string]whisper.TranscribeResponse{
			"sys.wav": {Text: "fallback text"},
		},
	}
	emitter := &fakeChunkSpeechEmitter{errors: map[string]error{"sys": emitErr}}
	chunkTranscriber := ChunkTranscriber{Transcriber: transcriber, SpeechEmitter: emitter}

	got := chunkTranscriber.Transcribe(context.Background(), testAudioChunk(start))

	if len(got.SystemEvents) != 1 || got.SystemEvents[0].Text != "fallback text" {
		t.Fatalf("system events = %+v", got.SystemEvents)
	}
	if !errors.Is(got.SystemEmitErr, emitErr) || !errors.Is(got.Err, emitErr) {
		t.Fatalf("errors = system:%v joined:%v, want %v", got.SystemEmitErr, got.Err, emitErr)
	}
}

func TestChunkTranscriber_EmptyResponsesReportNoSpeech(t *testing.T) {
	start := chunkStart()
	transcriber := &fakeChunkTranscriber{}
	emitter := &fakeChunkSpeechEmitter{}
	chunkTranscriber := ChunkTranscriber{Transcriber: transcriber, SpeechEmitter: emitter}

	got := chunkTranscriber.Transcribe(context.Background(), testAudioChunk(start))

	if !got.NoSpeech() {
		t.Fatalf("NoSpeech() = false, want true")
	}
	if len(got.SystemEvents) != 0 || len(got.MicEvents) != 0 {
		t.Fatalf("events = sys:%+v mic:%+v", got.SystemEvents, got.MicEvents)
	}
}

type fakeChunkTranscriber struct {
	responses map[string]whisper.TranscribeResponse
	errors    map[string]error
	calls     []string
}

func (t *fakeChunkTranscriber) Transcribe(
	_ context.Context,
	req whisper.TranscribeRequest,
) (whisper.TranscribeResponse, error) {
	t.calls = append(t.calls, req.Filename)
	return t.responses[req.Filename], t.errors[req.Filename]
}

type fakeChunkSpeechEmitter struct {
	errors         map[string]error
	suppressSource map[string]bool
	calls          []fakeChunkEmitCall
}

func (e *fakeChunkSpeechEmitter) Emit(
	_ context.Context,
	source string,
	segments []speech.Segment,
	refs []transcript.Event,
) ([]transcript.Event, error) {
	e.calls = append(e.calls, fakeChunkEmitCall{
		source:   source,
		segments: append([]speech.Segment(nil), segments...),
		refs:     append([]transcript.Event(nil), refs...),
	})
	if e.suppressSource[source] {
		return nil, e.errors[source]
	}

	events := make([]transcript.Event, 0, len(segments))
	for _, segment := range segments {
		events = append(events, transcript.Event{
			Time:   segment.Start,
			Type:   transcript.Speech,
			Source: source,
			Text:   segment.Text,
		})
	}
	return events, e.errors[source]
}

type fakeChunkEmitCall struct {
	source   string
	segments []speech.Segment
	refs     []transcript.Event
}

func chunkStart() time.Time {
	return time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
}

func testAudioChunk(start time.Time) AudioChunk {
	return AudioChunk{
		SysWAV:    []byte("sys"),
		MicWAV:    []byte("mic"),
		StartTime: start,
		EndTime:   start.Add(10 * time.Second),
	}
}
