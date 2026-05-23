# Recorder — Agent Instructions

Ambient meeting recorder daemon. Always-on capture → transcription → daily event log.

## Architecture

```
parec (mic) ─┐                          ┌─ server-side ─┐
             ├→ RMS gate → AudioChunk → │ Silero VAD    │→ LLM cleanup ─┐
parec (sys) ─┘   (with timestamps)      │ whisper decode│               │
                                         └───────────────┘               │
                                                                         ↓
CDP (Meet/Teams) → SpeakerTimeline ←─── Transcription Worker → Transcript
             └───→ MeetingState ────────────────┘                  ↓
                                                          IncrementalSegmenter
                                                                  ↓
                                                          Summarize → Segments
```

- **Daemon** — stdout is a structured log stream
- **No TUI** — plain timestamped log output, composable with tmux panes
- **Sole writer** — only the transcription goroutine writes to the transcript file
- **Notes** — `recorder note` accepts CLI args or stdin

### Goroutine Model

| Goroutine               | Role                                                              | Writes to transcript? |
| ----------------------- | ----------------------------------------------------------------- | --------------------- |
| Main (capture loop)     | parec read, RMS gating, silence counting, segmenter.OnSilence()   | No                    |
| Transcription worker    | Whisper → cleanup → speaker lookup → write → segmenter.OnSpeech() | Yes (sole writer)     |
| Speaker collector       | CDP polling → SpeakerTimeline + ParticipantSet + MeetingState     | No                    |
| Input loop              | Keyboard (p/s) via raw terminal                                   | No                    |
| Ephemeral (per segment) | Summarization + segment file                                      | Yes (seg marker)      |

Critical invariant: only the transcription goroutine calls `transcript.Append()` for speech and signal lines.

## Language & Build

- **Language**: Go
- **Module**: `github.com/odsod/recorder`
- **Binary**: single static binary → `~/.local/bin/recorder`
- **External deps**: none (stdlib only)
- **Config**: JSON (`$XDG_CONFIG_HOME/recorder/config.json`, default `~/.config/recorder/config.json`)
- **Build**: `go build -o ~/.local/bin/recorder ./cmd/recorder`

## Structure

```
recorder/
├── cmd/recorder/main.go          # Entrypoint, subcommand dispatch
├── internal/
│   ├── config/config.go          # Config structs + JSON loading (XDG)
│   ├── audio/                    # capture.go (parec), rms.go, wav.go
│   ├── timeline/                 # speaker.go, meeting.go (mutex-guarded state)
│   ├── cdp/                      # client.go (WebSocket), detector.go, platforms.go
│   ├── transcribe/               # whisper.go, cleanup.go, dedup.go
│   ├── transcript/               # daily.go (append-only), parse.go
│   ├── segment/                  # boundary.go (pure), segmenter.go (state machine)
│   ├── summarize/                # summarize.go, output.go, prompts.go (//go:embed)
│   ├── signals/                  # speaker.go, silence.go (collector goroutines)
│   ├── lock/lock.go              # JSON lockfile with heartbeat
│   └── note/note.go              # CLI arg / stdin → transcript append
└── mise.toml
```

## CLI

Single `recorder` binary, all functionality as subcommands:

```
recorder run                                  # start the daemon
recorder note                                 # interactive note (stdin)
recorder note "text"                          # note via CLI argument
recorder segment <transcript>                 # dry-run: show boundaries + summaries
recorder segment <transcript> --write         # write segment files + seg markers
recorder segment <transcript> --boundaries    # only show boundaries, no LLM calls
```

## Daemon Controls

Keybindings in the terminal (raw terminal input, no prefix):

| Key   | Action                                     |
| ----- | ------------------------------------------ |
| `C-c` | Quit (clean shutdown, final segmenter run) |
| `p`   | Pause/resume capture                       |
| `s`   | Insert `📍 pin` (segment boundary hint)    |

## Capture Pipeline

Per-channel chunking with server-side VAD:

1. `parec` captures mic + system audio continuously (separate channels)
2. Permissive RMS gate (threshold 0.002) — avoids sending dead silence over HTTP
3. Accumulate until 1s+ silence, min 10s / max 45s chunks
4. Each chunk carries wall-clock `(start_time, end_time)` for speaker correlation
5. Submit to whisper-server → **Silero VAD** decides speech/non-speech server-side
6. Whisper decodes only VAD-approved segments
7. LLM cleanup (filler, grammar, dedup, hallucination filtering)
8. **Speaker resolution** — query SpeakerTimeline for chunk's time window
9. **Audio dedup** — mic text suppressed if token overlap ≥ 60% with system text
10. Append clean timestamped speech to daily transcript with inline speaker attribution

