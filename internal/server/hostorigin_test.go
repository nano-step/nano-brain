package server_test

import (
	"context"
	"database/sql"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/server"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// newGuardTestServer builds a server whose queries are backed by a closed
// *sql.DB: sql.Open never dials, and every query on a closed DB returns an
// error, so handlers that get past the middleware answer 500 instead of
// dereferencing the nil queries newTestServer wires. No database is contacted.
func newGuardTestServer(t *testing.T, auth config.AuthConfig) *server.Server {
	t.Helper()
	return newGuardTestServerOn(t, "127.0.0.1", auth)
}

// newGuardTestServerOn is newGuardTestServer with a chosen server.host. The
// server is never started, so the address is configuration only — nothing
// binds it.
func newGuardTestServerOn(t *testing.T, host string, auth config.AuthConfig) *server.Server {
	t.Helper()
	db, err := sql.Open("pgx", "postgres://unused.invalid/unused")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("db.Close: %v", err)
	}
	fullCfg := &config.Config{
		Server: config.ServerConfig{Host: host, Port: 3100, Auth: auth},
	}
	return server.New(fullCfg, "", &mockPool{}, nil, sqlc.New(db), nil, nil, nil, nil, zerolog.Nop(), "test-v1", 0, nil)
}

// route is one method+path pair exercised by the guard wiring tests below.
type route struct {
	method, path string
}

func (r route) String() string { return r.method + " " + r.path }

// The Host guard is server-wide middleware, not an MCP-only wrap: a
// DNS-rebinding page is same-origin with every route, so every route —
// including /health — must refuse a Host outside the allowlist (#652).
func TestAllRoutesRejectForeignHost(t *testing.T) {
	s := newGuardTestServer(t, config.AuthConfig{})

	routes := []route{
		{http.MethodGet, "/api/status"},
		{http.MethodGet, "/api/v1/config"},
		{http.MethodGet, "/api/v1/workspaces"},
		{http.MethodPost, "/api/v1/init"},
		{http.MethodPost, "/api/v1/config"},
		{http.MethodPost, "/api/reload-config"},
		{http.MethodPost, "/api/harvest"},
		{http.MethodGet, "/health"},
	}
	hosts := []struct {
		host          string
		wantForbidden bool
	}{
		{"evil.example:3100", true},
		{"evil.example", true},
		{"localhost:3100", false},
		{"127.0.0.1:3100", false},
		{"host.docker.internal:3100", false},
	}

	for _, rt := range routes {
		for _, h := range hosts {
			t.Run(rt.String()+" Host "+h.host, func(t *testing.T) {
				req := httptest.NewRequest(rt.method, rt.path, nil)
				req.Host = h.host
				rec := httptest.NewRecorder()
				s.ServeHTTP(rec, req)

				if got := rec.Code == http.StatusForbidden; got != h.wantForbidden {
					t.Errorf("%s with Host %q: status %d, want forbidden=%v", rt, h.host, rec.Code, h.wantForbidden)
				}
			})
		}
	}
}

// The CSRF origin check is server-wide too: every state-changing route, not
// only the write subgroup, must refuse a cross-origin browser request while
// still admitting clients that send no Origin (CLI, curl, MCP) and pages on
// a loopback origin (the dashboard dev server) (#652). The two routes that
// act without a body admit no other loopback port (#673).
func TestStateChangingRoutesRejectForeignOrigin(t *testing.T) {
	exactOriginOnly := map[string]bool{"/api/reload-config": true, "/api/harvest": true}
	s := newGuardTestServer(t, config.AuthConfig{})

	routes := []route{
		{http.MethodPost, "/api/v1/init"},
		{http.MethodPost, "/api/v1/config"},
		{http.MethodPost, "/api/v1/reset-workspace"},
		{http.MethodDelete, "/api/v1/workspaces/abc"},
		{http.MethodPost, "/api/v1/collections"},
		{http.MethodPut, "/api/v1/collections/abc"},
		{http.MethodDelete, "/api/v1/collections/abc"},
		{http.MethodDelete, "/api/v1/documents/abc"},
		{http.MethodPost, "/api/reload-config"},
		{http.MethodPost, "/api/harvest"},
		{http.MethodPost, "/mcp"},
		{http.MethodPost, "/sse"},
		// No route registers PATCH today; the check must still run ahead of
		// routing so one added later is covered from the start.
		{http.MethodPatch, "/api/v1/config"},
	}
	origins := []struct {
		name          string
		header, value string
		wantForbidden bool
	}{
		{"foreign origin", "Origin", "http://evil.example", true},
		{"null origin", "Origin", "null", true},
		{"foreign referer", "Referer", "http://evil.example/page", true},
		{"no origin", "", "", false},
		{"loopback origin other port", "Origin", "http://localhost:5173", false},
	}

	for _, rt := range routes {
		for _, o := range origins {
			t.Run(rt.String()+" "+o.name, func(t *testing.T) {
				// "{" is invalid JSON, so handlers that read a body stop at 400.
				req := httptest.NewRequest(rt.method, rt.path, strings.NewReader("{"))
				req.Host = "localhost:3100"
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Accept", "application/json, text/event-stream")
				if o.header != "" {
					req.Header.Set(o.header, o.value)
				}
				rec := httptest.NewRecorder()
				s.ServeHTTP(rec, req)

				want := o.wantForbidden || (o.name == "loopback origin other port" && exactOriginOnly[rt.path])
				if got := rec.Code == http.StatusForbidden; got != want {
					t.Errorf("%s with %s %q: status %d, want forbidden=%v", rt, o.header, o.value, rec.Code, want)
				}
			})
		}
	}
}

