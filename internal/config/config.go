package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	NotesDir   string `json:"notes_dir"`
	Editor     string `json:"editor"`
	AIEnabled  bool   `json:"ai_enabled"`
	GeminiKey  string `json:"api_key"`
	GeminiModel string `json:"model"`
}

type PairyConfig struct {
	APIKey string `json:"api_key"`
	Model  string `json:"model"`
}

func Load() (*Config, error) {
	cfg := &Config{
		NotesDir:    defaultNotesDir(),
		Editor:      defaultEditor(),
		AIEnabled:   true,
		GeminiModel: "gemini-2.5-flash",
	}

	// Load grove config if exists
	groveConfigPath := filepath.Join(xdgConfig(), "grove", "config.json")
	if data, err := os.ReadFile(groveConfigPath); err == nil {
		_ = json.Unmarshal(data, cfg)
	}

	// Fallback: load Gemini key from pairy config
	if cfg.GeminiKey == "" {
		pairyConfigPath := filepath.Join(xdgConfig(), "pairy", "config.json")
		if data, err := os.ReadFile(pairyConfigPath); err == nil {
			var pc PairyConfig
			if err := json.Unmarshal(data, &pc); err == nil {
				cfg.GeminiKey = pc.APIKey
				if pc.Model != "" {
					cfg.GeminiModel = pc.Model
				}
			}
		}
	}

	// Also check GEMINI_API_KEY env
	if cfg.GeminiKey == "" {
		cfg.GeminiKey = os.Getenv("GEMINI_API_KEY")
	}

	// Ensure notes dir exists
	if err := os.MkdirAll(cfg.NotesDir, 0755); err != nil {
		return nil, err
	}

	return cfg, nil
}

func Save(cfg *Config) error {
	dir := filepath.Join(xdgConfig(), "grove")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0644)
}

func xdgConfig() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

func defaultNotesDir() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "grove", "notes")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "grove", "notes")
}

func defaultEditor() string {
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	if e := os.Getenv("VISUAL"); e != "" {
		return e
	}
	return "vim"
}
