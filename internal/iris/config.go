package iris

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const maxConfigBytes = 64 << 10

type config struct {
	dataRoot    string
	database    string
	runtimeRoot string
	socket      string
	configPath  string
	settings    settings
}

type settings struct {
	SchemaVersion         int `json:"schema_version"`
	MaxConnections        int `json:"max_connections"`
	RequestTimeoutSeconds int `json:"request_timeout_seconds"`
}

type environment struct {
	home       string
	dataHome   string
	configHome string
	runtimeDir string
}

func defaultConfig() (config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return config{}, fmt.Errorf("resolve home: %w", err)
	}

	resolved, err := configFromEnvironment(environment{
		home: home, dataHome: os.Getenv("XDG_DATA_HOME"),
		configHome: os.Getenv("XDG_CONFIG_HOME"), runtimeDir: os.Getenv("XDG_RUNTIME_DIR"),
	})
	if err != nil {
		return config{}, err
	}

	if err := resolved.loadSettings(); err != nil {
		return config{}, err
	}

	return resolved, nil
}

func configFromEnvironment(env environment) (config, error) {
	paths := []struct {
		name  string
		value string
	}{
		{"home", env.home},
		{"XDG_DATA_HOME", env.dataHome},
		{"XDG_CONFIG_HOME", env.configHome},
		{"XDG_RUNTIME_DIR", env.runtimeDir},
	}
	for _, path := range paths {
		if path.value != "" && !filepath.IsAbs(path.value) {
			return config{}, fmt.Errorf("%s must be absolute", path.name)
		}
	}

	if env.home == "" || env.runtimeDir == "" {
		return config{}, fmt.Errorf("home and XDG_RUNTIME_DIR are required")
	}

	if env.dataHome == "" {
		env.dataHome = filepath.Join(env.home, ".local", "share")
	}

	if env.configHome == "" {
		env.configHome = filepath.Join(env.home, ".config")
	}

	dataRoot := filepath.Join(env.dataHome, "iris")
	runtimeRoot := filepath.Join(env.runtimeDir, "iris")

	return config{
		dataRoot: dataRoot, database: filepath.Join(dataRoot, "iris.db"),
		runtimeRoot: runtimeRoot, socket: filepath.Join(runtimeRoot, "iris.sock"),
		configPath: filepath.Join(env.configHome, "iris", "config.json"),
		settings:   settings{SchemaVersion: 1, MaxConnections: 128, RequestTimeoutSeconds: 10},
	}, nil
}

func (cfg *config) loadSettings() error {
	file, err := os.Open(cfg.configPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("open configuration: %w", err)
	}

	data, readErr := io.ReadAll(io.LimitReader(file, maxConfigBytes+1))

	closeErr := file.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return fmt.Errorf("read configuration: %w", err)
	}

	if len(data) > maxConfigBytes {
		return fmt.Errorf("configuration exceeds 64 KiB")
	}

	if err := decodeJSON(data, &cfg.settings); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg.settings.validate()
}

func (s settings) validate() error {
	if s.SchemaVersion != 1 {
		return fmt.Errorf("configuration schema_version must be 1")
	}

	if s.MaxConnections < 1 || s.MaxConnections > 128 {
		return fmt.Errorf("max_connections must be between 1 and 128")
	}

	if s.RequestTimeoutSeconds < 1 || s.RequestTimeoutSeconds > 60 {
		return fmt.Errorf("request_timeout_seconds must be between 1 and 60")
	}

	return nil
}
