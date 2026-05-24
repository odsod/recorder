package main

import (
	"net/http"
	"time"

	"github.com/odsod/recorder/internal/capture"
	"github.com/odsod/recorder/internal/conference"
	"github.com/odsod/recorder/internal/conference/meet"
	"github.com/odsod/recorder/internal/conference/teams"
	"github.com/odsod/recorder/internal/config"
	"github.com/odsod/recorder/internal/protocol/cdp"
	"github.com/odsod/recorder/internal/protocol/llm"
	"github.com/odsod/recorder/internal/protocol/parec"
	"github.com/odsod/recorder/internal/protocol/whisper"
	"github.com/odsod/recorder/internal/recorder"
	"github.com/odsod/recorder/internal/segment"
	"github.com/odsod/recorder/internal/speaker"
	"github.com/odsod/recorder/internal/summarize"
	"github.com/odsod/recorder/internal/transcribe"
)

type deps struct {
	llm             *llm.Client
	whisper         *whisper.Client
	cleaner         *transcribe.Cleaner
	summarizer      *summarize.Summarizer
	speakerDetector *speaker.Detector
	capture         capture.Source
}

func buildDeps(cfg config.Config, httpClient *http.Client) deps {
	llmClient := llm.New(httpClient, llm.Config{
		URL:         cfg.LLM.URL,
		Model:       cfg.LLM.Model,
		Timeout:     time.Duration(cfg.LLM.TimeoutS) * time.Second,
		Temperature: 0.3,
		MaxTokens:   4096,
	})

	cdpClient := cdp.New(httpClient)

	return deps{
		llm: llmClient,
		whisper: whisper.New(httpClient, whisper.Config{
			URL:     cfg.Whisper.URL,
			Timeout: time.Duration(cfg.Whisper.TimeoutS) * time.Second,
		}),
		cleaner:    transcribe.NewCleaner(llmClient, cfg.Prompts.Cleanup),
		summarizer: summarize.NewSummarizer(llmClient, cfg.Prompts.Summarize, cfg.Prompts.Combine),
		speakerDetector: speaker.NewDetector(cdpClient, cdpClient, cfg.Signals.CDPPorts,
			[]conference.Provider{meet.New(), teams.New()}),
		capture: capture.NewParec(parec.NewDefault()),
	}
}

func recorderServices(cfg config.Config, d deps) recorder.Services {
	return recorder.Services{
		Transcriber:     d.whisper,
		Cleaner:         d.cleaner,
		Summarizer:      d.summarizer,
		SpeakerDetector: d.speakerDetector,
		Capture:         d.capture,
		SegmentHandler: &segment.FuncHandler{
			SummarizeFn: d.summarizer.SummarizeSegment,
			WriteSegmentFn: func(title, summary string, seg segment.Segment, date string) (string, error) {
				return summarize.WriteSegmentFile(title, summary, seg, date, cfg.Segments.OutputDir)
			},
		},
	}
}
