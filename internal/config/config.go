// Package config loads YAML and environment configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/v2"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/rs/zerolog"
)

// Config holds all application configuration.
type Config struct {
	Server         ServerConfig         `koanf:"server"`
	Database       DatabaseConfig       `koanf:"database"`
	Embedding      EmbeddingConfig      `koanf:"embedding"`
	Harvester      HarvesterConfig      `koanf:"harvester"`
	Intervals      IntervalsConfig      `koanf:"intervals"`
	Watcher        WatcherConfig        `koanf:"watcher"`
	Search         SearchConfig         `koanf:"search"`
	Storage        StorageConfig        `koanf:"storage"`
	Telemetry      TelemetryConfig      `koanf:"telemetry"`
	Logging        LoggingConfig        `koanf:"logging"`
	Summarization  SummarizationConfig  `koanf:"summarization"`
}

// ServerConfig holds server configuration.
type ServerConfig struct {
	Host string `koanf:"host"`
	Port int    `koanf:"port"`
}

// DatabaseConfig holds database configuration.
type DatabaseConfig struct {
	URL string `koanf:"url"`
}

// EmbeddingConfig holds embedding provider configuration.
type EmbeddingConfig struct {
	Provider     string `koanf:"provider"`
	URL          string `koanf:"url"`
	Model        string `koanf:"model"`
	Dimension    int    `koanf:"dimension"`
	Concurrency  int    `koanf:"concurrency"`
	VoyageAPIKey string `koanf:"voyage_api_key"`
}

// HarvesterConfig holds harvester configuration.
type HarvesterConfig struct {
	OpenCode   OpenCodeHarvesterConfig   `koanf:"opencode"`
	ClaudeCode ClaudeCodeHarvesterConfig `koanf:"claudecode"`
}

// OpenCodeHarvesterConfig holds OpenCode harvester configuration.
type OpenCodeHarvesterConfig struct {
	SessionDir string `koanf:"session_dir"`
	DBPath     string `koanf:"db_path"`
}

// ClaudeCodeHarvesterConfig holds ClaudeCode harvester configuration.
type ClaudeCodeHarvesterConfig struct {
	Enabled    bool   `koanf:"enabled"`
	SessionDir string `koanf:"session_dir"`
}

// IntervalsConfig holds interval configuration.
type IntervalsConfig struct {
	SessionPoll int `koanf:"session_poll"`
}

type WorkspaceFilterConfig struct {
	ExcludePatterns   []string `koanf:"exclude_patterns"`
	AllowedExtensions []string `koanf:"allowed_extensions"`
}

type WatcherConfig struct {
	DebounceMs        int                               `koanf:"debounce_ms"`
	ReindexInterval   int                               `koanf:"reindex_interval"`
	ExcludePatterns   []string                          `koanf:"exclude_patterns"`
	AllowedExtensions []string                          `koanf:"allowed_extensions"`
	Workspaces        map[string]WorkspaceFilterConfig  `koanf:"workspaces"`
}

// SearchConfig holds search configuration.
type SearchConfig struct {
	RrfK                float64 `koanf:"rrf_k"`
	RecencyWeight       float64 `koanf:"recency_weight"`
	RecencyHalfLifeDays int     `koanf:"recency_half_life_days"`
	Limit               int     `koanf:"limit"`
}

// StorageConfig holds storage configuration.
type StorageConfig struct {
	MaxFileSize int64 `koanf:"max_file_size"`
	MaxSize     int64 `koanf:"max_size"`
}

