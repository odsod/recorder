# Recorder — Agent Instructions

Ambient meeting recorder daemon. Always-on capture → transcription → daily event log.

- **Design doc**: `~/Vaults/odsod/wiki/synthesis/ambient-meeting-recorder.md`

## Architecture

```
parec (mic) ─┐                          ┌─ server-side ─┐
             ├→ RMS gate → AudioChunk → │ Silero VAD    │→ LLM cleanup ─┐
parec (sys) ─┘   (with timestamps)      │ whisper decode│               │
                                         └───────────────┘               │
                                                                         ↓
CDP (Meet/Teams) → SpeakerTimeline ←─── Transcription Worker → Transcript
KWin (kdotool) → WindowTimeline ←───────────────┘                  ↓
                                                          IncrementalSegmenter
                                                                  ↓
                                                          Summarize → Inbox
```

- **Daemon** — runs in a tmux session, stdout is a structured log stream
- **No TUI** — plain timestamped log output, composable with tmux panes
- **Sole writer** — only the transcription goroutine writes to the transcript file
- **Notes** — `Meta+W` launches `recorder note` via KWin/kdialog
- **Toggle** — `recorder-toggle` creates/switches tmux session

### Goroutine Model

| Goroutine                  | Role                                                               | Writes to transcript?    |
| -------------------------- | ------------------------------------------------------------------ | ------------------------ |
| Main (capture loop)        | parec read, RMS gating, silence counting, segmenter.OnSilence()    | No                       |
| Transcription worker       | Whisper → cleanup → speaker lookup → write → segmenter.OnSpeech()  | Yes (sole writer)        |
| Speaker collector          | CDP polling → SpeakerTimeline + ParticipantSet                     | No                       |
| Window collector           | KWin polling → WindowTimeline                                      | No                       |
| Input loop                 | Keyboard (p/s) via raw terminal                                    | No                       |
| Ephemeral (per segment)    | Summarization + inbox draft                                        | Yes (seg marker + inbox) |

Critical invariant: only the transcription goroutine calls `transcript.Append()` for speech and signal lines.

## Language & Build

- **Language**: Go
- **Module**: `github.com/odsod/machine/recorder`
- **Binary**: single static binary → `~/.local/bin/recorder`
- **External deps**: `github.com/coder/websocket` (CDP), `golang.org/x/term` (raw stdin)
- **Config**: JSON (`~/.config/recorder/config.json`, symlinked from `hosts/`)
- **Build**: `go build -o ~/.local/bin/recorder ./cmd/recorder`

## Structure

```
recorder/
├── cmd/recorder/main.go          # Entrypoint, subcommand dispatch, Recorder struct
├── internal/
│   ├── config/config.go          # Config structs + JSON loading
│   ├── audio/                    # capture.go (parec), rms.go, wav.go
│   ├── timeline/                 # speaker.go, window.go (mutex-guarded ring buffers)
│   ├── cdp/                      # client.go (WebSocket), detector.go, platforms.go
│   ├── kwin/kwin.go              # kdotool wrapper
│   ├── transcribe/               # whisper.go, cleanup.go, dedup.go
│   ├── transcript/               # daily.go (append-only), parse.go
│   ├── segment/                  # boundary.go (pure), segmenter.go (state machine)
│   ├── summarize/                # summarize.go, inbox.go, prompts.go (//go:embed)
│   ├── signals/                  # speaker.go, window.go, silence.go (collector goroutines)
│   ├── lock/lock.go              # JSON lockfile with heartbeat
│   └── note/note.go              # kdialog → transcript append
├── hosts/                        # Host-specific JSON configs
├── recorder-toggle               # Fish script — tmux session toggle
└── Makefile
```

## CLI

Single `recorder` binary, all functionality as subcommands:

```
recorder run                                  # start the daemon
recorder note                                 # desktop note dialog
recorder segment <transcript>                 # dry-run: show boundaries + summaries
recorder segment <transcript> --write         # write inbox drafts + seg markers
recorder segment <transcript> --boundaries    # only show boundaries, no LLM calls
```

## Daemon Controls

Keybindings in the tmux pane (raw terminal input, no prefix):

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
6. Whisper decodes only VAD-approved segments (ROCm GPU)
7. LLM cleanup (filler, grammar, dedup, hallucination filtering)
8. **Speaker resolution** — query SpeakerTimeline for chunk's time window
9. **Audio dedup** — mic text suppressed if token overlap ≥ 60% with system text
10. Append clean timestamped speech to daily transcript with inline speaker attribution

## Transcript Format

