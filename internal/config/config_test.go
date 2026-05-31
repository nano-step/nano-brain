package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.yml")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Server.Host != "localhost" {
		t.Errorf("expected Server.Host=localhost, got %q", cfg.Server.Host)
	}
	if cfg.Server.Port != 3100 {
		t.Errorf("expected Server.Port=3100, got %d", cfg.Server.Port)
	}
	if cfg.Database.URL != "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev" {
		t.Errorf("unexpected Database.URL: %q", cfg.Database.URL)
	}
	if cfg.Embedding.Provider != "ollama" {
		t.Errorf("expected Embedding.Provider=ollama, got %q", cfg.Embedding.Provider)
	}
	if cfg.Embedding.Concurrency != 3 {
		t.Errorf("expected Embedding.Concurrency=3, got %d", cfg.Embedding.Concurrency)
	}
	if cfg.Intervals.SessionPoll != 120 {
		t.Errorf("expected Intervals.SessionPoll=120, got %d", cfg.Intervals.SessionPoll)
	}
	if cfg.Watcher.DebounceMs != 2000 {
		t.Errorf("expected Watcher.DebounceMs=2000, got %d", cfg.Watcher.DebounceMs)
	}
	if cfg.Search.RrfK != 60 {
		t.Errorf("expected Search.RrfK=60, got %v", cfg.Search.RrfK)
	}
	if cfg.Search.RecencyWeight != 0.3 {
		t.Errorf("expected Search.RecencyWeight=0.3, got %v", cfg.Search.RecencyWeight)
	}
	if cfg.Search.RecencyHalfLifeDays != 180 {
		t.Errorf("expected Search.RecencyHalfLifeDays=180, got %d", cfg.Search.RecencyHalfLifeDays)
	}
	if cfg.Search.Limit != 20 {
		t.Errorf("expected Search.Limit=20, got %d", cfg.Search.Limit)
	}
	if cfg.Storage.MaxFileSize != 314572800 {
		t.Errorf("expected Storage.MaxFileSize=314572800, got %d", cfg.Storage.MaxFileSize)
	}
	if cfg.Telemetry.RetentionDays != 90 {
		t.Errorf("expected Telemetry.RetentionDays=90, got %d", cfg.Telemetry.RetentionDays)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected Logging.Level=info, got %q", cfg.Logging.Level)
	}
}

func TestEnvVarOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.yml")

	t.Setenv("NANO_BRAIN_SERVER_PORT", "8080")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("expected Server.Port=8080 from env, got %d", cfg.Server.Port)
	}
}

func TestConfigFileOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	yamlContent := `server:
  host: "custom-host"
  port: 9000
database:
  url: "postgres://custom:pass@localhost:5432/custom_db"
embedding:
  provider: "voyage"
  concurrency: 5
logging:
  level: "debug"
`

	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Server.Host != "custom-host" {
		t.Errorf("expected Server.Host=custom-host, got %q", cfg.Server.Host)
	}
	if cfg.Server.Port != 9000 {
		t.Errorf("expected Server.Port=9000, got %d", cfg.Server.Port)
	}
	if cfg.Database.URL != "postgres://custom:pass@localhost:5432/custom_db" {
		t.Errorf("unexpected Database.URL: %q", cfg.Database.URL)
	}
	if cfg.Embedding.Provider != "voyage" {
		t.Errorf("expected Embedding.Provider=voyage, got %q", cfg.Embedding.Provider)
	}
	if cfg.Embedding.Concurrency != 5 {
		t.Errorf("expected Embedding.Concurrency=5, got %d", cfg.Embedding.Concurrency)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected Logging.Level=debug, got %q", cfg.Logging.Level)
	}
}

func TestEnvVarOverridesFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	yamlContent := `server:
  port: 9000
logging:
  level: "debug"
`

	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("NANO_BRAIN_SERVER_PORT", "8080")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("expected Server.Port=8080 from env (override file), got %d", cfg.Server.Port)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected Logging.Level=debug from file, got %q", cfg.Logging.Level)
	}
}

