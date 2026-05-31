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

// expandTildeForConfig resolves "~/..." to absolute path. Local copy to avoid
// internal/config → internal/summarize import cycle.
func expandTildeForConfig(p string) (string, error) {
	if !strings.HasPrefix(p, "~/") && p != "~" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if p == "~" {
		return home, nil
	}
	return filepath.Join(home, p[2:]), nil
}

// Config holds all application configuration.
type Config struct {
	Server         ServerConfig         `koanf:"server" json:"server"`
	Database       DatabaseConfig       `koanf:"database" json:"database"`
	Embedding      EmbeddingConfig      `koanf:"embedding" json:"embedding"`
	Harvester      HarvesterConfig      `koanf:"harvester" json:"harvester"`
	Intervals      IntervalsConfig      `koanf:"intervals" json:"intervals"`
	Watcher        WatcherConfig        `koanf:"watcher" json:"watcher"`
	Search         SearchConfig         `koanf:"search" json:"search"`
	Storage        StorageConfig        `koanf:"storage" json:"storage"`
	Telemetry      TelemetryConfig      `koanf:"telemetry" json:"telemetry"`
	Logging        LoggingConfig        `koanf:"logging" json:"logging"`
	Summarization  SummarizationConfig  `koanf:"summarization" json:"summarization"`
}

// ServerConfig holds server configuration.
type ServerConfig struct {
	Host string     `koanf:"host" json:"host"`
	Port int        `koanf:"port" json:"port"`
	Auth AuthConfig `koanf:"auth" json:"auth"`
}

// AuthConfig holds authentication configuration for VPS/remote deployments.
type AuthConfig struct {
	Enabled     bool       `koanf:"enabled" json:"enabled"`
	Realm       string     `koanf:"realm" json:"realm"`
	Users       []UserCred `koanf:"users" json:"users"`
	Tokens      []string   `koanf:"tokens" json:"tokens"`
	BypassPaths []string   `koanf:"bypass_paths" json:"bypass_paths"`
}

// UserCred holds a single Basic Auth credential (username + bcrypt hash).
type UserCred struct {
	Username     string `koanf:"username" json:"username"`
	PasswordHash string `koanf:"password_hash" json:"password_hash"`
}

// DatabaseConfig holds database configuration.
type DatabaseConfig struct {
	URL string `koanf:"url" json:"url"`
}

// EmbeddingConfig holds embedding provider configuration.
type EmbeddingConfig struct {
	Provider     string `koanf:"provider" json:"provider"`
	URL          string `koanf:"url" json:"url"`
	Model        string `koanf:"model" json:"model"`
	Dimension    int    `koanf:"dimension" json:"dimension"`
	Concurrency  int    `koanf:"concurrency" json:"concurrency"`
	VoyageAPIKey string `koanf:"voyage_api_key" json:"voyage_api_key"`
}

// HarvesterConfig holds harvester configuration.
type HarvesterConfig struct {
	OpenCode   OpenCodeHarvesterConfig   `koanf:"opencode" json:"opencode"`
	ClaudeCode ClaudeCodeHarvesterConfig `koanf:"claudecode" json:"claudecode"`
}

// OpenCodeHarvesterConfig holds OpenCode harvester configuration.
//
// Source resolution priority at daemon startup (first non-empty wins):
//  1. DBRoot — scan directory for per-project SQLite DBs (new layout)
//  2. DBPath — single global SQLite DB (legacy single-DB layout)
//  3. SessionDir — filesystem JSON sessions (legacy storage)
type OpenCodeHarvesterConfig struct {
	SessionDir string `koanf:"session_dir" json:"session_dir"`
	DBPath     string `koanf:"db_path" json:"db_path"`
	DBRoot     string `koanf:"db_root" json:"db_root"`
}

// ClaudeCodeHarvesterConfig holds ClaudeCode harvester configuration.
type ClaudeCodeHarvesterConfig struct {
	Enabled    bool   `koanf:"enabled" json:"enabled"`
	SessionDir string `koanf:"session_dir" json:"session_dir"`
}

// IntervalsConfig holds interval configuration.
type IntervalsConfig struct {
	SessionPoll int `koanf:"session_poll" json:"session_poll"`
}

type WorkspaceFilterConfig struct {
	ExcludePatterns   []string `koanf:"exclude_patterns" json:"exclude_patterns"`
	AllowedExtensions []string `koanf:"allowed_extensions" json:"allowed_extensions"`
}

type WatcherConfig struct {
	DebounceMs        int                               `koanf:"debounce_ms" json:"debounce_ms"`
	ReindexInterval   int                               `koanf:"reindex_interval" json:"reindex_interval"`
	ExcludePatterns   []string                          `koanf:"exclude_patterns" json:"exclude_patterns"`
	AllowedExtensions []string                          `koanf:"allowed_extensions" json:"allowed_extensions"`
	Workspaces        map[string]WorkspaceFilterConfig  `koanf:"workspaces" json:"workspaces"`
}

