package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// WhisperConfig holds whisper-server connection settings.
type WhisperConfig struct {
	URL      string `json:"url"`
	TimeoutS int    `json:"timeoutS"`
}

// LLMConfig holds LLM server connection settings.
type LLMConfig struct {
	URL      string `json:"url"`
	Model    string `json:"model"`
	TimeoutS int    `json:"timeoutS"`
}

// TranscriptConfig holds transcript output settings.
type TranscriptConfig struct {
	OutputDir string `json:"outputDir"`
}

// SegmentsConfig holds segment file output settings.
type SegmentsConfig struct {
	OutputDir string `json:"outputDir"`
}

// DedupConfig holds audio deduplication settings.
type DedupConfig struct {
	Threshold float64 `json:"threshold"`
}

// SignalsConfig holds signal detection settings.
type SignalsConfig struct {
	SilenceThresholdS int   `json:"silenceThresholdS"`
	CDPPorts          []int `json:"cdpPorts"`
}

// LogConfig holds logging settings.
type LogConfig struct {
	// File is the path for JSONL log output. Empty disables file logging.
	File string `json:"file"`
}

// Config is the top-level application configuration.
type Config struct {
	Whisper     WhisperConfig     `json:"whisper"`
	LLM         LLMConfig         `json:"llm"`
	Transcript  TranscriptConfig  `json:"transcript"`
	Segments    SegmentsConfig    `json:"segments"`
	Dedup       DedupConfig       `json:"dedup"`
	Signals     SignalsConfig     `json:"signals"`
	Log         LogConfig         `json:"log"`
	PromptPaths PromptPathsConfig `json:"prompts"`
	PromptVars  PromptVarsConfig  `json:"promptVars"`
	Prompts     Prompts           `json:"-"`
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
		PromptVars: defaultPromptVars(),
	}
}

// Load reads config from $XDG_CONFIG_HOME/recorder/config.json, falling back to defaults.
func Load() (Config, error) {
	cfg := defaults()

	configPath := filepath.Join(Dir(), "recorder", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return finalize(cfg)
		}
		return cfg, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	return finalize(cfg)
}

func finalize(cfg Config) (Config, error) {
	cfg.Transcript.OutputDir = expandHome(cfg.Transcript.OutputDir)
	cfg.Segments.OutputDir = expandHome(cfg.Segments.OutputDir)
	cfg.Log.File = expandHome(cfg.Log.File)
	cfg.PromptPaths.Cleanup = expandHome(cfg.PromptPaths.Cleanup)
	cfg.PromptPaths.Summarize = expandHome(cfg.PromptPaths.Summarize)
	cfg.PromptPaths.Combine = expandHome(cfg.PromptPaths.Combine)
	cfg.PromptVars = mergePromptVars(cfg.PromptVars, defaultPromptVars())

	prompts, err := resolvePrompts(cfg.PromptPaths, cfg.PromptVars)
	if err != nil {
		return cfg, err
	}
	cfg.Prompts = prompts

	return cfg, nil
}

func expandHome(path string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		return filepath.Join(HomeDir(), path[2:])
	}
	return path
}

// HomeDir returns the user's home directory.
func HomeDir() string {
	home, _ := os.UserHomeDir()
	return home
}

// Dir returns the XDG config directory.
func Dir() string {
	return envDir("XDG_CONFIG_HOME", filepath.Join(HomeDir(), ".config"))
}

// DataDir returns the XDG data directory.
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
