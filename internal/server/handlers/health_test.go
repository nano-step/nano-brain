package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/rs/zerolog"
)

type mockPool struct{ pingErr error }

func (m *mockPool) Ping(_ context.Context) error { return m.pingErr }

type mockQueue struct{}

func (m *mockQueue) Depth() int           { return 0 }
func (m *mockQueue) Capacity() int        { return 0 }
func (m *mockQueue) Status() string       { return "idle" }
func (m *mockQueue) PendingCount() int64  { return 0 }

type mockCounter struct {
	count int64
	err   error
}

func (m *mockCounter) CountWorkspaces(_ context.Context) (int64, error) {
	return m.count, m.err
}

func newTestHealth(counter handlers.WorkspaceCounter) *handlers.Health {
	getCfg := func() (config.HarvesterConfig, config.IntervalsConfig) {
		return config.HarvesterConfig{}, config.IntervalsConfig{}
	}
	return handlers.NewHealth(&mockPool{}, zerolog.Nop(), "test", time.Now(), &mockQueue{}, getCfg, counter, config.EmbeddingConfig{Provider: "ollama"}, 0)
}

func decodeJSON(t *testing.T, r io.Reader) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

func TestStatusReturnsRealWorkspaceCount(t *testing.T) {
	h := newTestHealth(&mockCounter{count: 3})
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Status(c); err != nil {
		t.Fatalf("Status: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", rec.Code)
	}
	body := decodeJSON(t, rec.Body)
	if got, ok := body["workspace_count"].(float64); !ok || int(got) != 3 {
		t.Errorf("workspace_count = %v, want 3", body["workspace_count"])
	}
}

func TestHealthReturnsRealWorkspaceCount(t *testing.T) {
	h := newTestHealth(&mockCounter{count: 7})
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Health(c); err != nil {
		t.Fatalf("Health: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", rec.Code)
	}
	body := decodeJSON(t, rec.Body)
	if got, ok := body["workspace_count"].(float64); !ok || int(got) != 7 {
		t.Errorf("workspace_count = %v, want 7", body["workspace_count"])
	}
}

func TestStatusSoftFailsOnCountError(t *testing.T) {
	h := newTestHealth(&mockCounter{err: errors.New("db down")})
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Status(c); err != nil {
		t.Fatalf("Status: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200 (soft-fail)", rec.Code)
	}
	body := decodeJSON(t, rec.Body)
	got, ok := body["workspace_count"].(float64)
	if !ok || int(got) != 0 {
		t.Errorf("workspace_count = %v, want 0 on error", body["workspace_count"])
	}
}

func TestHealthSoftFailsOnCountError(t *testing.T) {
	h := newTestHealth(&mockCounter{err: errors.New("db down")})
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Health(c); err != nil {
		t.Fatalf("Health: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200 (soft-fail)", rec.Code)
	}
	body := decodeJSON(t, rec.Body)
	if v, present := body["workspace_count"]; present {
		got, ok := v.(float64)
		if !ok || int(got) != 0 {
			t.Errorf("workspace_count = %v, want 0 on error", v)
		}
	}
}

func openCodeStatus(t *testing.T, snap handlers.HarvestStatusSnapshot) map[string]interface{} {
	t.Helper()
	h := newTestHealth(nil)
	h.SetHarvestStatus(snap)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := h.Status(c); err != nil {
		t.Fatalf("Status: %v", err)
	}
	body := decodeJSON(t, rec.Body)
	hs, _ := body["harvester_status"].(map[string]interface{})
	oc, _ := hs["opencode"].(map[string]interface{})
	return oc
}

func TestStatusReturnsVersion(t *testing.T) {
	getCfg := func() (config.HarvesterConfig, config.IntervalsConfig) {
		return config.HarvesterConfig{}, config.IntervalsConfig{}
	}
	h := handlers.NewHealth(&mockPool{}, zerolog.Nop(), "v1.2.3", time.Now(), &mockQueue{}, getCfg, nil, config.EmbeddingConfig{}, 0)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Status(c); err != nil {
		t.Fatalf("Status: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", rec.Code)
	}
	body := decodeJSON(t, rec.Body)
	if got, ok := body["version"].(string); !ok || got != "v1.2.3" {
		t.Errorf("version = %v, want v1.2.3", body["version"])
	}
}

func TestStatusOpenCode_DBRootMode(t *testing.T) {
	oc := openCodeStatus(t, handlers.HarvestStatusSnapshot{
		Mode: "db_root", DBRoot: "/home/u/dbs", DBCount: 3,
	})
	if oc["mode"] != "db_root" {
		t.Errorf("mode = %v, want db_root", oc["mode"])
	}
	if oc["enabled"] != true {
		t.Errorf("enabled = %v, want true", oc["enabled"])
	}
	if int(oc["db_count"].(float64)) != 3 {
		t.Errorf("db_count = %v, want 3", oc["db_count"])
	}
}

func TestStatusOpenCode_DBPathMode(t *testing.T) {
	oc := openCodeStatus(t, handlers.HarvestStatusSnapshot{
		Mode: "db_path", DBPath: "/home/u/opencode.db", DBCount: 1,
	})
	if oc["mode"] != "db_path" {
		t.Errorf("mode = %v, want db_path", oc["mode"])
	}
	if oc["enabled"] != true {
		t.Errorf("enabled = %v, want true", oc["enabled"])
	}
}

func TestStatusOpenCode_SessionDirMode(t *testing.T) {
	oc := openCodeStatus(t, handlers.HarvestStatusSnapshot{
		Mode: "session_dir", SessionDir: "/home/u/.local/share/opencode/storage", DBCount: 1,
	})
	if oc["mode"] != "session_dir" {
		t.Errorf("mode = %v, want session_dir", oc["mode"])
	}
	if oc["enabled"] != true {
		t.Errorf("enabled = %v, want true", oc["enabled"])
	}
}

func TestStatusOpenCode_DisabledMode(t *testing.T) {
	oc := openCodeStatus(t, handlers.HarvestStatusSnapshot{Mode: "disabled"})
	if oc["mode"] != "disabled" {
		t.Errorf("mode = %v, want disabled", oc["mode"])
	}
	if oc["enabled"] != false {
		t.Errorf("enabled = %v, want false", oc["enabled"])
	}
}