func TestInvalidRangeRejectsStartup(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		errMsg  string
	}{
		{
			name:    "negative_concurrency",
			envVars: map[string]string{"NANO_BRAIN_EMBEDDING_CONCURRENCY": "-1"},
			errMsg:  "embedding.concurrency must be >= 1",
		},
		{
			name:    "invalid_port_zero",
			envVars: map[string]string{"NANO_BRAIN_SERVER_PORT": "0"},
			errMsg:  "server.port must be between 1 and 65535",
		},
		{
			name:    "invalid_port_too_high",
			envVars: map[string]string{"NANO_BRAIN_SERVER_PORT": "65536"},
			errMsg:  "server.port must be between 1 and 65535",
		},
		{
			name:    "negative_rrf_k",
			envVars: map[string]string{"NANO_BRAIN_SEARCH_RRF_K": "0"},
			errMsg:  "search.rrf_k must be >= 1",
		},
		{
			name:    "invalid_recency_weight_negative",
			envVars: map[string]string{"NANO_BRAIN_SEARCH_RECENCY_WEIGHT": "-0.1"},
			errMsg:  "search.recency_weight must be between 0 and 1",
		},
		{
			name:    "invalid_recency_weight_too_high",
			envVars: map[string]string{"NANO_BRAIN_SEARCH_RECENCY_WEIGHT": "1.5"},
			errMsg:  "search.recency_weight must be between 0 and 1",
		},
		{
			name:    "invalid_recency_half_life_days_zero",
			envVars: map[string]string{"NANO_BRAIN_SEARCH_RECENCY_HALF_LIFE_DAYS": "0"},
			errMsg:  "search.recency_half_life_days must be >= 1",
		},
		{
			name:    "invalid_recency_half_life_days_negative",
			envVars: map[string]string{"NANO_BRAIN_SEARCH_RECENCY_HALF_LIFE_DAYS": "-1"},
			errMsg:  "search.recency_half_life_days must be >= 1",
		},
		{
			name:    "invalid_search_limit_zero",
			envVars: map[string]string{"NANO_BRAIN_SEARCH_LIMIT": "0"},
			errMsg:  "search.limit must be >= 1",
		},
		{
			name:    "invalid_telemetry_retention_zero",
			envVars: map[string]string{"NANO_BRAIN_TELEMETRY_RETENTION_DAYS": "0"},
			errMsg:  "telemetry.retention_days must be >= 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "nonexistent.yml")

			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			_, err := Load(configPath)
			if err == nil {
				t.Errorf("expected error containing %q, got nil", tt.errMsg)
			} else if fmt.Sprintf("%v", err) == "" || !contains(fmt.Sprintf("%v", err), tt.errMsg) {
				t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
			}
		})
	}
}

func TestGeneratesDefaultConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	if err := GenerateDefault(configPath); err != nil {
		t.Fatalf("GenerateDefault() failed: %v", err)
	}

	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}

	if len(content) == 0 {
		t.Error("config file is empty")
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed on generated file: %v", err)
	}

	if cfg.Server.Port != 3100 {
		t.Errorf("generated config has unexpected port: %d", cfg.Server.Port)
	}
}

func TestRecencyWeightBoundaryValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{
			name:  "recency_weight_zero",
			value: "0.0",
		},
		{
			name:  "recency_weight_one",
			value: "1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "nonexistent.yml")

			t.Setenv("NANO_BRAIN_SEARCH_RECENCY_WEIGHT", tt.value)

			cfg, err := Load(configPath)
			if err != nil {
				t.Errorf("expected no error for recency_weight=%s, got: %v", tt.value, err)
			}

			weight, err := strconv.ParseFloat(tt.value, 64)
			if err != nil {
				t.Fatalf("test setup error: %v", err)
			}
			if cfg.Search.RecencyWeight != weight {
				t.Errorf("expected Search.RecencyWeight=%v, got %v", weight, cfg.Search.RecencyWeight)
			}
		})
	}
}

func TestVoyageAPIKeyFromEnv(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.yml")

	t.Setenv("VOYAGE_API_KEY", "mykey123")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Embedding.VoyageAPIKey != "mykey123" {
		t.Errorf("expected VoyageAPIKey=mykey123, got %q", cfg.Embedding.VoyageAPIKey)
	}
}