// On a non-loopback connection the Host guard is enforced only while auth is
// disabled. Docker binds 0.0.0.0 with auth off, so a rebinding browser lands
// on the container IP and must be refused; with auth on, the request is let
// through to the auth middleware, which answers 401 (#652).
func TestHostGuardOnNonLoopbackConnectionDependsOnAuth(t *testing.T) {
	tests := []struct {
		name string
		auth config.AuthConfig
		want int
	}{
		{"auth disabled guards", config.AuthConfig{}, http.StatusForbidden},
		{"auth enabled defers to auth", config.AuthConfig{Enabled: true, Tokens: []string{"test-token"}}, http.StatusUnauthorized},
	}
	containerAddr := &net.TCPAddr{IP: net.ParseIP("172.17.0.2"), Port: 3100}

	for _, tt := range tests {
		for _, rt := range []route{
			{http.MethodGet, "/api/v1/config"},
			{http.MethodPost, "/api/v1/init"},
			{http.MethodPost, "/mcp"},
		} {
			t.Run(tt.name+" "+rt.String(), func(t *testing.T) {
				s := newGuardTestServer(t, tt.auth)

				req := httptest.NewRequest(rt.method, rt.path, nil)
				req.Host = "evil.example:3100"
				req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, containerAddr))
				rec := httptest.NewRecorder()
				s.ServeHTTP(rec, req)

				if rec.Code != tt.want {
					t.Errorf("%s with foreign Host on %v: status %d, want %d", rt, containerAddr, rec.Code, tt.want)
				}
			})
		}
	}
}

// The CSRF check accepts an Origin that matches the request's own Host, so on
// its own it would admit a DNS-rebinding page (Host and Origin both the
// attacker's name). The Host guard ahead of it is what refuses that request;
// this pins the pair, on a wildcard bind as well as a loopback one (#652).
func TestRebindingSameOriginRejectedByChain(t *testing.T) {
	routes := []route{
		{http.MethodPost, "/api/v1/init"},
		{http.MethodPost, "/api/v1/config"},
		{http.MethodDelete, "/api/v1/workspaces/abc"},
		{http.MethodPost, "/api/reload-config"},
		{http.MethodPost, "/mcp"},
	}
	for _, bind := range []string{"127.0.0.1", "0.0.0.0"} {
		s := newGuardTestServerOn(t, bind, config.AuthConfig{})
		for _, rt := range routes {
			t.Run("bind "+bind+" "+rt.String(), func(t *testing.T) {
				req := httptest.NewRequest(rt.method, rt.path, strings.NewReader("{"))
				req.Host = "evil.example"
				req.Header.Set("Origin", "http://evil.example")
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				s.ServeHTTP(rec, req)

				if rec.Code != http.StatusForbidden {
					t.Errorf("%s with Host and Origin evil.example: status %d, want 403", rt, rec.Code)
				}
			})
		}
	}
}

