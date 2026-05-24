package app

import (
	"net/http"
	"time"

	"github.com/odsod/recorder/internal/audio"
	"github.com/odsod/recorder/internal/conference"
	"github.com/odsod/recorder/internal/conference/meet"
	"github.com/odsod/recorder/internal/conference/teams"
	"github.com/odsod/recorder/internal/config"
	"github.com/odsod/recorder/internal/httpclient"
	"github.com/odsod/recorder/internal/protocol/cdp"
	"github.com/odsod/recorder/internal/protocol/llm"
	"github.com/odsod/recorder/internal/protocol/parec"
	"github.com/odsod/recorder/internal/protocol/whisper"
	"github.com/odsod/recorder/internal/speaker"
	"github.com/odsod/recorder/internal/summarize"
	"github.com/odsod/recorder/internal/transcribe"
)

type Deps struct {
	Config          config.Config
	HTTP            *http.Client
	LLM             *llm.Client
	Whisper         *whisper.Client
	Cleaner         *transcribe.Cleaner
	Summarizer      *summarize.Summarizer
	SpeakerDetector *speaker.Detector
	Capture         audio.Capture
}

func BuildDeps(cfg config.Config) Deps {
	httpClient := httpclient.New()

	llmClient := llm.New(httpClient, llm.Config{
		URL:         cfg.LLM.URL,
		Model:       cfg.LLM.Model,
		Timeout:     time.Duration(cfg.LLM.TimeoutS) * time.Second,
		Temperature: 0.3,
		MaxTokens:   4096,
	})

	cdpClient := cdp.New(httpClient)

	return Deps{
		Config: cfg,
		HTTP:   httpClient,
		LLM:    llmClient,
		Whisper: whisper.New(httpClient, whisper.Config{
			URL:     cfg.Whisper.URL,
			Timeout: time.Duration(cfg.Whisper.TimeoutS) * time.Second,
		}),
		Cleaner:    transcribe.NewCleaner(llmClient),
		Summarizer: summarize.NewSummarizer(llmClient),
		SpeakerDetector: speaker.NewDetector(cdpClient, cdpClient, cfg.Signals.CDPPorts,
			[]conference.Provider{meet.New(), teams.New()}),
		Capture: audio.NewParecCapture(parec.NewDefault()),
	}
}

func (d Deps) Close() {
	httpclient.Close(d.HTTP)
}