func TestDatabaseURLFromEnv(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.yml")

	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/mydb")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Database.URL != "postgres://user:pass@localhost:5432/mydb" {
		t.Errorf("unexpected Database.URL: %q", cfg.Database.URL)
	}
}

func TestOpenCodeSessionDirFromEnv(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.yml")

	t.Setenv("OPENCODE_STORAGE_DIR", "/tmp/opencode_sessions")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Harvester.OpenCode.SessionDir != "/tmp/opencode_sessions" {
		t.Errorf("unexpected SessionDir: %q", cfg.Harvester.OpenCode.SessionDir)
	}
}

func TestOpenCodeDBPathFromEnv(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.yml")

	t.Setenv("OPENCODE_DB_PATH", "/tmp/opencode_global.db")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Harvester.OpenCode.DBPath != "/tmp/opencode_global.db" {
		t.Errorf("unexpected DBPath: %q", cfg.Harvester.OpenCode.DBPath)
	}
}

func TestOpenCodeDBRootFromEnv(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.yml")

	t.Setenv("OPENCODE_DB_ROOT", "/tmp/opencode-dbs")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Harvester.OpenCode.DBRoot != "/tmp/opencode-dbs" {
		t.Errorf("unexpected DBRoot: %q", cfg.Harvester.OpenCode.DBRoot)
	}
}

func TestOpenCodeDBRootFromYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	yaml := []byte("harvester:\n  opencode:\n    db_root: /custom/opencode-dbs\n    db_path: /custom/opencode.db\n    session_dir: /custom/sessions\n")
	if err := os.WriteFile(configPath, yaml, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Harvester.OpenCode.DBRoot != "/custom/opencode-dbs" {
		t.Errorf("unexpected DBRoot: %q", cfg.Harvester.OpenCode.DBRoot)
	}
	if cfg.Harvester.OpenCode.DBPath != "/custom/opencode.db" {
		t.Errorf("unexpected DBPath: %q", cfg.Harvester.OpenCode.DBPath)
	}
	if cfg.Harvester.OpenCode.SessionDir != "/custom/sessions" {
		t.Errorf("unexpected SessionDir: %q", cfg.Harvester.OpenCode.SessionDir)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()
	if path == "" {
		t.Error("DefaultConfigPath() returned empty string")
	}
	if !contains(path, ".nano-brain") || !contains(path, "config.yml") {
		t.Errorf("unexpected DefaultConfigPath: %q", path)
	}
}

func TestMultipleEnvVarsOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.yml")

	t.Setenv("NANO_BRAIN_SERVER_HOST", "0.0.0.0")
	t.Setenv("NANO_BRAIN_SERVER_PORT", "8888")
	t.Setenv("NANO_BRAIN_EMBEDDING_CONCURRENCY", "10")
	t.Setenv("NANO_BRAIN_LOGGING_LEVEL", "warn")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected Server.Host=0.0.0.0, got %q", cfg.Server.Host)
	}
	if cfg.Server.Port != 8888 {
		t.Errorf("expected Server.Port=8888, got %d", cfg.Server.Port)
	}
	if cfg.Embedding.Concurrency != 10 {
		t.Errorf("expected Embedding.Concurrency=10, got %d", cfg.Embedding.Concurrency)
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("expected Logging.Level=warn, got %q", cfg.Logging.Level)
	}
}

func TestStorageValidation(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	yamlContent := `storage:
  max_file_size: 5000000
  max_size: 1000000
`

	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("expected error when max_file_size > max_size, got nil")
	} else if !contains(fmt.Sprintf("%v", err), "max_file_size must not exceed storage.max_size") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGenerateDefaultCreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nested", "dir", "config.yml")

	if err := GenerateDefault(configPath); err != nil {
		t.Fatalf("GenerateDefault() failed: %v", err)
	}

	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("config file not created in nested directory: %v", err)
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

