package mcp

import (
	"net"
	"net/http"
	"strings"
)

// defaultAllowedHosts is the Host-header allowlist applied when
// server.allowed_hosts is not configured.
//
// host.docker.internal is included because it is the access path nano-brain
// itself hands out: `nano-brain mcp-url` prints
// http://host.docker.internal:3100/mcp when it detects /.dockerenv, and the
// same URL is documented in AGENTS.md and the bundled skill.
var defaultAllowedHosts = []string{
	"localhost",
	"127.0.0.1",
	"::1",
	"host.docker.internal",
}

// HostGuard rejects requests whose Host header is not in allowed, replacing
// the SDK's built-in DNS-rebinding check.
//
// The SDK check (mcp.StreamableHTTPOptions.DisableLocalhostProtection) is
// binary: when the connection lands on a loopback address it rejects every
// non-loopback Host. nano-brain binds to localhost by default, so that check
// rejects host.docker.internal — breaking the container access path the
// daemon documents and prints for itself. The protection is still wanted
// though: DNS rebinding grants same-origin access, bypassing CORS entirely,
// and this daemon exposes memory_write, memory_delete, and the user's whole
// indexed codebase. So the SDK check is disabled and this allowlist is
// substituted rather than dropping the protection.
//
// An empty allowed slice falls back to defaultAllowedHosts; pass a configured
// server.allowed_hosts to extend it for reverse-proxy or VPS deployments,
// which AuthConfig explicitly supports.
// The guard fires only when the connection landed on a loopback local
// address, mirroring the SDK check it replaces. That is not a weakening: DNS
// rebinding is a browser attack, and a browser attacking this daemon resolves
// the attacker's hostname to 127.0.0.1 and connects there — so the local
// address IS loopback and the guard fires. A remote client reaching a
// 0.0.0.0-bound daemon on its public address lands on a non-loopback local
// address, where the SDK never guarded either. Firing unconditionally would
// 403 those VPS/reverse-proxy deployments, which AuthConfig explicitly
// supports and which worked before this upgrade.
func HostGuard(allowed []string, next http.Handler) http.Handler {
	set := buildHostSet(allowed)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if localAddrIsLoopback(r) && !hostAllowed(r.Host, set) {
			http.Error(w, "Forbidden: Host header "+r.Host+" is not in server.allowed_hosts", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// localAddrIsLoopback reports whether the connection was accepted on a
// loopback address. An unavailable or unparseable local address is treated as
// loopback so the guard fails closed.
func localAddrIsLoopback(r *http.Request) bool {
	addr, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok || addr == nil {
		return true
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		host = addr.String()
	}
	ip := net.ParseIP(strings.TrimSuffix(strings.TrimPrefix(host, "["), "]"))
	if ip == nil {
		return true
	}
	// An unspecified address (0.0.0.0, ::) is as unknowable as a missing one.
	// net/http reports the concrete accepted address rather than the wildcard,
	// so this should not arise — but guarding costs nothing and removes a case
	// that would otherwise have to be reasoned about.
	return ip.IsLoopback() || ip.IsUnspecified()
}

// hostAllowed reports whether host (a Host header value, with or without a
// port) is in set.
//
// An empty Host is rejected. net/http already 400s HTTP/1.1 requests that omit
// it, and HTTP/2 always maps :authority onto r.Host — but HTTP/1.0 makes Host
// optional, and such a request does reach this handler (verified against a
// running server: a raw HTTP/1.0 POST with no Host passed straight through an
// earlier revision of this guard). No real MCP client speaks HTTP/1.0, so
// rejecting costs nothing and removes the bypass.
func hostAllowed(host string, set map[string]struct{}) bool {
	h := normalizeHost(host)
	if h == "" {
		return false
	}
	_, ok := set[h]
	return ok
}

// normalizeHost reduces a Host header value or a configured allowlist entry to
// a bare lowercase hostname. Both sides must go through this: normalizing them
// differently is what let a configured "[::1]" never match an incoming
// "[::1]:3100", and what let a port-only Host (":3100", which normalizes to
// "") match an empty allowlist entry.
//
// Returns "" for anything that reduces to no hostname at all — port-only,
// blank, or whitespace — which callers treat as not-a-host rather than as a
// key to look up.
func normalizeHost(s string) string {
	h := strings.ToLower(strings.TrimSpace(s))
	if name, _, err := net.SplitHostPort(h); err == nil {
		h = name
	}
	// IPv6 literals arrive bracketed; "[::1]:3100" loses its brackets via
	// SplitHostPort, but a portless "[::1]" — or a config entry copied from a
	// URL — does not.
	h = strings.TrimSuffix(strings.TrimPrefix(h, "["), "]")
	return strings.TrimSpace(h)
}

// buildHostSet normalizes and de-blanks the allowlist. An allowlist that is
// empty, or contains nothing but blanks, falls back to defaultAllowedHosts so
// a malformed config degrades to safe-and-working rather than to a set whose
// only key is "" — which would reject every real host while admitting a
// port-only one.
func buildHostSet(allowed []string) map[string]struct{} {
	set := make(map[string]struct{}, len(allowed))
	for _, h := range allowed {
		if n := normalizeHost(h); n != "" {
			set[n] = struct{}{}
		}
	}
	if len(set) == 0 {
		for _, h := range defaultAllowedHosts {
			set[normalizeHost(h)] = struct{}{}
		}
	}
	return set
}
