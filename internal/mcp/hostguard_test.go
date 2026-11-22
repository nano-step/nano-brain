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
			h := mcp.HostGuard(nil, false, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

// TestHostGuard_GuardCondition pins when the guard fires: on a connection
// accepted on loopback (the DNS-rebinding case on a host install), and on any
// connection while auth is disabled (the Docker case: the daemon binds 0.0.0.0
// in the container, so a browser rebinding to the published port arrives on
// the container IP). It must NOT fire for a non-loopback connection with auth
// enabled, where a rebinding page has no credentials and firing would 403 the
// VPS/reverse-proxy deployments reached under their own hostname.
func TestHostGuard_GuardCondition(t *testing.T) {
	tests := []struct {
		name        string
		localAddr   net.Addr
		authEnabled bool
		want        int
	}{
		{"auth on, loopback v4 connection guards", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 3100}, true, http.StatusForbidden},
		{"auth on, loopback v6 connection guards", &net.TCPAddr{IP: net.ParseIP("::1"), Port: 3100}, true, http.StatusForbidden},
		{"auth on, public connection does not guard", &net.TCPAddr{IP: net.ParseIP("203.0.113.7"), Port: 3100}, true, http.StatusOK},
		{"auth on, lan connection does not guard", &net.TCPAddr{IP: net.ParseIP("192.168.1.10"), Port: 3100}, true, http.StatusOK},
		// Fail closed when the local address is unknowable.
		{"auth on, absent local addr guards", nil, true, http.StatusForbidden},
		{"auth on, unspecified v4 guards", &net.TCPAddr{IP: net.IPv4zero, Port: 3100}, true, http.StatusForbidden},
		{"auth on, unspecified v6 guards", &net.TCPAddr{IP: net.IPv6zero, Port: 3100}, true, http.StatusForbidden},
		// Auth disabled: every reachable address is attackable.
		{"auth off, loopback connection guards", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 3100}, false, http.StatusForbidden},
		{"auth off, container connection guards", &net.TCPAddr{IP: net.ParseIP("172.17.0.2"), Port: 3100}, false, http.StatusForbidden},
		{"auth off, public connection guards", &net.TCPAddr{IP: net.ParseIP("203.0.113.7"), Port: 3100}, false, http.StatusForbidden},
		{"auth off, absent local addr guards", nil, false, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := mcp.HostGuard(nil, tt.authEnabled, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
				t.Errorf("localAddr %v authEnabled %v: got %d, want %d", tt.localAddr, tt.authEnabled, rec.Code, tt.want)
			}
		})
	}
}

// TestHostGuard_AuthDisabledAllowsListedHostOnAnyAddress covers the Docker
// access path: auth off, connection on the container IP, Host from the
// allowlist — the guard is enforced but must still admit the listed hosts.
func TestHostGuard_AuthDisabledAllowsListedHostOnAnyAddress(t *testing.T) {
	for _, host := range []string{"localhost:3100", "127.0.0.1:3100", "host.docker.internal:3100"} {
		t.Run(host, func(t *testing.T) {
			h := mcp.HostGuard(nil, false, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			req.Host = host
			req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey,
				&net.TCPAddr{IP: net.ParseIP("172.17.0.2"), Port: 3100}))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("Host %q: got %d, want 200", host, rec.Code)
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
			h := mcp.HostGuard(tt.allowed, false, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	h := mcp.HostGuard([]string{"brain.example.com", "localhost"}, false,
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
