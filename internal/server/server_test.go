package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/server"
	"github.com/rs/zerolog"
)

type mockPool struct {
	pingErr error
}

func (m *mockPool) Ping(_ context.Context) error {
	return m.pingErr
}

func newTestServer(pool *mockPool) *server.Server {
	fullCfg := &config.Config{
		Server:    config.ServerConfig{Host: "127.0.0.1", Port: 3100},
		Embedding: config.EmbeddingConfig{Concurrency: 3},
		Search:    config.SearchConfig{RrfK: 60, RecencyWeight: 0.3, RecencyHalfLifeDays: 180, Limit: 20},
		Harvester: config.HarvesterConfig{},
		Intervals: config.IntervalsConfig{SessionPoll: 120},
		Watcher:   config.WatcherConfig{DebounceMs: 2000, ReindexInterval: 300},
		Storage:   config.StorageConfig{MaxFileSize: 314572800, MaxSize: 10737418240},
		Logging:   config.LoggingConfig{Level: "info"},
	}
	logger := zerolog.Nop()
	return server.New(fullCfg, "", pool, nil, nil, nil, nil, nil, nil, logger, "test-v1", 0, nil)
}

func TestHealthEndpointHealthyDB(t *testing.T) {
	s := newTestServer(&mockPool{})

	req := httptest.NewRequest(http.MethodGet, "http://localhost:3100/health", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body["status"])
	}
	if body["ready"] != true {
		t.Errorf("expected ready=true, got %v", body["ready"])
	}
	if body["version"] != "test-v1" {
		t.Errorf("expected version=test-v1, got %v", body["version"])
	}
}

func TestHealthEndpointUnreachableDB(t *testing.T) {
	s := newTestServer(&mockPool{pingErr: errors.New("connection refused")})

	req := httptest.NewRequest(http.MethodGet, "http://localhost:3100/health", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	if body["status"] != "degraded" {
		t.Errorf("expected status=degraded, got %v", body["status"])
	}
	if body["ready"] != false {
		t.Errorf("expected ready=false, got %v", body["ready"])
	}
	if body["reason"] != "database unreachable" {
		t.Errorf("expected reason set, got %v", body["reason"])
	}
}

func TestStatusEndpointShape(t *testing.T) {
	s := newTestServer(&mockPool{})

	req := httptest.NewRequest(http.MethodGet, "http://localhost:3100/api/status", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	requiredFields := []string{"pg_status", "migration_version", "embedding_queue_depth", "active_provider", "workspace_count", "queue_depth", "queue_capacity", "queue_status", "queue_pending", "harvester_status"}
	for _, field := range requiredFields {
		if _, ok := body[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}

	if body["pg_status"] != "healthy" {
		t.Errorf("expected pg_status=healthy, got %v", body["pg_status"])
	}
}

func TestVersionHeader(t *testing.T) {
	s := newTestServer(&mockPool{})

	req := httptest.NewRequest(http.MethodGet, "http://localhost:3100/health", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Nano-Brain-Version"); got != "test-v1" {
		t.Errorf("expected X-Nano-Brain-Version=test-v1, got %q", got)
	}
}

func TestHTTPErrorHandlerMapsErrWorkspaceRequired(t *testing.T) {
	s := newTestServer(&mockPool{})

	s.Echo().GET("/test-err", func(c echo.Context) error {
		return server.ErrWorkspaceRequired
	})

	req := httptest.NewRequest(http.MethodGet, "http://localhost:3100/test-err", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "workspace_required" {
		t.Errorf("expected error=workspace_required, got %q", body["error"])
	}
}

func TestRouteRegistration(t *testing.T) {
	s := newTestServer(&mockPool{})
	routes := s.Echo().Routes()

	paths := make(map[string]bool)
	for _, r := range routes {
		paths[r.Path] = true
	}

	for _, path := range []string{"/health", "/api/status", "/api/v1/init", "/api/v1/workspaces", "/api/v1/workspaces/resolve", "/api/v1/embed", "/api/v1/collections", "/api/v1/collections/:name"} {
		if !paths[path] {
			t.Errorf("route %s not registered", path)
		}
	}
}

// TestResolveWorkspaceRouteSkipsWorkspaceMiddleware confirms that
// POST /api/v1/workspaces/resolve is NOT subject to workspaceMiddleware:
// it must reject empty path with "path is required" (handler), not with
// "workspace_required" (middleware). See issue #316.
func TestResolveWorkspaceRouteSkipsWorkspaceMiddleware(t *testing.T) {
	s := newTestServer(&mockPool{})

	req := httptest.NewRequest(http.MethodPost, "http://localhost:3100/api/v1/workspaces/resolve", strings.NewReader(`{"path":""}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "workspace_required") {
		t.Errorf("workspace middleware leaked onto resolve route — body should mention 'path is required', got: %s", body)
	}
	if !strings.Contains(body, "path is required") {
		t.Errorf("expected 'path is required' from handler, got: %s", body)
	}
}

// The MCP endpoints must sit behind mcp.HostGuard: the SDK's own
// DNS-rebinding check is disabled, so the guard registered server-wide in
// registerMiddleware is the only protection. This pins the wiring, not just
// the guard function (#630).
func TestMCPRoutesRejectForeignHost(t *testing.T) {
	s := newTestServer(&mockPool{})
	const initialize = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`

	tests := []struct {
		name, method, path, host string
		wantForbidden            bool
	}{
		{"mcp post foreign host", http.MethodPost, "/mcp", "evil.example:3100", true},
		{"mcp get foreign host", http.MethodGet, "/mcp", "evil.example", true},
		{"sse post foreign host", http.MethodPost, "/sse", "evil.example:3100", true},
		{"sse get foreign host", http.MethodGet, "/sse", "evil.example", true},
		{"mcp post localhost", http.MethodPost, "/mcp", "localhost:3100", false},
		{"mcp post container host", http.MethodPost, "/mcp", "host.docker.internal:3100", false},
		{"sse post localhost", http.MethodPost, "/sse", "127.0.0.1:3100", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(initialize))
			req.Host = tt.host
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			rec := httptest.NewRecorder()
			s.ServeHTTP(rec, req)

			if got := rec.Code == http.StatusForbidden; got != tt.wantForbidden {
				t.Errorf("%s %s with Host %q: status %d, want forbidden=%v", tt.method, tt.path, tt.host, rec.Code, tt.wantForbidden)
			}
		})
	}
}

// A client that never finishes its request headers, or parks an idle
// keep-alive connection, must not hold a goroutine forever. ReadTimeout and
// WriteTimeout stay unset: /sse, /api/v1/events and long MCP calls stream
// (#673).
func TestServerTimeouts(t *testing.T) {
	srv := newGuardTestServer(t, config.AuthConfig{}).Echo().Server

	tests := []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		{"ReadHeaderTimeout", srv.ReadHeaderTimeout, 10 * time.Second},
		{"IdleTimeout", srv.IdleTimeout, 120 * time.Second},
		{"ReadTimeout", srv.ReadTimeout, 0},
		{"WriteTimeout", srv.WriteTimeout, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}
}