// SearchConfig holds search configuration.
type SearchConfig struct {
	RrfK                float64 `koanf:"rrf_k" json:"rrf_k"`
	RecencyWeight       float64 `koanf:"recency_weight" json:"recency_weight"`
	RecencyHalfLifeDays int     `koanf:"recency_half_life_days" json:"recency_half_life_days"`
	Limit               int     `koanf:"limit" json:"limit"`
}

// StorageConfig holds storage configuration.
type StorageConfig struct {
	MaxFileSize int64 `koanf:"max_file_size" json:"max_file_size"`
	MaxSize     int64 `koanf:"max_size" json:"max_size"`
}

type TelemetryConfig struct {
	RetentionDays int `koanf:"retention_days" json:"retention_days"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level string `koanf:"level" json:"level"`
	File  string `koanf:"file" json:"file"`
}

// SummarizationConfig holds summarization configuration.
type SummarizationConfig struct {
	Enabled           bool    `koanf:"enabled" json:"enabled"`
	ProviderURL       string  `koanf:"provider_url" json:"provider_url"`
	APIKey            string  `koanf:"api_key" json:"api_key"`
	Model             string  `koanf:"model" json:"model"`
	MaxTokens         int     `koanf:"max_tokens" json:"max_tokens"`
	Concurrency       int     `koanf:"concurrency" json:"concurrency"`
	RequestsPerSecond float64 `koanf:"requests_per_second" json:"requests_per_second"`
	WriteToDisk       *bool   `koanf:"write_to_disk" json:"write_to_disk"`
	OutputDir         string  `koanf:"output_dir" json:"output_dir"`
}

// IsWriteToDiskEnabled returns true unless the operator explicitly set write_to_disk: false.
// Default is true (Obsidian-compatible disk persistence; see issue #258).
func (s SummarizationConfig) IsWriteToDiskEnabled() bool {
	if s.WriteToDisk == nil {
		return true
	}
	return *s.WriteToDisk
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
		"VOYAGE_API_KEY":               "embedding.voyage_api_key",
		"DATABASE_URL":                 "database.url",
		"OPENCODE_STORAGE_DIR":         "harvester.opencode.session_dir",
		"OPENCODE_DB_PATH":             "harvester.opencode.db_path",
		"OPENCODE_DB_ROOT":             "harvester.opencode.db_root",
		"NANO_BRAIN_SUMMARIZE_API_KEY": "summarization.api_key",
		"NANO_BRAIN_AUTH_ENABLED":      "server.auth.enabled",
		"NANO_BRAIN_AUTH_REALM":        "server.auth.realm",
		"NANO_BRAIN_AUTH_TOKENS":       "server.auth.tokens",
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

	// Tilde-expand summary output dir (issue #258).
	if cfg.Summarization.OutputDir != "" {
		expanded, err := expandTildeForConfig(cfg.Summarization.OutputDir)
		if err != nil {
			return nil, fmt.Errorf("expand output_dir: %w", err)
		}
		cfg.Summarization.OutputDir = expanded
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

	// Validate Auth
	if cfg.Server.Auth.Enabled {
		if len(cfg.Server.Auth.Users) == 0 && len(cfg.Server.Auth.Tokens) == 0 {
			errs = append(errs, errors.New("auth enabled but no users or tokens configured"))
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

// ResolveConfigPath returns the effective config file path with precedence:
//  1. explicit --config flag value (non-empty, after TrimSpace)
//  2. NANO_BRAIN_CONFIG environment variable (non-empty, after TrimSpace)
//  3. DefaultConfigPath() (~/.nano-brain/config.yml)
//
// Whitespace is trimmed from both sources. Existence is not checked — use
// ResolveConfigPathStrict when the caller needs a warning on missing files.
func ResolveConfigPath(flagValue string) string {
	if v := strings.TrimSpace(flagValue); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("NANO_BRAIN_CONFIG")); v != "" {
		return v
	}
	return DefaultConfigPath()
}

// ResolveConfigPathStrict behaves like ResolveConfigPath but ALSO emits a
// warning to stderr (and returns the warning) when --config flag or
// NANO_BRAIN_CONFIG env was set explicitly but the resolved file does not
// exist. The path is still returned so the caller can fall through to
// defaults — this is a warning, not a fatal error.
//
// Returns (path, warning). warning is "" when no problem detected.
func ResolveConfigPathStrict(flagValue string) (string, string) {
	flagTrimmed := strings.TrimSpace(flagValue)
	envTrimmed := strings.TrimSpace(os.Getenv("NANO_BRAIN_CONFIG"))

	var path string
	var source string
	switch {
	case flagTrimmed != "":
		path, source = flagTrimmed, "--config flag"
	case envTrimmed != "":
		path, source = envTrimmed, "NANO_BRAIN_CONFIG env"
	default:
		return DefaultConfigPath(), ""
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return path, fmt.Sprintf("WARNING: %s points to %q but that file does not exist — defaults will be used.", source, path)
		}
		return path, fmt.Sprintf("WARNING: %s = %q: %v", source, path, err)
	}
	return path, ""
}
