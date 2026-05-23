package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type WhisperConfig struct {
	URL      string `json:"url"`
	TimeoutS int    `json:"timeoutS"`
}

type LLMConfig struct {
	URL      string `json:"url"`
	Model    string `json:"model"`
	TimeoutS int    `json:"timeoutS"`
}

type TranscriptConfig struct {
	OutputDir string `json:"outputDir"`
}

type SegmentsConfig struct {
	OutputDir string `json:"outputDir"`
}

type DedupConfig struct {
	Threshold float64 `json:"threshold"`
}

type SignalsConfig struct {
	SilenceThresholdS int   `json:"silenceThresholdS"`
	CDPPorts          []int `json:"cdpPorts"`
}

type Config struct {
	Whisper    WhisperConfig    `json:"whisper"`
	LLM        LLMConfig        `json:"llm"`
	Transcript TranscriptConfig `json:"transcript"`
	Segments   SegmentsConfig   `json:"segments"`
	Dedup      DedupConfig      `json:"dedup"`
	Signals    SignalsConfig    `json:"signals"`
}

func defaults() Config {
	return Config{
		Whisper: WhisperConfig{
			URL:      "http://localhost:8178/v1/audio/transcriptions",
			TimeoutS: 60,
		},
		LLM: LLMConfig{
			URL:      "http://localhost:8179/v1/chat/completions",
			Model:    "default",
			TimeoutS: 180,
		},
		Transcript: TranscriptConfig{
			OutputDir: filepath.Join(DataDir(), "recorder", "transcripts"),
		},
		Segments: SegmentsConfig{
			OutputDir: filepath.Join(DataDir(), "recorder", "segments"),
		},
		Dedup: DedupConfig{
			Threshold: 0.6,
		},
		Signals: SignalsConfig{
			SilenceThresholdS: 180,
			CDPPorts:          []int{},
		},
	}
}

func Load() (Config, error) {
	cfg := defaults()

	configPath := filepath.Join(ConfigDir(), "recorder", "config.json")
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
	cfg.Segments.OutputDir = expandHome(cfg.Segments.OutputDir)

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

func ConfigDir() string {
	return envDir("XDG_CONFIG_HOME", filepath.Join(HomeDir(), ".config"))
}

func DataDir() string {
	return envDir("XDG_DATA_HOME", filepath.Join(HomeDir(), ".local", "share"))
}

func envDir(name, fallback string) string {
	dir := expandHome(os.Getenv(name))
	if dir != "" && filepath.IsAbs(dir) {
		return dir
	}
	return fallback
}
