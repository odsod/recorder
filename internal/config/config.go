package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type WhisperConfig struct {
	URL      string `json:"url"`
	TimeoutS int    `json:"timeout_s"`
}

type LLMConfig struct {
	URL      string `json:"url"`
	TimeoutS int    `json:"timeout_s"`
}

type TranscriptConfig struct {
	OutputDir string `json:"output_dir"`
}

type DedupConfig struct {
	Threshold float64 `json:"threshold"`
}

type SignalsConfig struct {
	SilenceThresholdSecs  int      `json:"silence_threshold_secs"`
	KwinPollIntervalSecs  int      `json:"kwin_poll_interval_secs"`
	MeetingWindowPatterns []string `json:"meeting_window_patterns"`
	CDPPorts              []int    `json:"cdp_ports"`
}

type Config struct {
	Whisper    WhisperConfig    `json:"whisper"`
	LLM        LLMConfig        `json:"llm"`
	Transcript TranscriptConfig `json:"transcript"`
	Dedup      DedupConfig      `json:"dedup"`
	Signals    SignalsConfig     `json:"signals"`
}

func defaults() Config {
	return Config{
		Whisper: WhisperConfig{
			URL:      "http://localhost:8178/v1/audio/transcriptions",
			TimeoutS: 60,
		},
		LLM: LLMConfig{
			URL:      "http://localhost:8179/v1/chat/completions",
			TimeoutS: 180,
		},
		Transcript: TranscriptConfig{
			OutputDir: filepath.Join(HomeDir(), "Vaults/odsod/raw/transcripts"),
		},
		Dedup: DedupConfig{
			Threshold: 0.6,
		},
		Signals: SignalsConfig{
			SilenceThresholdSecs:  180,
			KwinPollIntervalSecs:  10,
			MeetingWindowPatterns: []string{"meet.google.com"},
			CDPPorts:              []int{9224, 9223},
		},
	}
}

func Load() (Config, error) {
	cfg := defaults()

	configPath := filepath.Join(HomeDir(), ".config/recorder/config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	cfg.Transcript.OutputDir = expandHome(cfg.Transcript.OutputDir)

	return cfg, nil
}

func expandHome(path string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		return filepath.Join(HomeDir(), path[2:])
	}
	return path
}

func HomeDir() string {
	home, _ := os.UserHomeDir()
	return home
}
