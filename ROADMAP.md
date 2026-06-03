# Recorder Roadmap

## Speaker Attribution Improvements

### Goals

- Keep long Whisper chunks for transcription quality
- Attribute speech at Whisper segment level, not audio chunk level
- Preserve useful ambiguity instead of discarding speaker evidence
- Keep complexity isolated behind testable interfaces

### Invariants

- Do not flush audio chunks on speaker changes
- Do not rely on word timestamps
  - `whisper.cpp --vad` preserves segment offsets
  - word offsets were inconsistent in validation
- Use Whisper `verbose_json` segment timestamps
- Treat speaker percentages as active-indicator coverage, not word ownership

### Validated Whisper Capability

- Server: `whisper.cpp` on `http://odsod-desktop:8178`
- Endpoint: `/v1/audio/transcriptions`
- Request format:

```text
response_format=verbose_json
```

- Response includes:
  - `text`
  - `segments[].start`
  - `segments[].end`
  - `segments[].text`
  - `segments[].words`
- Use `segments[]`
- Ignore `words[]` for attribution

### Transcript Target

```md
[11:28:14] 🔊 **sys** [Andreas Bäckevik 85%] clear segment...
[11:28:19] 🔊 **sys** [Andreas Bäckevik 55% / Sofia Thorén 35%] mixed segment...
[11:28:22] 🔊 **sys** [Andreas Bäckevik 35% / Sofia Thorén 28% / Oscar Söderlund 9%] group segment...
```

### Attribution Rules

- For each Whisper segment:
  - `absoluteStart = chunk.StartTime + segment.StartSec`
  - `absoluteEnd = chunk.StartTime + segment.EndSec`
  - compute active speaker coverage over that interval
- Include every active speaker above threshold:
  - `minCandidatePct = 5-10%`
  - `minCandidateDuration = 250ms`
- Sort speakers by coverage descending
- Render all included speakers in the prefix
- Percentages are:

```text
speaker active duration / segment duration
```

- Percentages may sum over 100% when Meet shows overlapping active indicators

### Implementation Steps

1. **Whisper Client**

- Send `response_format=verbose_json`
- Extend response model:

```go
type Segment struct {
    StartSec float64
    EndSec   float64
    Text     string
}

type TranscribeResponse struct {
    Text     string
    Segments []Segment
}
```

- Preserve `Text` fallback when `Segments` is empty
- Add protocol tests for request format and segment parsing

2. **Speaker Tracker**

- Add isolated debounced tracker
- Poll CDP at higher frequency, e.g. `250ms`
- Tracker input:

```go
[]signals.ParticipantState
```

- Tracker output:
  - stable speaker start/stop transitions
- Rules:
  - accept repeated short Meet indicator flashes
  - ignore isolated blips
  - hold through short UI dropouts
  - emit stop after grace period

3. **Timeline Coverage API**

- Add ranked speaker coverage lookup:

```go
type SpeakerCandidate struct {
    Name        string
    CoverageSec float64
    CoveragePct float64
}

type SpeakerAttribution struct {
    Candidates []SpeakerCandidate
}
```

- API should return all candidates above threshold, sorted descending
- Keep percentage math inside `internal/timeline`
- Add tests for:
  - overlapping speakers
  - low-threshold inclusion
  - min-duration filtering
  - no active speakers
  - percentages summing over 100%

4. **Transcription Worker**

- Emit one transcript event per Whisper segment
- Event time is segment absolute start
- Render speaker prefix with all candidates and percentages
- Keep long audio chunking unchanged
- Keep existing fallback path for text-only responses

5. **Cleanup**

- First implementation: cleanup each emitted segment independently
- Preserve existing chunk-level behavior as fallback for text-only responses
- Revisit segment-preserving cleanup only if per-segment cleanup degrades quality

6. **Mic/System Dedup**

- Target behavior:
  - compare mic segments against nearby system segments
  - dedup per segment instead of whole chunk