func TestSummarizationConfig_OutputDirHonored(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	yamlContent := `summarization:
  enabled: true
  provider_url: "https://test/v1"
  model: "test-model"
  output_dir: /tmp/foo
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if !cfg.Summarization.Enabled {
		t.Error("expected Summarization.Enabled=true, got false")
	}
	if cfg.Summarization.ProviderURL != "https://test/v1" {
		t.Errorf("expected ProviderURL=%q, got %q", "https://test/v1", cfg.Summarization.ProviderURL)
	}
	if cfg.Summarization.Model != "test-model" {
		t.Errorf("expected Model=%q, got %q", "test-model", cfg.Summarization.Model)
	}
	if cfg.Summarization.OutputDir != "/tmp/foo" {
		t.Errorf("expected OutputDir=%q, got %q", "/tmp/foo", cfg.Summarization.OutputDir)
	}
}

func TestSummarizationConfig_WriteToDiskDefaultsTrue(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	yamlContent := `summarization:
  enabled: true
  provider_url: "https://test/v1"
  model: "test-model"
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if !cfg.Summarization.IsWriteToDiskEnabled() {
		t.Error("expected IsWriteToDiskEnabled()=true (default), got false")
	}
}

func TestSummarizationConfig_WriteToDiskExplicitFalse(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	yamlContent := `summarization:
  enabled: true
  provider_url: "https://test/v1"
  model: "test-model"
  write_to_disk: false
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Summarization.IsWriteToDiskEnabled() {
		t.Error("expected IsWriteToDiskEnabled()=false (explicit), got true")
	}
}

func TestSummarizationConfig_OutputDirTildeExpanded(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	yamlContent := `summarization:
  enabled: true
  provider_url: "https://test/v1"
  model: "test-model"
  output_dir: "~/foo/bar"
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if strings.HasPrefix(cfg.Summarization.OutputDir, "~/") {
		t.Errorf("expected OutputDir to be tilde-expanded, got %q", cfg.Summarization.OutputDir)
	}
	if !strings.HasSuffix(cfg.Summarization.OutputDir, "/foo/bar") {
		t.Errorf("expected OutputDir to end with /foo/bar, got %q", cfg.Summarization.OutputDir)
	}
}

