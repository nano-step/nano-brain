package config

import (
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