- First implementation may keep chunk-level dedup if needed
- Avoid blocking segment-level system attribution on dedup complexity

7. **Diagnostics**

- Log debounced speaker transitions
- Log segment attribution decisions:
  - chunk number
  - segment start/end
  - candidates
  - thresholds applied
- Do not log every raw 250ms sample by default

### Verification

```bash
mise run test
mise run build
```

### Live Validation

- Run recorder in a Google Meet
- Inspect:
  - `~/.local/share/recorder/recorder.jsonl`
  - configured transcript file for the current day
- Confirm:
  - long audio chunks produce multiple transcript events
  - back-and-forth segments show multiple active speakers
  - speaker percentages are visible
  - unattributed lines only happen when no speaker crosses threshold

## Follow-Up Modularization

### Goals

- Raise confidence in the new segment-level attribution path
- Keep recorder orchestration small
- Move policy into focused, testable units
- Avoid leaking attribution complexity into capture, transcript writing, or segmenting

### Target Package Structure

```text
internal/
├── protocol/
│   ├── whisper/              # wire client; verbose_json structs
│   ├── llm/
│   ├── cdp/
│   └── parec/
├── signals/
│   ├── speaker.go            # collector ticker wiring
│   ├── speaker_collector.go  # SpeakerCollector.PollOnce
│   ├── speaker_tracker.go    # debounce policy
│   └── silence.go
├── timeline/
│   ├── speaker.go            # speaker intervals + coverage lookup
│   └── meeting.go
├── speech/
│   ├── segment.go            # canonical SpeechSegment type + normalizer
│   ├── attribution.go        # speaker percentage formatting
│   ├── dedup.go              # segment-level mic/sys dedup
│   └── emitter.go            # segments -> transcript.Event
├── recorder/
│   ├── recorder.go           # lifecycle, goroutines
│   ├── capture.go            # audio chunk production
│   ├── transcribe.go         # chunk orchestration only
│   ├── services.go
│   ├── types.go
│   └── writer.go
├── transcript/
├── segment/
├── summarize/
└── transcribe/               # LLM cleanup + text overlap helpers
```

### Package Responsibilities

- `protocol/whisper`
  - HTTP multipart wire protocol
  - OpenAI-compatible response parsing
  - `verbose_json` segment structs
- `timeline`
  - time-indexed meeting/speaker state
  - active speaker interval storage
  - coverage math
  - no transcript rendering
- `signals`
  - CDP polling
  - participant and meeting signal collection
  - speaker debounce state
  - ticker loop and `PollOnce`
- `speech`
  - turn Whisper responses into transcript speech events
  - normalize Whisper segments into absolute-time speech segments
  - apply cleanup
  - apply segment-level dedup
  - format speaker attribution percentages
- `recorder`
  - lifecycle
  - goroutines
  - capture loop
  - chunk-level transcription orchestration
  - transcript append and segmenter feed
- `transcript`
  - event format
  - event parsing
- `segment`
  - transcript segmentation
  - summarization input formatting

### `internal/speech` API Sketch

```go
package speech

type Segment struct {
    Start time.Time
    End   time.Time
    Text  string
}

func FromWhisper(resp whisper.TranscribeResponse, chunkStart, chunkEnd time.Time) []Segment
```

```go
type SpeakerLookup interface {
    Coverage(start, end time.Time, opts timeline.SpeakerLookupOptions) timeline.SpeakerAttribution
}

type Cleaner interface {
    Cleanup(ctx context.Context, text string) (string, error)
}

type Deduper interface {
    IsDuplicate(segment Segment, refs []transcript.Event) bool
}

type Emitter struct {
    Cleaner       Cleaner
    SpeakerLookup SpeakerLookup
    Deduper       Deduper
}

func (e *Emitter) Emit(ctx context.Context, source string, segments []Segment, refs []transcript.Event) ([]transcript.Event, error)
```