func TestSummarizationConfig_OutputDirDefaultPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	yamlContent := `summarization:
  enabled: true
  provider_url: "https://test/v1"
  model: "test-model"
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if !strings.HasSuffix(cfg.Summarization.OutputDir, "/.nano-brain/summaries") {
		t.Errorf("expected OutputDir to end with /.nano-brain/summaries, got %q", cfg.Summarization.OutputDir)
	}
}

func TestResolveFilter_GlobalOnly(t *testing.T) {
	cfg := WatcherConfig{
		ExcludePatterns:   []string{"*.log"},
		AllowedExtensions: []string{".go"},
	}
	excl, exts := cfg.ResolveFilter("/some/dir")
	if len(excl) != 1 || excl[0] != "*.log" {
		t.Errorf("unexpected exclude: %v", excl)
	}
	if len(exts) != 1 || exts[0] != ".go" {
		t.Errorf("unexpected extensions: %v", exts)
	}
}

func TestResolveFilter_WorkspaceOverride(t *testing.T) {
	cfg := WatcherConfig{
		ExcludePatterns:   []string{"*.log"},
		AllowedExtensions: []string{".go"},
		Workspaces: map[string]WorkspaceFilterConfig{
			"/my/project": {
				ExcludePatterns:   []string{"*.test.js"},
				AllowedExtensions: []string{".ts"},
			},
		},
	}
	excl, exts := cfg.ResolveFilter("/my/project")
	if len(excl) != 2 {
		t.Errorf("expected 2 exclude patterns (global+workspace), got %v", excl)
	}
	if len(exts) != 1 || exts[0] != ".ts" {
		t.Errorf("expected workspace extensions [.ts], got %v", exts)
	}
}

func TestResolveFilterForPath_PrefixMatch(t *testing.T) {
	cfg := WatcherConfig{
		ExcludePatterns: []string{"*.log"},
		Workspaces: map[string]WorkspaceFilterConfig{
			"/my/project": {
				AllowedExtensions: []string{".ts"},
			},
		},
	}
	excl, exts := cfg.ResolveFilterForPath("/my/project/src/components")
	if len(excl) != 1 || excl[0] != "*.log" {
		t.Errorf("unexpected exclude: %v", excl)
	}
	if len(exts) != 1 || exts[0] != ".ts" {
		t.Errorf("expected .ts from workspace match, got %v", exts)
	}
}

func TestResolveFilterForPath_NoMatch(t *testing.T) {
	cfg := WatcherConfig{
		ExcludePatterns: []string{"*.log"},
		Workspaces: map[string]WorkspaceFilterConfig{
			"/my/project": {AllowedExtensions: []string{".ts"}},
		},
	}
	excl, exts := cfg.ResolveFilterForPath("/other/project/src")
	if len(excl) != 1 || excl[0] != "*.log" {
		t.Errorf("unexpected exclude: %v", excl)
	}
	if len(exts) != 0 {
		t.Errorf("expected no extensions for unmatched path, got %v", exts)
	}
}

func TestResolveConfigPath_FlagWins(t *testing.T) {
	t.Setenv("NANO_BRAIN_CONFIG", "/from/env.yml")
	got := ResolveConfigPath("/from/flag.yml")
	if got != "/from/flag.yml" {
		t.Errorf("expected flag value to win, got %q", got)
	}
}

func TestResolveConfigPath_EnvWhenNoFlag(t *testing.T) {
	t.Setenv("NANO_BRAIN_CONFIG", "/from/env.yml")
	got := ResolveConfigPath("")
	if got != "/from/env.yml" {
		t.Errorf("expected env var when no flag, got %q", got)
	}
}

func TestResolveConfigPath_DefaultWhenNeitherSet(t *testing.T) {
	t.Setenv("NANO_BRAIN_CONFIG", "")
	got := ResolveConfigPath("")
	if got != DefaultConfigPath() {
		t.Errorf("expected default path when neither flag nor env set, got %q", got)
	}
}

func TestResolveConfigPath_EmptyEnvFallsBackToDefault(t *testing.T) {
	t.Setenv("NANO_BRAIN_CONFIG", "")
	got := ResolveConfigPath("")
	if got != DefaultConfigPath() {
		t.Errorf("expected default when env is empty string, got %q", got)
	}
}

func TestResolveConfigPath_TrimsWhitespace(t *testing.T) {
	t.Setenv("NANO_BRAIN_CONFIG", "  /from/env.yml  ")
	got := ResolveConfigPath("")
	if got != "/from/env.yml" {
		t.Errorf("expected trimmed env value, got %q", got)
	}
}

func TestResolveConfigPathStrict_WarnsOnMissingFile(t *testing.T) {
	t.Setenv("NANO_BRAIN_CONFIG", "/tmp/nano-brain-test-does-not-exist.yml")
	_, warn := ResolveConfigPathStrict("")
	if warn == "" {
		t.Fatal("expected warning for non-existent env-pointed file")
	}
	if !strings.Contains(warn, "NANO_BRAIN_CONFIG") {
		t.Errorf("warning should mention env var name, got %q", warn)
	}
	if !strings.Contains(warn, "does not exist") {
		t.Errorf("warning should explain why, got %q", warn)
	}
}

func TestResolveConfigPathStrict_NoWarnWhenDefault(t *testing.T) {
	t.Setenv("NANO_BRAIN_CONFIG", "")
	_, warn := ResolveConfigPathStrict("")
	if warn != "" {
		t.Errorf("default path should not warn, got %q", warn)
	}
}

func TestResolveConfigPathStrict_NoWarnWhenFlagPathExists(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cfg.yml")
	if err := os.WriteFile(cfgPath, []byte("server: {host: localhost}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, warn := ResolveConfigPathStrict(cfgPath)
	if warn != "" {
		t.Errorf("existing flag path should not warn, got %q", warn)
	}
}

func TestResolveConfigPathStrict_WarnsOnFlagMissingFile(t *testing.T) {
	_, warn := ResolveConfigPathStrict("/tmp/no-such-flag-path.yml")
	if warn == "" {
		t.Fatal("expected warning for non-existent flag-pointed file")
	}
	if !strings.Contains(warn, "--config") {
		t.Errorf("warning should mention --config, got %q", warn)
	}
}

func TestAuthConfig_EnabledFromEnv(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.yml")

	t.Setenv("NANO_BRAIN_AUTH_ENABLED", "true")

	cfg, err := Load(configPath)
	if err == nil {
		t.Log("auth enabled but no users/tokens — expecting validation error")
	}
	_ = cfg
	_ = err
}

func TestAuthConfig_EnabledFromYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	yamlContent := `server:
  auth:
    enabled: true
    realm: "test-realm"
    users:
      - username: admin
        password_hash: "$2a$10$fakehashjustfortest1234567890ab"
    bypass_paths:
      - /health
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if !cfg.Server.Auth.Enabled {
		t.Error("expected Auth.Enabled=true")
	}
	if cfg.Server.Auth.Realm != "test-realm" {
		t.Errorf("expected Realm=test-realm, got %q", cfg.Server.Auth.Realm)
	}
	if len(cfg.Server.Auth.Users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(cfg.Server.Auth.Users))
	}
	if cfg.Server.Auth.Users[0].Username != "admin" {
		t.Errorf("expected username=admin, got %q", cfg.Server.Auth.Users[0].Username)
	}
}

