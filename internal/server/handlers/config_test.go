package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/rs/zerolog"
)

func nopLogger() zerolog.Logger { return zerolog.Nop() }

func makeTestConfig() *config.Config {
	return &config.Config{
		Server:   config.ServerConfig{Host: "localhost", Port: 3100},
		Database: config.DatabaseConfig{URL: "postgres://secret@localhost/db"},
		Embedding: config.EmbeddingConfig{
			Provider:     "ollama",
			URL:          "http://localhost:11434",
			Model:        "nomic-embed-text",
			VoyageAPIKey: "sk-secret",
			Concurrency:  3,
		},
		Search:        config.SearchConfig{RrfK: 60, RecencyWeight: 0.3, RecencyHalfLifeDays: 180, Limit: 20},
		Watcher:       config.WatcherConfig{DebounceMs: 2000, ReindexInterval: 300},
		Storage:       config.StorageConfig{MaxFileSize: 300 * 1024 * 1024, MaxSize: 10 * 1024 * 1024 * 1024},
		Telemetry:     config.TelemetryConfig{RetentionDays: 90},
		Logging:       config.LoggingConfig{Level: "info"},
		Intervals:     config.IntervalsConfig{SessionPoll: 120},
		Summarization: config.SummarizationConfig{APIKey: "sk-sum-secret"},
	}
}

func TestGetConfig_SecretsRedacted(t *testing.T) {
	e := echo.New()
	h := handlers.GetConfig("/tmp/config.yml", makeTestConfig, nopLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}

	body := rec.Body.String()
	if strings.Contains(body, "postgres://secret") {
		t.Error("database URL should be redacted")
	}
	if strings.Contains(body, "sk-secret") {
		t.Error("voyage API key should be redacted")
	}
	if strings.Contains(body, "sk-sum-secret") {
		t.Error("summarization API key should be redacted")
	}
	if !strings.Contains(body, "redacted") {
		t.Error("expected redacted values in response")
	}
}

func TestGetConfig_IncludeSource(t *testing.T) {
	e := echo.New()
	h := handlers.GetConfig("/tmp/config.yml", makeTestConfig, nopLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config?include_source=true", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h(c); err != nil {
		t.Fatal(err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["source"] != "/tmp/config.yml" {
		t.Errorf("expected source in response, got %v", resp["source"])
	}
}

func TestPatchConfig_SecretRejected(t *testing.T) {
	e := echo.New()
	h := handlers.PatchConfig("/tmp/config.yml", makeTestConfig, func() {}, nopLogger())

	body := `{"path":"database.url","value":"postgres://new"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h(c)
	if err == nil {
		t.Fatal("expected error for secret field patch")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %v", err)
	}
}

func TestPatchConfig_ValidPatchPersists(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yml")

	initial := `server:
  host: localhost
  port: 3100
search:
  rrf_k: 60
  recency_weight: 0.3
  recency_half_life_days: 180
  limit: 20
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	reloaded := false
	e := echo.New()
	h := handlers.PatchConfig(cfgPath, makeTestConfig, func() { reloaded = true }, nopLogger())

	body := `{"path":"search.rrf_k","value":80}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if !reloaded {
		t.Error("reload was not triggered")
	}

	data, _ := os.ReadFile(cfgPath)
	if !strings.Contains(string(data), "80") {
		t.Error("expected patched value 80 in file")
	}
}

func TestPatchConfig_UnpatchableField(t *testing.T) {
	e := echo.New()
	h := handlers.PatchConfig("/tmp/config.yml", makeTestConfig, func() {}, nopLogger())

	body := `{"path":"harvester.opencode.session_dir","value":"/tmp"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h(c)
	if err == nil {
		t.Fatal("expected error for non-patchable field")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %v", err)
	}
}
