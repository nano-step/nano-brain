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
			Auth: AuthConfig{
				Enabled:     false,
				Realm:       "nano-brain",
				BypassPaths: []string{"/health"},
			},
		},
		Database: DatabaseConfig{
			URL: "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev",
		},
		Embedding: EmbeddingConfig{
			Provider:    "ollama",
			URL:         "http://localhost:11434",
			Model:       "nomic-embed-text",
			Concurrency: 3,
			MaxChars:    3000,
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
			Git: GitHarvesterConfig{
				Enabled: false,
			},
		},
		Intervals: IntervalsConfig{
			SessionPoll: 120,
		},
		Watcher: WatcherConfig{
			DebounceMs:      2000,
			ReindexInterval: 300,
			ChunkOverlap:    600,
		},
		Search: SearchConfig{
			RrfK:                  60,
			RecencyWeight:         0.3,
			RecencyHalfLifeDays:   180,
			Limit:                 20,
		PageRankEnabled:       false,
		PageRankWeight:        0.2,
		PageRankEdgeThreshold: 100,
		EntityBoostEnabled:    false,
		EntityBoostFactor:     0.3,
		QueryPreprocessing: QueryPreprocessingConfig{
			Enabled:      false,
			ProviderURL:  "",
			APIKey:       "",
			Model:        "",
			MaxLatencyMs: 500,
		},
		BM25Language:         "english",
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
			OutputDir:   "~/.nano-brain/summaries",
		},
		CodeSummarization: CodeSummarizationConfig{
			Enabled:              false,
			ProviderURL:          "",
			APIKey:               "",
			Model:                "",
			BatchSize:            30,
			MaxOutputTokens:      8000,
			Concurrency:          2,
			MaxRequestsPerDay:    0,
			MaxSymbolLines:       500,
			PollIntervalSeconds:  60,
			MaxSummariesPerCycle: 300,
			FallbackModel:        "",
			MaxBatchTokens:       100000,
			MaxRetries:           3,
			RetryBackoffSeconds:  1,
		},
		Flow: FlowConfig{
			Enabled:   false,
			MaxDepth:  10,
			MaxFanout: 8,
		},
		Intelligence: IntelligenceConfig{
			Enabled:          false,
			ProviderURL:      "",
			APIKey:           "",
			Model:            "claude-sonnet-4-5",
			MaxTokens:        8000,
			Concurrency:      3,
			ConsolidationAge: 7,
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
