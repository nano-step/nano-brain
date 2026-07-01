package mcp

import (
	"context"
	"net/http"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewStreamableHTTPHandler creates an http.Handler for MCP Streamable HTTP transport.
// Stateless mode is enabled so that clients do not need to track session IDs.
// Each tool call is self-contained (workspace hash passed per-request), so
// server-side session state provides no benefit for nano-brain.
func NewStreamableHTTPHandler(server *mcpsdk.Server) http.Handler {
	return mcpsdk.NewStreamableHTTPHandler(
		func(_ *http.Request) *mcpsdk.Server {
			return server
		},
		&mcpsdk.StreamableHTTPOptions{Stateless: true},
	)
}

// ctxKeyDefaultWorkspace is the private context key under which
// WrapStreamableHandler stores the per-connection default workspace value.
// Only this package reads it (requireWorkspace/requireRegisteredWorkspace in
// tools.go) per Go convention: the package that defines a context key is the
// package that reads it.
type ctxKeyDefaultWorkspace struct{}

// WrapStreamableHandler wraps an http.Handler (the Streamable HTTP MCP
// handler) with middleware that reads the `workspace` URL query parameter
// and injects it into the request context as a per-connection default.
//
// requireWorkspace/requireRegisteredWorkspace fall back to this value when a
// tool call omits the `workspace` argument, letting a `.mcp.json` entry bind
// a connection to a single default workspace via
// `http://.../mcp?workspace=<name-or-hash>`.
//
// This must wrap the handler BEFORE it is registered with echo.WrapHandler,
// not applied as an echo.MiddlewareFunc — the SDK reads req.Context()
// directly, and Echo's c.Set values on echo.Context never reach it.
//
// The raw query value is stored as-is (not resolved to a hash) so that
// resolution stays lazy inside requireWorkspace, avoiding an extra DB
// round-trip on every request, including ones that don't need a workspace
// (e.g. memory_status). An empty `workspace=` value is treated as absent.
//
// The literal "all" is also treated as absent (never stored as a default):
// per D-02, a connection-level default must never resolve calls to the
// cross-workspace "all" scope — that stays an explicit per-call opt-in only.
// Without this guard, requireWorkspace's own "all" special-case would apply
// to the fallback value too, silently turning every omitted-arg tool call on
// an `?workspace=all`-configured connection into a cross-workspace query.
func WrapStreamableHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v := r.URL.Query().Get("workspace"); v != "" && v != "all" {
			r = r.WithContext(context.WithValue(r.Context(), ctxKeyDefaultWorkspace{}, v))
		}
		next.ServeHTTP(w, r)
	})
}