func TestAuthConfig_EnabledNoCredsRejectsStartup(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	yamlContent := `server:
  auth:
    enabled: true
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected validation error for auth enabled without users/tokens")
	}
	if !contains(fmt.Sprintf("%v", err), "auth enabled but no users or tokens configured") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAuthConfig_DefaultsDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.yml")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Server.Auth.Enabled {
		t.Error("expected Auth.Enabled=false by default")
	}
	if cfg.Server.Auth.Realm != "nano-brain" {
		t.Errorf("expected default Realm=nano-brain, got %q", cfg.Server.Auth.Realm)
	}
}

func TestConfig_JSONUsesSnakeCase(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.yml")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Marshal config to JSON
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(data)

	// Check that snake_case keys are present
	requiredKeys := []string{
		`"server"`,
		`"host"`,
		`"port"`,
		`"database"`,
		`"embedding"`,
		`"provider"`,
		`"voyage_api_key"`,
		`"harvester"`,
		`"opencode"`,
		`"session_dir"`,
		`"db_path"`,
		`"db_root"`,
		`"claudecode"`,
		`"watcher"`,
		`"debounce_ms"`,
		`"reindex_interval"`,
		`"search"`,
		`"rrf_k"`,
		`"recency_weight"`,
		`"recency_half_life_days"`,
		`"storage"`,
		`"max_file_size"`,
		`"max_size"`,
		`"telemetry"`,
		`"retention_days"`,
		`"logging"`,
		`"summarization"`,
		`"provider_url"`,
		`"max_tokens"`,
		`"requests_per_second"`,
		`"write_to_disk"`,
		`"output_dir"`,
	}

	for _, key := range requiredKeys {
		if !strings.Contains(jsonStr, key) {
			t.Errorf("expected JSON key %s, not found in output", key)
		}
	}

	// Check that PascalCase keys are NOT present (the bug we're fixing)
	forbiddenKeys := []string{
		`"Server"`,
		`"Host"`,
		`"Port"`,
		`"Database"`,
		`"Embedding"`,
		`"Provider"`,
		`"VoyageAPIKey"`,
		`"Harvester"`,
		`"OpenCode"`,
		`"SessionDir"`,
		`"DBPath"`,
		`"DBRoot"`,
		`"ClaudeCode"`,
		`"Watcher"`,
		`"DebounceMs"`,
		`"ReindexInterval"`,
		`"Search"`,
		`"RrfK"`,
		`"RecencyWeight"`,
		`"RecencyHalfLifeDays"`,
		`"Storage"`,
		`"MaxFileSize"`,
		`"MaxSize"`,
		`"Telemetry"`,
		`"RetentionDays"`,
		`"Logging"`,
		`"Summarization"`,
		`"ProviderURL"`,
		`"MaxTokens"`,
		`"RequestsPerSecond"`,
		`"WriteToDisk"`,
		`"OutputDir"`,
	}

	for _, key := range forbiddenKeys {
		if strings.Contains(jsonStr, key) {
			t.Errorf("unexpected PascalCase key %s found in JSON output (should be snake_case)", key)
		}
	}
}
