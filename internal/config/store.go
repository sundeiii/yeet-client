package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// JsonStore implements the Store interface using json files
type JsonStore struct {
	Path string
}

// NewStore creates a new config store for the current platform.
func NewStore() Store {
	// This should automatically resolve to the proper user configuration directory
	// for each platform (e.g. %appdata% or ~/.config)
	configDir, err := os.UserConfigDir()

	if err != nil {
		// Fallback to ~/.config if UserConfigDir fails for some reason
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config")
	}

	return &JsonStore{
		Path: filepath.Join(configDir, "yeet", "config.json"),
	}
}

// Load reads the configuration from the json file.
func (s *JsonStore) Load() (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(s.Path)
	if os.IsNotExist(err) {
		// Return default configuration if the file doesn't exist yet
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes the configuration to the json file.
func (s *JsonStore) Save(cfg *Config) error {
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.Path, data, 0644)
}
