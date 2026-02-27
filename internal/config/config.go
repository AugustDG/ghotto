package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config holds the user's ghotto configuration.
type Config struct {
	Model string `toml:"model"`
}

// Default returns the default configuration.
func Default() Config {
	return Config{
		Model: "anthropic/claude-sonnet-4-20250514",
	}
}

// Load reads the config from disk, creating a default one if it doesn't exist.
// Returns the config, the path it was loaded from, and any error.
func Load() (Config, string, error) {
	cfg := Default()
	path, err := configPath()
	if err != nil {
		return cfg, "", err
	}

	if _, err := os.Stat(path); err == nil {
		if _, err := toml.DecodeFile(path, &cfg); err != nil {
			return cfg, path, err
		}
	} else if errors.Is(err, os.ErrNotExist) {
		if err := Save(path, cfg); err != nil {
			return cfg, path, err
		}
		return cfg, path, nil
	} else {
		return cfg, path, err
	}

	return cfg, path, nil
}

// Save writes the config to disk at the given path.
func Save(path string, cfg Config) error {
	if err := ensureParent(path); err != nil {
		return err
	}
	buf := &bytes.Buffer{}
	if err := toml.NewEncoder(buf).Encode(cfg); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// SaveDefault saves the config to the default path.
func SaveDefault(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	return Save(path, cfg)
}

func configPath() (string, error) {
	if p := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); p != "" {
		return filepath.Join(p, "ghotto", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "ghotto", "config.toml"), nil
}

func ensureParent(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}