## Transcript Format

Append-only daily event log at `<transcript.output_dir>/YYYY-MM-DD-recorder.md`.

Every line: `[HH:MM:SS] <emoji> **<tag>** <text>`

| Tag | Emoji | Source                                                           |
| --- | ----- | ---------------------------------------------------------------- |
| sys | 🔊    | System audio transcription (with inline `[Speaker]` attribution) |
| mic | 🎤    | Mic audio transcription (with inline `[Speaker]` attribution)    |
| mtg | 🪟    | CDP — meeting tab joined/ended                                   |
| ppl | 👥    | CDP — participant set changes                                    |
| idl | 💤    | Silence detector                                                 |
| nfo | 📝    | User — freeform annotation (`recorder note`)                     |
| pin | 📍    | User — segment boundary hint (`s` in recorder pane)              |
| seg | ✂️    | Segmenter — segment boundary emitted                             |
| rec | 🟢/🔴 | Recorder started/stopped                                         |

## Runtime Dependencies

| Service        | URL                                             | Purpose                 |
| -------------- | ----------------------------------------------- | ----------------------- |
| whisper-server | `http://localhost:8178/v1/audio/transcriptions` | ASR (local or remote)   |
| llm-server     | `http://localhost:8179/v1/chat/completions`     | Cleanup + summarization |

System: `pulseaudio-utils` (parec).
Chrome (Meet): `--remote-debugging-port=9224`.
Chrome (Teams): `--remote-debugging-port=9223`.

## Development

- **Install**: `mise run install`
- **Build only**: `go build ./cmd/recorder`
- **Test**: `mise run test`
- **Full CI**: `mise run build` (lint → test → tidy → diff)
- **Config**: `$XDG_CONFIG_HOME/recorder/config.json` (default `~/.config/recorder/config.json`)

### Config Sections

```json
{
  "whisper": { "url": "http://localhost:8178/v1/audio/transcriptions", "timeout_s": 60 },
  "llm": {
    "url": "http://localhost:8179/v1/chat/completions",
    "model": "default",
    "timeout_s": 180
  },
  "transcript": { "output_dir": "~/.local/share/recorder/transcripts" },
  "segments": { "output_dir": "~/.local/share/recorder/segments" },
  "dedup": { "threshold": 0.6 },
  "signals": {
    "silence_threshold_secs": 180,
    "cdp_ports": [9224, 9223]
  }
}
```

## Concurrency Design

- **Channel**: `chan AudioChunk` (buffered, cap 8) — capture → transcription
- **MeetingState**: mutex-guarded; speaker collector writes, transcription worker reads via `Consume()`
- **Timelines**: `sync.Mutex`-guarded structs, written by collectors, read by transcription worker
- **Shutdown**: `signal.NotifyContext(SIGINT, SIGTERM)` → ctx cancelled → all goroutines exit via `select` on `ctx.Done()` → `sync.WaitGroup.Wait()` → lock released
- **Segmenter flush**: transcription worker calls `segmenter.Flush()` after draining channel; flushes join all ephemeral summarization goroutines (3 min timeout)

## Segmentation

### Online: IncrementalSegmenter

Detects boundaries and finalizes segments as they complete:

- **Boundary detected** when: silence crosses 5 min, meeting tab changes, or user pins
- **Boundary finalized** when: speech resumes after the boundary
- **Finalization triggers** summarization immediately (in background goroutine)

### Boundary Triggers

| Trigger            | Detects                                        |
| ------------------ | ---------------------------------------------- |
| Silence ≥ 5 min    | Topic changes in long calls, gaps between work |
| Meeting tab change | Back-to-back meetings with no silence gap      |
| Pin                | Anything the algorithm misses                  |

## Summarization

Local LLM produces structured markdown summaries per segment.

- **Short segments** (≤35k chars): single LLM call
- **Long segments**: map-reduce — summarize each chunk → combine results
- **Output**: `<segments.output_dir>/YYYY-MM-DD-HHMM-<slug>.md`

## Speaker Attribution & Meeting Detection (CDP)

`SpeakerDetector` scans CDP ports for meeting tabs, auto-detects platform (Meet/Teams),
discovers speaking indicator class via temporal diffing of CSS class sets.

- **Speaker detection**: exact — platform's own visual indicator, ~1s polling interval
- **Meeting detection**: tab URL/title changes trigger `MeetingState.Set()`
- **Platforms**: Meet (port 9224), Teams (port 9223)
- **Cache invalidation**: WebSocket URL change → reset discovery

## Lockfile

- **Location**: `<transcript_output_dir>/.recorder-lock`
- **Contents**: JSON `{hostname, pid, updated}`
- **Heartbeat**: 30s, **Stale after**: 120s
