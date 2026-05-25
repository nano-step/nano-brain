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
	return handlers.NewHealth(&mockPool{}, zerolog.Nop(), "test", time.Now(), &mockQueue{}, getCfg, counter)
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
