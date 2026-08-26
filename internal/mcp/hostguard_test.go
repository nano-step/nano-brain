package mcp_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nano-brain/nano-brain/internal/mcp"
)

// TestHostGuard_AllowsDocumentedContainerHost pins the behavior that go-sdk
// v1.7.0's default DNS-rebinding protection broke: nano-brain binds to
// localhost, so the SDK's binary loopback check rejected every non-loopback
// Host — including host.docker.internal, which `nano-brain mcp-url` prints
// for itself inside a container. No test covered this, which is why a
// wire-breaking dependency upgrade passed build, vet, unit and integration.
func TestHostGuard_AllowsDocumentedContainerHost(t *testing.T) {
	tests := []struct {
		name string
		host string
		want int
	}{
		{"localhost", "localhost:3100", http.StatusOK},
		{"loopback v4", "127.0.0.1:3100", http.StatusOK},
		{"loopback v6", "[::1]:3100", http.StatusOK},
		{"docker host gateway", "host.docker.internal:3100", http.StatusOK},
		{"portless", "host.docker.internal", http.StatusOK},
		{"uppercase", "HOST.DOCKER.INTERNAL:3100", http.StatusOK},
		{"rebinding attacker", "evil.example.com", http.StatusForbidden},
		{"lan address", "192.168.1.50:3100", http.StatusForbidden},
		// HTTP/1.0 makes Host optional and such a request does reach this
		// handler — net/http only 400s the HTTP/1.1 case.
		{"absent host", "", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := mcp.HostGuard(nil, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			req.Host = tt.host
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Errorf("Host %q: got %d, want %d", tt.host, rec.Code, tt.want)
			}
		})
	}
}

// TestHostGuard_OnlyGuardsLoopbackConnections pins the gating that keeps
// VPS/reverse-proxy deployments working. The guard must fire for a connection
// accepted on loopback (the DNS-rebinding case: a browser resolves the
// attacker's hostname to 127.0.0.1 and connects there) and must NOT fire for
// one accepted on a public address, where the SDK never guarded either and
// where firing would 403 deployments that worked before the upgrade.
func TestHostGuard_OnlyGuardsLoopbackConnections(t *testing.T) {
	tests := []struct {
		name      string
		localAddr net.Addr
		want      int
	}{
		{"loopback v4 connection guards", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 3100}, http.StatusForbidden},
		{"loopback v6 connection guards", &net.TCPAddr{IP: net.ParseIP("::1"), Port: 3100}, http.StatusForbidden},
		{"public connection does not guard", &net.TCPAddr{IP: net.ParseIP("203.0.113.7"), Port: 3100}, http.StatusOK},
		{"lan connection does not guard", &net.TCPAddr{IP: net.ParseIP("192.168.1.10"), Port: 3100}, http.StatusOK},
		// Fail closed when the local address is unknowable.
		{"absent local addr guards", nil, http.StatusForbidden},
		{"unspecified v4 guards", &net.TCPAddr{IP: net.IPv4zero, Port: 3100}, http.StatusForbidden},
		{"unspecified v6 guards", &net.TCPAddr{IP: net.IPv6zero, Port: 3100}, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := mcp.HostGuard(nil, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			req.Host = "brain.example.com" // not in the default allowlist
			if tt.localAddr != nil {
				req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, tt.localAddr))
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Errorf("localAddr %v: got %d, want %d", tt.localAddr, rec.Code, tt.want)
			}
		})
	}
}

// TestHostGuard_MalformedAllowlist covers the config-level twin of the
// empty-Host bypass: a blank allowlist entry became the key "", and a
// port-only Host (":3199") normalizes to "" — so the two met and admitted a
// request. Both sides now share normalizeHost, and blanks are filtered out.
func TestHostGuard_MalformedAllowlist(t *testing.T) {
	tests := []struct {
		name    string
		allowed []string
		host    string
		want    int
	}{
		{"blank entry does not admit port-only host", []string{"localhost", ""}, ":3199", http.StatusForbidden},
		{"whitespace entry does not admit port-only host", []string{"localhost", "   "}, ":3199", http.StatusForbidden},
		{"port-only host rejected under defaults", nil, ":3199", http.StatusForbidden},
		// An all-blank allowlist must fall back to the defaults rather than
		// leaving a set whose only key is "".
		{"all-blank falls back to defaults", []string{"", "  "}, "host.docker.internal:3199", http.StatusOK},
		{"all-blank still rejects strangers", []string{""}, "evil.example.com", http.StatusForbidden},
		// A bracketed IPv6 entry copied from a URL must match the wire form.
		{"bracketed ipv6 config entry matches", []string{"[::1]"}, "[::1]:3100", http.StatusOK},
		{"unbracketed ipv6 config entry matches", []string{"::1"}, "[::1]:3100", http.StatusOK},
		{"entry with stray whitespace matches", []string{" localhost "}, "localhost:3100", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := mcp.HostGuard(tt.allowed, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			req.Host = tt.host
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Errorf("allowed=%q host=%q: got %d, want %d", tt.allowed, tt.host, rec.Code, tt.want)
			}
		})
	}
}

// TestHostGuard_ConfiguredAllowlistReplacesDefault covers the reverse-proxy
// and VPS deployments AuthConfig documents: an operator terminating on their
// own hostname must be able to add it.
func TestHostGuard_ConfiguredAllowlistReplacesDefault(t *testing.T) {
	h := mcp.HostGuard([]string{"brain.example.com", "localhost"},
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

	for host, want := range map[string]int{
		"brain.example.com:8080": http.StatusOK,
		"localhost:3100":         http.StatusOK,
		// Not in the configured list — an explicit allowlist replaces the
		// default rather than extending it, so the operator stays in control.
		"host.docker.internal:3100": http.StatusForbidden,
	} {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("Host %q: got %d, want %d", host, rec.Code, want)
		}
	}
}