```go
func FormatAttribution(a timeline.SpeakerAttribution) string
```

### Recorder Shape After Refactor

```go
sysSegments := speech.FromWhisper(sysResp, chunk.StartTime, chunk.EndTime)
sysEvents := r.speechEmitter.Emit(ctx, "sys", sysSegments, nil)

micSegments := speech.FromWhisper(micResp, chunk.StartTime, chunk.EndTime)
micEvents := r.speechEmitter.Emit(ctx, "mic", micSegments, sysEvents)
```

### Package Boundaries To Avoid

- Do not create package-per-helper boundaries:
  - `internal/attribution`
  - `internal/dedup`
  - `internal/whispersegments`
- Keep related speech-event policy together in `internal/speech`
- Keep rendering out of `timeline`
- Keep lifecycle/goroutine concerns out of `speech`

### Recommended Order

1. **Speech Segment Emitter**

- Extract from recorder transcription flow
- Input:
  - source (`sys` / `mic`)
  - audio chunk timing
  - Whisper segments
  - nearby opposite-channel transcript events
- Output:
  - `[]transcript.Event`
- Owns:
  - segment cleanup
  - speaker attribution lookup
  - transcript event construction
  - segment-level dedup decision
- Rationale:
  - highest-risk new behavior
  - currently coupled to `Recorder`

2. **Speaker Collector `PollOnce`**

- Keep ticker loop thin
- Add testable collector unit:

```go
type SpeakerCollector struct {
    Detector SpeakerPoller
    Tracker  *SpeakerTracker
    Timeline SpeakerTimelineWriter
    People   ParticipantWriter
    Meetings MeetingWriter
}

func (c *SpeakerCollector) PollOnce(ctx context.Context, now time.Time) error
```

- Test:
  - participant updates
  - meeting reset
  - tracker reset on meeting change
  - active/inactive timeline writes
  - CDP error handling

3. **Whisper Segment Normalizer**

- Move segment normalization out of `internal/recorder/transcribe.go`
- Input:
  - `whisper.TranscribeResponse`
  - chunk start/end
- Output:
  - `[]SpeechSegment`
- Test:
  - verbose segments
  - empty segments
  - text fallback
  - zero/invalid segment duration fallback

4. **Speaker Attribution Formatter**

- Extract prefix rendering policy
- Input:
  - `timeline.SpeakerAttribution`
- Output:
  - speaker prefix string
- Test:
  - rounding
  - ordering
  - empty attribution
  - multiple speakers

5. **Segment Deduper**

- Extract mic/system dedup policy
- Interface:

```go
type SegmentDeduper interface {
    IsDuplicate(segment SpeechSegment, refs []transcript.Event) bool
}
```

- Test:
  - nearby duplicate
  - distant non-duplicate
  - threshold behavior
  - empty references

6. **Transcription Orchestrator**

- Longer-term extraction:

```go
type ChunkTranscriber struct {
    Transcriber Transcriber
    Emitter     SegmentEmitter
}

func (t *ChunkTranscriber) Transcribe(ctx context.Context, chunk AudioChunk) ([]transcript.Event, error)
```

- Recorder stays responsible for:
  - appending events
  - feeding segmenter
  - lifecycle and shutdown

### Interfaces To Keep Narrow

```go
type SpeakerLookup interface {
    Coverage(start, end time.Time, opts timeline.SpeakerLookupOptions) timeline.SpeakerAttribution
}

type SpeakerTimelineWriter interface {
    SetSpeakerActive(ts time.Time, name string, active bool)
    Append(ts time.Time, name string)
}
```

### Risk Reduction

- Add direct tests for:
  - one audio chunk producing multiple transcript events
  - percentage attribution attached to each emitted event
  - mixed speaker segment preserving all active candidates
  - collector reset on meeting/tab change
  - no stale speaker carry-over between meetings