type TelemetryConfig struct {
	RetentionDays int `koanf:"retention_days"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level string `koanf:"level"`
	File  string `koanf:"file"`
}

// SummarizationConfig holds summarization configuration.
type SummarizationConfig struct {
	Enabled           bool    `koanf:"enabled"`
	ProviderURL       string  `koanf:"provider_url"`
	APIKey            string  `koanf:"api_key"`
	Model             string  `koanf:"model"`
	MaxTokens         int     `koanf:"max_tokens"`
	Concurrency       int     `koanf:"concurrency"`
	RequestsPerSecond float64 `koanf:"requests_per_second"`
	OutputDir         string  `koanf:"output_dir"`
}

// Load loads configuration from file and environment variables.
// Config file path can be overridden via NANO_BRAIN_CONFIG env var.
// If no file is provided, defaults are used and merged with env vars.
func Load(configPath string) (*Config, error) {
	k := koanf.New(".")

	// Load defaults first
	defaults := getDefaults()
	if err := k.Load(structs.Provider(defaults, "koanf"), nil); err != nil {
		return nil, fmt.Errorf("failed to load defaults: %w", err)
	}

	// If config path is provided, load from file
	if configPath != "" {
		// Expand tilde if present
		if strings.HasPrefix(configPath, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to get home directory: %w", err)
			}
			configPath = filepath.Join(home, configPath[1:])
		}

		// Only try to load if file exists
		if _, err := os.Stat(configPath); err == nil {
			if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
				return nil, fmt.Errorf("failed to load config file %s: %w", configPath, err)
			}
		}
	}

	// Load environment variables
	// Standard NANO_BRAIN_ prefixed vars
	for _, envVar := range os.Environ() {
		if !strings.HasPrefix(envVar, "NANO_BRAIN_") {
			continue
		}

		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		// Convert NANO_BRAIN_SERVER_PORT to server.port
		// Only the first underscore (section boundary) becomes a dot
		key = strings.TrimPrefix(key, "NANO_BRAIN_")
		key = strings.ToLower(key)
		// Replace first underscore with dot (section.field mapping)
		if idx := strings.Index(key, "_"); idx != -1 {
			key = key[:idx] + "." + key[idx+1:]
		}
		// NOTE: Env var parsing replaces only the FIRST underscore after the prefix
		// with a dot separator. This means deeply nested keys like
		// harvester.opencode.session_dir cannot be set via NANO_BRAIN_HARVESTER_OPENCODE_SESSION_DIR.
		// Use the special env var mappings (e.g., OPENCODE_STORAGE_DIR) for these cases.

		_ = k.Set(key, value)
	}

	// Special non-prefixed env vars
	specialEnvVars := map[string]string{
		"VOYAGE_API_KEY":           "embedding.voyage_api_key",
		"DATABASE_URL":             "database.url",
		"OPENCODE_STORAGE_DIR":     "harvester.opencode.session_dir",
		"NANO_BRAIN_SUMMARIZE_API_KEY": "summarization.api_key",
	}
	for envVar, key := range specialEnvVars {
		if value, exists := os.LookupEnv(envVar); exists {
			_ = k.Set(key, value)
		}
	}

	// Unmarshal into Config struct with type conversion
	cfg := &Config{}
	if err := k.UnmarshalWithConf("", cfg, koanf.UnmarshalConf{
		Tag: "koanf",
	}); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := expandPaths(cfg); err != nil {
		return nil, err
	}

	// Validate configuration
	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// expandPaths expands "~/" prefixes in path-type config fields to the real home directory.
// os.MkdirAll and os.Open do not interpret tilde — it must be resolved explicitly.
func expandPaths(cfg *Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory for path expansion: %w", err)
	}
	expand := func(p string) string {
		if strings.HasPrefix(p, "~/") {
			return filepath.Join(home, p[2:])
		}
		return p
	}
	cfg.Summarization.OutputDir = expand(cfg.Summarization.OutputDir)
	return nil
}

// validate checks configuration validity.
func validate(cfg *Config) error {
	var errs []error

	// Validate Server
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		errs = append(errs, errors.New("server.port must be between 1 and 65535"))
	}

	// Validate Embedding
	if cfg.Embedding.Concurrency < 1 {
		errs = append(errs, errors.New("embedding.concurrency must be >= 1"))
	}

	// Validate Search
	if cfg.Search.RrfK < 1 {
		errs = append(errs, errors.New("search.rrf_k must be >= 1"))
	}
	if cfg.Search.RecencyWeight < 0 || cfg.Search.RecencyWeight > 1 {
		errs = append(errs, errors.New("search.recency_weight must be between 0 and 1"))
	}
	if cfg.Search.RecencyHalfLifeDays < 1 {
		errs = append(errs, errors.New("search.recency_half_life_days must be >= 1"))
	}
	if cfg.Search.Limit < 1 {
		errs = append(errs, errors.New("search.limit must be >= 1"))
	}

	// Validate Storage
	if cfg.Storage.MaxFileSize < 1 {
		errs = append(errs, errors.New("storage.max_file_size must be >= 1"))
	}
	if cfg.Storage.MaxSize < 1 {
		errs = append(errs, errors.New("storage.max_size must be >= 1"))
	}
	if cfg.Storage.MaxFileSize > cfg.Storage.MaxSize {
		errs = append(errs, errors.New("storage.max_file_size must not exceed storage.max_size"))
	}

	// Validate Intervals
	if cfg.Intervals.SessionPoll < 1 {
		errs = append(errs, errors.New("intervals.session_poll must be >= 1"))
	}

	// Validate Watcher
	if cfg.Watcher.DebounceMs < 1 {
		errs = append(errs, errors.New("watcher.debounce_ms must be >= 1"))
	}
	if cfg.Watcher.ReindexInterval < 1 {
		errs = append(errs, errors.New("watcher.reindex_interval must be >= 1"))
	}

	if cfg.Telemetry.RetentionDays < 1 {
		errs = append(errs, errors.New("telemetry.retention_days must be >= 1"))
	}

	if cfg.Logging.Level != "" {
		if _, err := zerolog.ParseLevel(cfg.Logging.Level); err != nil {
			errs = append(errs, fmt.Errorf("logging.level %q is not valid", cfg.Logging.Level))
		}
	}

	// Validate Summarization
	if cfg.Summarization.Enabled {
		if cfg.Summarization.ProviderURL == "" {
			errs = append(errs, errors.New("summarization.provider_url is required when summarization.enabled is true"))
		}
		if cfg.Summarization.Concurrency < 1 {
			errs = append(errs, errors.New("summarization.concurrency must be >= 1 when summarization.enabled is true"))
		}
	}

	if len(errs) > 0 {
		var msg strings.Builder
		msg.WriteString("configuration validation failed:")
		for _, err := range errs {
			msg.WriteString("\n  - ")
			msg.WriteString(err.Error())
		}
		return errors.New(msg.String())
	}

	return nil
}

func (w WatcherConfig) ResolveFilter(workspaceDir string) (excludePatterns, allowedExtensions []string) {
	excludePatterns = append(excludePatterns, w.ExcludePatterns...)
	allowedExtensions = append(allowedExtensions, w.AllowedExtensions...)

	if ws, ok := w.Workspaces[workspaceDir]; ok {
		excludePatterns = append(excludePatterns, ws.ExcludePatterns...)
		if len(ws.AllowedExtensions) > 0 {
			allowedExtensions = ws.AllowedExtensions
		}
	}

	return excludePatterns, allowedExtensions
}

func (w WatcherConfig) ResolveFilterForPath(collectionPath string) (excludePatterns, allowedExtensions []string) {
	excludePatterns = append(excludePatterns, w.ExcludePatterns...)
	allowedExtensions = append(allowedExtensions, w.AllowedExtensions...)

	best := ""
	for wsDir, wsCfg := range w.Workspaces {
		if strings.HasPrefix(collectionPath, wsDir) && len(wsDir) > len(best) {
			best = wsDir
			_ = wsCfg
		}
	}
	if best != "" {
		ws := w.Workspaces[best]
		excludePatterns = append(excludePatterns, ws.ExcludePatterns...)
		if len(ws.AllowedExtensions) > 0 {
			allowedExtensions = ws.AllowedExtensions
		}
	}

	return excludePatterns, allowedExtensions
}

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.nano-brain/config.yml"
	}
	return filepath.Join(home, ".nano-brain", "config.yml")
}
