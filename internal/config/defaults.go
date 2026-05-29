package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// getDefaults returns the default configuration.
func getDefaults() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "localhost",
			Port: 3100,
		},
		Database: DatabaseConfig{
			URL: "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev",
		},
		Embedding: EmbeddingConfig{
			Provider:    "ollama",
			URL:         "http://localhost:11434",
			Model:       "nomic-embed-text",
			Concurrency: 3,
			VoyageAPIKey: "",
		},
		Harvester: HarvesterConfig{
			OpenCode: OpenCodeHarvesterConfig{
				SessionDir: "",
				DBPath:     "",
				DBRoot:     "",
			},
			ClaudeCode: ClaudeCodeHarvesterConfig{
				Enabled:    false,
				SessionDir: "",
			},
		},
		Intervals: IntervalsConfig{
			SessionPoll: 120,
		},
		Watcher: WatcherConfig{
			DebounceMs:      2000,
			ReindexInterval: 300,
		},
		Search: SearchConfig{
			RrfK:                60,
			RecencyWeight:       0.3,
			RecencyHalfLifeDays: 180,
			Limit:               20,
		},
		Storage: StorageConfig{
			MaxFileSize: 314572800,  // 300MB
			MaxSize:     10737418240, // 10GB
		},
		Telemetry: TelemetryConfig{
			RetentionDays: 90,
		},
		Logging: LoggingConfig{
			Level: "info",
			File:  "",
		},
		Summarization: SummarizationConfig{
			Enabled:     false,
			ProviderURL: "",
			APIKey:      "",
			Model:       "nano-brain",
			MaxTokens:   8000,
			Concurrency: 3,
		},
	}
}

// GenerateDefault writes a default configuration YAML file to the given path.
func GenerateDefault(path string) error {
	if path == "" {
		return errors.New("config path cannot be empty")
	}

	// Expand tilde if present
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// Create parent directories if they don't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	// Marshal defaults to YAML
	defaults := getDefaults()
	data, err := yaml.Marshal(defaults)
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", path, err)
	}

	return nil
}