// A same-origin page must keep working when the daemon is bound to a wildcard
// address (the Docker deployment), where the bound address names no host a
// browser can be on; a foreign Origin must still be refused there (#652).
func TestWildcardBindSameOriginAllowed(t *testing.T) {
	s := newGuardTestServerOn(t, "0.0.0.0", config.AuthConfig{})

	tests := []struct {
		name          string
		host, origin  string
		wantForbidden bool
	}{
		{"same origin", "localhost:3100", "http://localhost:3100", false},
		{"loopback origin other port", "localhost:3100", "http://localhost:5173", false},
		{"container host same origin", "host.docker.internal:3100", "http://host.docker.internal:3100", false},
		{"foreign origin", "localhost:3100", "http://evil.example", true},
	}
	for _, tt := range tests {
		for _, rt := range []route{
			{http.MethodPost, "/api/v1/init"},
			{http.MethodPost, "/api/v1/config"},
			{http.MethodPost, "/mcp"},
		} {
			t.Run(tt.name+" "+rt.String(), func(t *testing.T) {
				req := httptest.NewRequest(rt.method, rt.path, strings.NewReader("{"))
				req.Host = tt.host
				req.Header.Set("Origin", tt.origin)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Accept", "application/json, text/event-stream")
				rec := httptest.NewRecorder()
				s.ServeHTTP(rec, req)

				if got := rec.Code == http.StatusForbidden; got != tt.wantForbidden {
					t.Errorf("%s Host %q Origin %q: status %d, want forbidden=%v", rt, tt.host, tt.origin, rec.Code, tt.wantForbidden)
				}
			})
		}
	}
}

// The order is guard, then CSRF, then auth. Each case sends a request that
// two of them would reject and checks which one answered, so swapping any pair
// in registerMiddleware fails here.
func TestHostOriginMiddlewareOrder(t *testing.T) {
	authOn := config.AuthConfig{Enabled: true, Tokens: []string{"test-token"}}
	tests := []struct {
		name       string
		auth       config.AuthConfig
		host       string
		origin     string
		wantStatus int
		wantBody   string
	}{
		{"guard answers before CSRF", config.AuthConfig{}, "evil.example", "http://other.example", http.StatusForbidden, "server.allowed_hosts"},
		{"guard answers before auth", authOn, "evil.example", "", http.StatusForbidden, "server.allowed_hosts"},
		{"CSRF answers before auth", authOn, "localhost:3100", "http://evil.example", http.StatusForbidden, "CSRF"},
		{"auth answers last", authOn, "localhost:3100", "", http.StatusUnauthorized, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newGuardTestServer(t, tt.auth)
			req := httptest.NewRequest(http.MethodPost, "/api/reload-config", nil)
			req.Host = tt.host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rec := httptest.NewRecorder()
			s.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus || !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("status %d body %q, want %d containing %q", rec.Code, rec.Body.String(), tt.wantStatus, tt.wantBody)
			}
		})
	}
}

// /api/harvest and /api/reload-config take no body, so a page on another
// local port could trigger them through the loopback leniency of the CSRF
// check. They accept only the exact origin, and nothing a browser marks as
// cross-site or same-site; the CLI, which sends neither header, still gets
// through (#673).
func TestBodylessActionsRequireSameOrigin(t *testing.T) {
	s := newGuardTestServer(t, config.AuthConfig{})

	tests := []struct {
		name          string
		headers       map[string]string
		wantForbidden bool
	}{
		{"cli, no browser headers", nil, false},
		{"same origin", map[string]string{"Origin": "http://localhost:3100", "Sec-Fetch-Site": "same-origin"}, false},
		{"same origin, no fetch metadata", map[string]string{"Origin": "http://localhost:3100"}, false},
		{"user-initiated", map[string]string{"Sec-Fetch-Site": "none"}, false},
		{"other loopback port", map[string]string{"Origin": "http://localhost:8080"}, true},
		{"other loopback name", map[string]string{"Origin": "http://127.0.0.1:3100"}, true},
		{"cross-site fetch metadata", map[string]string{"Sec-Fetch-Site": "cross-site"}, true},
		{"same-site fetch metadata", map[string]string{"Sec-Fetch-Site": "same-site"}, true},
		{"ui header waives the check", map[string]string{"Sec-Fetch-Site": "cross-site", "X-Requested-With": "nano-brain-ui"}, false},
	}
	for _, path := range []string{"/api/harvest", "/api/reload-config"} {
		for _, tt := range tests {
			t.Run(path+" "+tt.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, path, nil)
				req.Host = "localhost:3100"
				for k, v := range tt.headers {
					req.Header.Set(k, v)
				}
				rec := httptest.NewRecorder()
				s.ServeHTTP(rec, req)

				if got := rec.Code == http.StatusForbidden; got != tt.wantForbidden {
					t.Errorf("POST %s with %v: status %d, want forbidden=%v", path, tt.headers, rec.Code, tt.wantForbidden)
				}
			})
		}
	}
}
