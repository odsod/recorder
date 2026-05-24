# recorder

Ambient audio recorder daemon for Linux workstations. Captures microphone and
system audio via PipeWire, transcribes with a local whisper server, attributes
speakers via Chrome DevTools Protocol, segments conversations, and produces
structured summaries via a local LLM.

## Architecture

```
                                    ┌────────────────┐
 parec (mic) ──┐                    │ whisper-server │
               ├─→ RMS gate ─→ WAV ─│ (Silero VAD +  │─→ LLM cleanup ────┐
 parec (sys) ──┘   (silence         │  ASR decode)   │                   │
                    detection)      └────────────────┘                   │
                                                                         ▼
 Chrome CDP ──→ SpeakerTimeline ←── Transcription Worker ──→ DailyTranscript
 (Meet/Teams)   (who spoke when)            │                (append-only)
           └──→ MeetingState ──────→ IncrementalSegmenter
                (tab changes)               │
                                            ▼
                                    LLM Summarization ──→ Segment Files
```

**Goroutines**: capture loop (main), transcription worker, speaker collector
(CDP polling). Only the transcription worker writes to the transcript file.

## Requirements

- Linux with PipeWire (or PulseAudio)
- `parec` (from `pipewire-utils` or `pulseaudio-utils`)
- whisper-server with OpenAI-compatible `/v1/audio/transcriptions` endpoint
- LLM server with OpenAI-compatible `/v1/chat/completions` endpoint
- Chrome/Chromium with `--remote-debugging-port` (for speaker + meeting detection)

## Installation

```bash
go build -o ~/.local/bin/recorder .
```

## Configuration

Config is loaded from `$XDG_CONFIG_HOME/recorder/config.json` (default:
`~/.config/recorder/config.json`). If the file does not exist, built-in
defaults are used.

See [`config.example.json`](config.example.json) for all available fields.

| Section      | Field               | Default                               | Description                               |
| ------------ | ------------------- | ------------------------------------- | ----------------------------------------- |
| `whisper`    | `url`               | `http://localhost:8178/v1/...`        | Whisper server transcription endpoint     |
| `whisper`    | `timeoutS`          | `60`                                  | HTTP timeout for transcription requests   |
| `llm`        | `url`               | `http://localhost:8179/v1/...`        | LLM server chat completions endpoint      |
| `llm`        | `model`             | `default`                             | Model name sent in API requests           |
| `llm`        | `timeoutS`          | `180`                                 | HTTP timeout for LLM requests             |
| `transcript` | `outputDir`         | `~/.local/share/recorder/transcripts` | Directory for daily transcript files      |
| `segments`   | `outputDir`         | `~/.local/share/recorder/segments`    | Directory for segment summary files       |
| `dedup`      | `threshold`         | `0.6`                                 | Token overlap threshold for mic/sys dedup |
| `signals`    | `silenceThresholdS` | `180`                                 | Silence duration before segment boundary  |
| `signals`    | `cdpPorts`          | `[]`                                  | Chrome DevTools Protocol ports to poll    |

## Output

### Daily Transcripts

Streaming, append-only markdown files in `transcript.outputDir`:

```
<output_dir>/YYYY-MM-DD-recorder.md
```

Each file has YAML frontmatter and timestamped event lines:

```markdown
---
date: 2026-05-23
type: recorder-transcript
---

[15:04:32] 🔊 **sys** [Alice Smith] Let's migrate the API
[15:04:35] 🎤 **mic** We should start with the schema
[15:04:47] 🪟 **mtg** joined: Meet - API Planning
[15:05:01] 👥 **ppl** Alice Smith, Bob Johnson
[15:20:15] 💤 **idl** 15 min
[15:20:16] ✂️ **seg** | 1504 api-migration
```

**Event tags**: `sys` (system audio), `mic` (microphone), `mtg` (meeting
change), `ppl` (participants), `idl` (silence), `nfo` (user note), `pin`
(boundary hint), `seg` (segment boundary), `rec` (start/stop).

### Segment Summaries

Atomic markdown files in `segments.outputDir`:

```
<output_dir>/YYYY-MM-DD-HHMM-<slug>.md
```

Each file has YAML frontmatter with metadata and contains the LLM-generated
summary followed by the full segment transcript:

```yaml
---
title: "API Migration & Query Optimization"
date: 2026-05-23
time: "15:04–15:45"
duration: 41m
type: segment
source: "[[raw/transcripts/2026-05-23-recorder.md]]"
participants: ["Alice Smith", "Bob Johnson"]
---
```

## Usage

```bash
recorder run                              # start the daemon
recorder note                             # interactive note (stdin)
recorder note "meeting started late"      # note via CLI argument
recorder segment <transcript>             # show segments (dry-run)
recorder segment <transcript> --write     # write segment files + transcript markers
recorder segment <transcript> --boundaries  # show boundaries only (no LLM)
```

## Chrome DevTools Protocol

Speaker attribution and meeting detection require Chrome launched with remote
debugging. Configure the ports in `config.json` under `signals.cdpPorts`:

```bash
google-chrome --remote-debugging-port=9222
```

The recorder polls all configured CDP ports, auto-detects meeting tabs
(Google Meet, Microsoft Teams), identifies active speakers by observing CSS
class changes on participant tiles, and detects meeting changes when tabs
appear/disappear or titles change.