Append-only daily event log at `~/Vaults/odsod/raw/transcripts/YYYY-MM-DD-recorder.md`.

Every line: `[HH:MM:SS] <emoji> **<tag>** <text>`

| Tag | Emoji | Source                                                           |
| --- | ----- | ---------------------------------------------------------------- |
| sys | 🔊    | System audio transcription (with inline `[Speaker]` attribution) |
| mic | 🎤    | Mic audio transcription (with inline `[Speaker]` attribution)    |
| win | 🪟    | kdotool polling — open/close/title change                        |
| ppl | 👥    | CDP polling — participant set changes                            |
| idl | 💤    | Silence detector                                                 |
| nfo | 📝    | User — freeform annotation (`Meta+W`)                            |
| pin | 📍    | User — segment boundary hint (`s` in recorder pane)              |
| seg | ✂️    | Segmenter — segment boundary emitted                             |
| rec | 🟢/🔴 | Recorder started/stopped                                         |

## Runtime Dependencies

| Service        | URL                                             | Purpose            |
| -------------- | ----------------------------------------------- | ------------------ |
| whisper-server | `http://odsod-desktop:8178/v1/audio/…`          | ASR (ROCm GPU)     |
| llama-server   | `http://odsod-desktop:8179/v1/chat/completions` | Cleanup (Qwen 3.5) |

System: `pulseaudio-utils` (parec), `kdotool`, `kdialog`.
Chrome (Meet): `--remote-debugging-port=9224`.
Chrome (Teams): `--remote-debugging-port=9223`.

## Development

- **Install**: `make -C recorder install`
- **Build only**: `make -C recorder build`
- **Test**: `make -C recorder test` or `go test ./...`
- **Config**: `~/.config/recorder/config.json` (symlink to host config)
- **Host source**: `recorder/hosts/$(hostname).json`

### Config Sections

```json
{
  "audio": { "sample_rate": 16000, "format": "s16le" },
  "whisper": { "url": "...", "timeout_s": 60 },
  "llm": { "url": "...", "timeout_s": 180 },
  "transcript": { "output_dir": "~/Vaults/odsod/raw/transcripts" },
  "dedup": { "threshold": 0.6 },
  "signals": {
    "silence_threshold_secs": 180,
    "kwin_poll_interval_secs": 10,
    "meeting_window_patterns": ["meet.google.com"],
    "cdp_ports": [9224, 9223]
  }
}
```

## Concurrency Design

- **Channel**: `chan AudioChunk` (buffered, cap 8) — capture → transcription
- **Timelines**: `sync.Mutex`-guarded structs, written by collectors, read by transcription worker
- **Shutdown**: `signal.NotifyContext(SIGINT, SIGTERM)` → ctx cancelled → all goroutines exit via `select` on `ctx.Done()` → `sync.WaitGroup.Wait()` → lock released
- **Segmenter flush**: transcription worker calls `segmenter.Flush()` after draining channel; flushes join all ephemeral summarization goroutines (3 min timeout)

## Segmentation

### Online: IncrementalSegmenter

Detects boundaries and finalizes segments as they complete:

- **Boundary detected** when: silence crosses 5 min, meeting identity changes, or user pins
- **Boundary finalized** when: speech resumes after the boundary
- **Finalization triggers** summarization immediately (in background goroutine)

### Boundary Triggers

| Trigger                 | Detects                                        |
| ----------------------- | ---------------------------------------------- |
| Silence ≥ 5 min         | Topic changes in long calls, gaps between work |
| Meeting identity change | Back-to-back meetings with no silence gap      |
| Pin                     | Anything the algorithm misses                  |

## Summarization

Local LLM (Qwen 3.5 9B) produces structured markdown summaries per segment.

- **Short segments** (≤35k chars): single LLM call
- **Long segments**: map-reduce — summarize each chunk → combine results
- **Output**: `~/Vaults/odsod/inbox/YYYY-MM-DD-HHMM-<slug>.md`

## Speaker Attribution (CDP)

`SpeakerDetector` scans CDP ports for meeting tabs, auto-detects platform (Meet/Teams),
discovers speaking indicator class via temporal diffing of CSS class sets.

- **Accuracy**: exact — platform's own visual indicator
- **Latency**: ~1s polling interval
- **Platforms**: Meet (port 9224), Teams (port 9223)
- **Cache invalidation**: WebSocket URL change → reset discovery

## Lockfile

- **Location**: `<transcript_output_dir>/.recorder-lock`
- **Contents**: JSON `{hostname, pid, updated}`
- **Heartbeat**: 30s, **Stale after**: 120s
