package app

import (
	"net/http"

	"github.com/odsod/recorder/internal/audio"
	"github.com/odsod/recorder/internal/cdp"
	"github.com/odsod/recorder/internal/config"
	"github.com/odsod/recorder/internal/httpclient"
	"github.com/odsod/recorder/internal/llm"
	"github.com/odsod/recorder/internal/summarize"
	"github.com/odsod/recorder/internal/transcribe"
)

type Deps struct {
	Config          config.Config
	HTTP            *http.Client
	Whisper         *transcribe.WhisperClient
	LLM             *llm.Client
	Cleaner         *transcribe.Cleaner
	Summarizer      *summarize.Summarizer
	CDP             *cdp.Client
	SpeakerDetector *cdp.SpeakerDetector
	Capture         audio.Capture
}

func BuildDeps(cfg config.Config) Deps {
	httpClient := httpclient.New()
	llmClient := llm.New(httpClient, cfg.LLM)
	cdpClient := cdp.NewClient(httpClient)

	return Deps{
		Config:          cfg,
		HTTP:            httpClient,
		Whisper:         transcribe.NewWhisperClient(httpClient, cfg.Whisper),
		LLM:             llmClient,
		Cleaner:         transcribe.NewCleaner(llmClient),
		Summarizer:      summarize.NewSummarizer(llmClient),
		CDP:             cdpClient,
		SpeakerDetector: cdp.NewSpeakerDetector(cdpClient, cfg.Signals.CDPPorts),
		Capture:         audio.NewParecCapture(),
	}
}

func (d Deps) Close() {
	httpclient.Close(d.HTTP)
}
