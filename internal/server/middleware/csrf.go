// Package middleware provides HTTP middleware for the nano-brain server.
package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
)

// loopbackHosts maps localhost variants to true for CSRF origin matching.
var loopbackHosts = map[string]bool{
	"localhost": true,
	"127.0.0.1": true,
	"::1":       true,
	"[::1]":     true,
}

// CSRF returns Echo middleware implementing a 7-step origin-check decision
// order. It is registered server-wide and protects every POST/PUT/PATCH/DELETE
// route (REST API, /mcp, /sse) from cross-origin
// browser-initiated requests while allowing CLI/curl/MCP (no Origin header).
//
// 7-step rules (first match wins):
//  1. X-Requested-With: nano-brain-ui → ALLOW
//  2. Both Origin and Referer absent → ALLOW (CLI/curl/MCP)
//  3. Origin: null → REJECT 403
//  4. Origin present + host matches boundAddr or the request's Host → ALLOW
//  5. Origin present + host mismatches both → REJECT 403
//  6. Origin absent + Referer present + host matches boundAddr or Host → ALLOW
//  7. Else → REJECT 403
//
// Matching the request's own Host header is what lets a same-origin page work
// when the daemon is bound to a wildcard address (0.0.0.0 in Docker), where
// boundAddr names no host a browser can be on. It is only sound because the
// Host guard (mcp.HostGuard, registered ahead of this middleware in
// registerMiddleware) has already rejected a Host outside
// server.allowed_hosts: a page on evil.example calling localhost has Origin
// evil.example ≠ Host localhost and is rejected here, and a DNS-rebinding
// page, whose Host and Origin are both evil.example, never gets this far.
// This middleware must not be registered without, or ahead of, that guard.
func CSRF(boundAddr string) echo.MiddlewareFunc {
	boundHost := extractHost(boundAddr)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			method := c.Request().Method
			if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch && method != http.MethodDelete {
				return next(c)
			}

			// Rule 1: X-Requested-With header
			if c.Request().Header.Get("X-Requested-With") == "nano-brain-ui" {
				return next(c)
			}

			origin := c.Request().Header.Get("Origin")
			referer := c.Request().Header.Get("Referer")

			// Rule 2: no Origin and no Referer → CLI/curl
			if origin == "" && referer == "" {
				return next(c)
			}

			// Rule 3: Origin: null → sandboxed iframe
			if origin == "null" {
				return echo.NewHTTPError(http.StatusForbidden, "CSRF: null origin rejected")
			}

			reqHost := extractHost(c.Request().Host)
			trusted := func(host string) bool {
				return hostsMatch(host, boundHost) || (reqHost != "" && hostsMatch(host, reqHost))
			}

			if origin != "" {
				// Rule 4 & 5: Origin host match
				if trusted(extractHost(origin)) {
					return next(c)
				}
				return echo.NewHTTPError(http.StatusForbidden, "CSRF: origin mismatch")
			}

			// Origin absent, Referer present
			if referer != "" {
				// Rule 6: Referer host match
				if trusted(extractHost(referer)) {
					return next(c)
				}
			}

			// Rule 7: default reject
			return echo.NewHTTPError(http.StatusForbidden, "CSRF: request rejected")
		}
	}
}

func extractHost(raw string) string {
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return u.Host
}

func hostsMatch(a, b string) bool {
	ah, _ := splitHostPort(a)
	bh, _ := splitHostPort(b)

	if isLoopback(ah) && isLoopback(bh) {
		return true
	}
	return strings.EqualFold(a, b)
}

func isLoopback(host string) bool {
	return loopbackHosts[strings.ToLower(host)]
}

func splitHostPort(hostport string) (host, port string) {
	if idx := strings.LastIndex(hostport, ":"); idx >= 0 {
		if hostport[0] == '[' {
			end := strings.Index(hostport, "]")
			if end >= 0 && end+1 < len(hostport) && hostport[end+1] == ':' {
				return hostport[:end+1], hostport[end+2:]
			}
			return hostport, ""
		}
		return hostport[:idx], hostport[idx+1:]
	}
	return hostport, ""
}
