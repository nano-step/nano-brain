# Phase 9: MCP workspace config binding - Research

**Researched:** 2026-07-01
**Domain:** Go HTTP middleware over an Echo-wrapped third-party MCP SDK handler; MCP tool schema design
**Confidence:** HIGH

## Summary

This phase is small in surface area but has one integration subtlety that CONTEXT.md's discretion note flagged and this research resolves definitively: **the codebase's established middleware convention is `echo.MiddlewareFunc`, but the MCP streamable handler is registered via `echo.WrapHandler(streamableHandler)` at `internal/server/routes.go:137-139`** — meaning the new middleware cannot be an `echo.MiddlewareFunc` in the conventional sense if it needs its injected value to survive into the SDK's own `context.Context`. Echo's `c.Set(key, val)` stores values on `echo.Context`, not on the underlying `*http.Request`'s `context.Context` — and the SDK reads `req.Context()` directly (verified against vendored source, see below), so `c.Set` alone would not reach `requireWorkspace`. The correct implementation is a plain `func(http.Handler) http.Handler` that wraps `streamableHandler` *before* `echo.WrapHandler()` sees it, mutating `req = req.WithContext(...)` directly. This can still be written in a style consistent with the codebase (small closures, same-file colocation) but must not follow the `echo.MiddlewareFunc` pattern used by `workspaceMiddleware`/`csrfMW`/`contentTypeMiddleware`.

All 7 locked decisions (D-01 through D-07) in CONTEXT.md hold up under this research and require no changes. The SDK's `req.Context()` extension point is confirmed by direct inspection of the vendored `go-sdk@v0.8.0` source (not just CONTEXT.md's earlier citation — independently re-verified in this session). The exact set of tools requiring the required-fields edit is enumerated precisely below: **13 of 17** `memory_*` tools list `"workspace"` as required (CONTEXT.md's phase description estimated "~18" from memory — this count needed correction). No existing test in `internal/mcp/*_test.go` exercises `requireWorkspace`/`requireRegisteredWorkspace`/`streamable.go` directly by name, so new tests will not duplicate existing coverage, but there IS a directly relevant existing test file (`internal/mcp/tools_security_test.go`) whose two `TestMemoryWrite_*` / `TestMemoryUpdate_RejectsUnregisteredWorkspace` tests exercise the exact `requireRegisteredWorkspace` path this phase touches — the planner should extend that file rather than create a new one, to keep the MCP-transport-level Postgres test harness (`setupMCPSecTestPG`/`setupMCPSecClient`) in one place.

**Primary recommendation:** Implement the middleware as a standalone `func(http.Handler) http.Handler` in `internal/mcp/streamable.go` (colocated with `NewStreamableHTTPHandler`), applied by wrapping `streamableHandler` in `routes.go` before the `echo.WrapHandler(...)` call — not as an `echo.MiddlewareFunc`. Reuse `storage.ResolveWorkspaceParam` unchanged. No new context-key naming convention exists elsewhere in the codebase to match, so `type ctxKeyDefaultWorkspace struct{}` (or equally terse equivalent) is a clean, unconflicting choice.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| URL query-param extraction (`?workspace=`) | API / Backend (HTTP middleware) | — | Query params are parsed from the raw `*http.Request` before any MCP/tool dispatch; this is server-side HTTP handling, not client or DB concern |
| Context value injection for per-connection default | API / Backend (HTTP middleware) | — | `context.Context` propagation across the SDK's `server.Connect(req.Context(), ...)` boundary is a backend-only mechanism; no client-visible state |
| Workspace resolution (name/hash → hash) | API / Backend (`storage.ResolveWorkspaceParam`) | Database / Storage (via `sqlc` queries) | Existing resolution logic already lives in `internal/storage`; this phase adds a second *caller* of it, not new resolution logic |
| Fallback precedence (arg > connection-default > error) | API / Backend (`requireWorkspace`/`requireRegisteredWorkspace`) | — | Precedence is a per-tool-call decision made inside the MCP tool handler layer, which already owns workspace validation today |
| Tool schema required-fields (removing `"workspace"`) | API / Backend (`toolSchema(...)` calls in `internal/mcp/tools.go`) | — | Schema is served to the MCP client (agent) as part of `tools/list`; it is metadata about the API contract, owned by the backend that defines the tools |
| `.mcp.json` / client config documentation | Documentation (docs/, README.md) | — | Config surface facing the human operator who edits `.mcp.json`, not runtime code |

## Standard Stack

No new dependencies. This phase is a pure modification of existing Go stdlib (`net/http`, `context`) and the already-vendored `github.com/modelcontextprotocol/go-sdk v0.8.0`.

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `net/http` (stdlib) | go1.x | Middleware wrapping, query param parsing | Already the transport layer under Echo; no new abstraction needed for a single wrap |
| `context` (stdlib) | go1.x | Carrying the default-workspace value across the SDK boundary | This is literally the SDK's documented, sanctioned extension mechanism — no alternative exists |
| `github.com/modelcontextprotocol/go-sdk` | v0.8.0 [VERIFIED: go.mod] | MCP protocol implementation, `NewStreamableHTTPHandler` | Already pinned in `go.mod`; version confirmed via `grep go-sdk go.mod` in this session |
| `github.com/labstack/echo/v4` | (existing, see go.mod) | HTTP router already used for the whole server | Pre-existing; the new middleware wraps the `http.Handler` supplied *to* `echo.WrapHandler`, not an Echo route group |

### Supporting
None — no new packages needed.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Plain `func(http.Handler) http.Handler` wrapper | `echo.MiddlewareFunc` registered on an Echo group covering `/mcp` | Rejected: `c.Set()` does not propagate into `req.Context()`, which is what the SDK actually reads via `req.Context()` at `streamable.go:297` in the vendored SDK — an Echo-native middleware would silently fail to reach the tool handlers. Only viable if the middleware also called `c.SetRequest(c.Request().WithContext(...))`, which is possible but adds an unnecessary layer of indirection versus wrapping before `echo.WrapHandler`. |

**Installation:** No installation needed — no new packages.

**Version verification:**
```bash
grep go-sdk go.mod
# github.com/modelcontextprotocol/go-sdk v0.8.0
```
Confirmed present and already vendored in the local module cache at `/Users/tamlh/go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v0.8.0/` [VERIFIED: local module cache, `go env GOMODCACHE`].

## Package Legitimacy Audit

Not applicable — this phase installs no new packages. Existing pinned dependency (`go-sdk v0.8.0`) is already in use elsewhere in the codebase and out of scope for a legitimacy re-check.

## Architecture Patterns

### System Architecture Diagram

```
   MCP client (.mcp.json)
   url: "http://localhost:3100/mcp?workspace=<name-or-hash>"
            │
            │ HTTP request (every call resends the same configured URL —
            │ stateless transport, no session storage needed)
            ▼
  ┌─────────────────────────────────────────────────────────┐
  │ internal/server/routes.go                                │
  │   s.echo.{GET,POST,DELETE}("/mcp",                        │
  │       echo.WrapHandler( defaultWorkspaceMiddleware(       │  <-- NEW: wraps
  │           streamableHandler ) ) )                         │      before WrapHandler
  └───────────────────┬───────────────────────────────────────┘
                       │ req *http.Request (query string intact)
                       ▼
  ┌─────────────────────────────────────────────────────────┐
  │ internal/mcp/streamable.go                                │
  │   defaultWorkspaceMiddleware(next http.Handler) http.Handler│ <-- NEW
  │     - r.URL.Query().Get("workspace")                       │
  │     - ctx := context.WithValue(r.Context(), ctxKey{}, v)   │
  │     - next.ServeHTTP(w, r.WithContext(ctx))                │
  └───────────────────┬───────────────────────────────────────┘
                       │ req.Context() now carries default workspace
                       ▼
  ┌─────────────────────────────────────────────────────────┐
  │ go-sdk StreamableHTTPHandler.ServeHTTP                    │
  │   server.Connect(req.Context(), transport, opts)  <-- SDK's│
  │   sanctioned extension point (verified: streamable.go:297) │
  └───────────────────┬───────────────────────────────────────┘
                       │ ctx flows through jsonrpc2 dispatch → ServerSession.handle(ctx, ...)
                       ▼
  ┌─────────────────────────────────────────────────────────┐
  │ internal/mcp/tools.go                                     │
  │   tool handler func(ctx context.Context, req *CallToolRequest)│
  │     a.requireWorkspace(ctx, args)                          │  <-- MODIFIED
  │       1. explicit args["workspace"] if present  (D-03)     │
  │       2. else ctx.Value(ctxKey{}) if present    (NEW)      │
  │       3. else -> "workspace is required" error  (D-04, unchanged)│
  │     storage.ResolveWorkspaceParam(ctx, q, input)  (reused, unchanged)│
  └─────────────────────────────────────────────────────────┘
```

### Recommended Project Structure

No new files needed — this phase modifies existing files in place:

```
internal/mcp/
├── streamable.go     # ADD: context key type, defaultWorkspaceMiddleware func
├── tools.go           # MODIFY: requireWorkspace, requireRegisteredWorkspace,
│                       #   and 13 toolSchema(...) required-fields lists
└── flowchart.go        # MODIFY: 1 toolSchema(...) required-fields list (memory_flowchart)

internal/server/
└── routes.go           # MODIFY: wrap streamableHandler with the new middleware
                          #   before echo.WrapHandler(...) at lines 137-139

docs/
├── SETUP_AGENT.md      # MODIFY: add ?workspace= example near "Step 9 — Configure your MCP client"
docs/reference-readme.md # MODIFY: add ?workspace= example near "MCP Configuration" section
README.md                # MODIFY: add ?workspace= example near "Add to your MCP client config"
```

### Pattern 1: Plain-http.Handler middleware wrapping an Echo-registered third-party handler

**What:** When a third-party `http.Handler` (here, the MCP SDK's `StreamableHTTPHandler`) is registered via `echo.WrapHandler(h)`, any middleware that needs to mutate the request *before* that handler sees it — in a way that must be visible through `req.Context()` — must wrap `h` directly as `http.Handler`, not be registered as `echo.MiddlewareFunc` on the route/group.

**When to use:** Any time a vendored SDK's `http.Handler` reads `req.Context()` internally and you need to inject a value before it runs, and that handler sits behind `echo.WrapHandler`.

**Example:**
```go
// Source: pattern derived from internal/server/routes.go:131-139 (existing wiring)
// and internal/mcp/streamable.go:9-20 (existing handler factory) — no direct
// upstream example needed, this is a standard net/http middleware shape.

type ctxKeyDefaultWorkspace struct{}

// defaultWorkspaceMiddleware reads the `workspace` URL query parameter and
// injects it into the request context so requireWorkspace/requireRegisteredWorkspace
// can fall back to it when a tool call omits the workspace argument.
func defaultWorkspaceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v := r.URL.Query().Get("workspace"); v != "" {
			r = r.WithContext(context.WithValue(r.Context(), ctxKeyDefaultWorkspace{}, v))
		}
		next.ServeHTTP(w, r)
	})
}
```

Wiring change in `routes.go` (illustrative — exact line numbers will shift slightly once applied):
```go
// Source: internal/server/routes.go:131-139 (current)
sseHandler := mcp.NewSSEHandler(s.mcpServer)
streamableHandler := mcp.NewStreamableHTTPHandler(s.mcpServer)

s.echo.GET("/sse", echo.WrapHandler(sseHandler))
s.echo.POST("/sse", echo.WrapHandler(sseHandler))

// NEW: wrap streamableHandler BEFORE echo.WrapHandler, not after
wrappedStreamable := mcp.DefaultWorkspaceMiddleware(streamableHandler) // exported if routes.go needs it from another package
s.echo.GET("/mcp", echo.WrapHandler(wrappedStreamable))
s.echo.POST("/mcp", echo.WrapHandler(wrappedStreamable))
s.echo.DELETE("/mcp", echo.WrapHandler(wrappedStreamable))
```

Note: `defaultWorkspaceMiddleware` must be exported (`DefaultWorkspaceMiddleware`) if `routes.go` (package `server`) is to call it, since it currently lives in package `mcp`. Alternatively, keep it unexported and add a one-line exported wrapper function in `internal/mcp` (e.g. `WrapStreamableHandler(h http.Handler) http.Handler`) that `routes.go` calls — this keeps the context-key type itself unexported/private to package `mcp`, matching D-`Claude's Discretion` guidance ("unexported struct-type key per Go convention"). **Recommended: export only a constructor-style wrapper function, keep the context key type unexported.**

### Pattern 2: Context-value fallback inside existing validation function

**What:** `requireWorkspace` gains a second lookup path, tried only when the explicit arg is empty.

**Example:**
```go
// Source: internal/mcp/tools.go:149-162 (current, to be modified)
func (a *Adapter) requireWorkspace(ctx context.Context, args map[string]any) (string, *mcpsdk.CallToolResult) {
	input := argString(args, "workspace")
	if input == "" {
		if v, ok := ctx.Value(ctxKeyDefaultWorkspace{}).(string); ok && v != "" {
			input = v
		}
	}
	if input == "" {
		return "", errResult("workspace is required")
	}
	if input == "all" {
		return "all", nil
	}
	hash, err := storage.ResolveWorkspaceParam(ctx, a.queries, input)
	if err != nil {
		return "", errResult(err.Error())
	}
	return hash, nil
}
```
`requireRegisteredWorkspace` (`tools.go:169-190`) calls `a.requireWorkspace(ctx, args)` internally already (`tools.go:177`), so **no separate change is needed there** beyond the fact that its `args["workspace"]` empty-check at the top (`tools.go:171-173`) must also fall through to context — i.e. that early check needs the identical `ctx.Value` fallback added, or it must be restructured to defer entirely to the inner `requireWorkspace` call for the empty-check. Recommend: **remove the duplicate empty-check in `requireRegisteredWorkspace` (lines 171-173) entirely** and let the inner `a.requireWorkspace` call be the single source of truth for the fallback + empty-check, since it already special-cases `"all"` differently — `requireRegisteredWorkspace` needs its own `"all"`-rejection check to run *before* delegating (already does, at line 174-176), so the structure becomes: check `"all"` explicitly first using the raw arg (unaffected by context fallback, since a context default is never `"all"` per D-02), then delegate straight to `a.requireWorkspace`.

### Anti-Patterns to Avoid
- **Registering the new logic as `echo.MiddlewareFunc` on an Echo group:** Values set via `c.Set()` live on `echo.Context`, which is discarded once `echo.WrapHandler`'s inner `http.HandlerFunc` runs — the SDK never sees an `echo.Context`, only the raw `*http.Request`. This would silently make the feature inert (query param parsed but never reaching tool handlers) without any test failure unless a test specifically exercises the full HTTP-to-tool-call path.
- **Resolving the workspace hash eagerly inside the middleware and storing the resolved hash in context:** Possible (CONTEXT.md leaves this as Claude's Discretion) but adds a DB round-trip on every single MCP request even for tool calls that don't need it (e.g., `memory_status`), and duplicates error-handling logic that `storage.ResolveWorkspaceParam` already owns inside the tool-handler layer. Storing the **raw string** and resolving lazily inside `requireWorkspace` (which already happens once per tool call that needs a workspace) is simpler and keeps the diff smaller — this is the recommended choice for "Claude's Discretion" item 2.
- **Duplicating the empty-check logic between `requireWorkspace` and `requireRegisteredWorkspace`:** As found above, `requireRegisteredWorkspace`'s own `input == ""` check at `tools.go:171` would bypass the context fallback for write tools if not also updated or removed — the CONTEXT.md's canonical_refs entry for `tools.go:149-180` needs both functions touched, not just the composed call.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Workspace name/hash resolution | A parallel `resolveDefaultWorkspace(...)` helper | `storage.ResolveWorkspaceParam` (already used by `requireWorkspace`) | Already handles: 64-char hex full-hash fast path, 8+ char hex prefix lookup with ambiguity detection, name lookup, and consistent error messages — reimplementing risks drifting from existing prefix-ambiguity semantics |
| Passing values through the MCP session boundary | A custom session-state store keyed by connection ID | `context.Context` via `req.Context()` (SDK's own sanctioned mechanism) | The transport is explicitly `Stateless: true` (`streamable.go:18`) — building session storage would fight the SDK's documented design and reintroduce the per-connection state the phase's D-01 rationale explicitly avoids |

**Key insight:** Every piece of "new" logic in this phase is a thin composition of two things that already exist and are already tested in isolation (`storage.ResolveWorkspaceParam`, and the SDK's `req.Context()` propagation) — the only genuinely new code is the middleware function itself (~10 lines) and the fallback branch in `requireWorkspace` (~4 lines).

## Common Pitfalls

### Pitfall 1: Registering middleware after `echo.WrapHandler` instead of before
**What goes wrong:** If the middleware wraps the *result* of `echo.WrapHandler(streamableHandler)` (an `echo.HandlerFunc`) using `echo.MiddlewareFunc` composition, the query-param-to-context injection happens on the Echo layer, which the underlying SDK `http.Handler` never observes through `req.Context()`.
**Why it happens:** It's the more "idiomatic-looking" Echo pattern (matches `workspaceMiddleware`, `csrfMW` elsewhere in `routes.go`), so it's an easy default to reach for without checking that this specific downstream handler needs raw-`http.Request`-level context, not Echo-context.
**How to avoid:** Wrap `streamableHandler` (the `http.Handler` return value of `mcp.NewStreamableHTTPHandler`) directly, before it's passed to `echo.WrapHandler(...)`.
**Warning signs:** A test that calls the tool via the full HTTP path (not `mcpsdk.NewInMemoryTransports` in-process) with `?workspace=` in the URL and an omitted `workspace` arg would fail with "workspace is required" if this pitfall occurs — the planner should include exactly this kind of test (see Validation Architecture below), because the existing `setupMCPSecClient` helper bypasses the HTTP layer entirely via in-memory transports and would NOT catch this bug.

### Pitfall 2: Losing the context fallback inside `requireRegisteredWorkspace`'s early empty-check
**What goes wrong:** `requireRegisteredWorkspace` (tools.go:169-190) currently does its own `input == ""` check (line 171-173) *before* delegating to `a.requireWorkspace`. If this early check isn't also given the context fallback (or removed in favor of full delegation), write tools (`memory_write`, `memory_update`) would still demand an explicit `workspace` arg even when a connection default is configured — silently breaking D-05's "uniform application to both read and write tools."
**Why it happens:** The two functions look similar enough that a mechanical patch might update only the more visible `requireWorkspace` and miss the shadow copy of the same check three lines into its sibling.
**How to avoid:** Restructure so `requireRegisteredWorkspace` checks `"all"` explicitly on the raw arg first (unaffected by context fallback per D-02 — a context default is never `"all"`), then delegates directly to `a.requireWorkspace(ctx, args)` for everything else, without its own duplicate empty-check.
**Warning signs:** A test calling `memory_write` with a connection-level default configured and no `workspace` arg still returns `"workspace is required"` instead of succeeding.

### Pitfall 3: Assuming all `memory_*` tools need the schema edit
**What goes wrong:** Applying the required-fields removal to all 17 tool registrations (or to the wrong subset) either breaks tools that never had `workspace` as a schema property at all (`memory_ticket`, `memory_workspaces_list`) or leaves stale required-ness on tools that do need it.
**Why it happens:** CONTEXT.md's phase description estimated "~18" tools from memory without a fresh grep; the actual count and exact tool names differ.
**How to avoid:** Use the exact enumerated list below (Code Examples section) — 13 tools, verified by grep against the current file state in this research session.
**Warning signs:** `go vet`/tests pass but an agent hitting `memory_ticket` or `memory_workspaces_list` sees an unexpected schema change (these two never had `"workspace"` required and must NOT be touched).

## Code Examples

### Exact enumeration of tools requiring the schema edit (13 of 17 `memory_*` tools)

Verified via `grep -n '"workspace"' internal/mcp/tools.go` and `grep -n '}, \[\]string{' internal/mcp/tools.go` plus manual cross-reference against `func registerMemory*` boundaries, in this research session on the current `feat/mcp-workspace-config-binding` branch state:

| # | Tool Name | File | Required-fields line (current) | Required list (current) |
|---|-----------|------|-------------------------------|--------------------------|
| 1 | `memory_query` | `internal/mcp/tools.go` | 309 | `{"query", "workspace"}` |
| 2 | `memory_search` | `internal/mcp/tools.go` | 500 | `{"query", "workspace"}` |
| 3 | `memory_vsearch` | `internal/mcp/tools.go` | 847 | `{"query", "workspace"}` |
| 4 | `memory_get` | `internal/mcp/tools.go` | 1139 | `{"path", "workspace"}` |
| 5 | `memory_write` | `internal/mcp/tools.go` | 1245 | `{"content", "workspace"}` |
| 6 | `memory_tags` | `internal/mcp/tools.go` | 1416 | `{"workspace"}` |
| 7 | `memory_update` | `internal/mcp/tools.go` | 1499 | `{"workspace"}` |
| 8 | `memory_wake_up` | `internal/mcp/tools.go` | 1526 | `{"workspace"}` |
| 9 | `memory_graph` | `internal/mcp/tools.go` | 1656 | `{"workspace", "node"}` |
| 10 | `memory_trace` | `internal/mcp/tools.go` | 1758 | `{"workspace", "node"}` |
| 11 | `memory_impact` | `internal/mcp/tools.go` | 1863 | `{"workspace", "node"}` |
| 12 | `memory_symbols` | `internal/mcp/tools.go` | 1981 | `{"workspace"}` |
| 13 | `memory_flow` | `internal/mcp/tools.go` | 2104 | `{"workspace", "entry"}` |
| 14 | `memory_flowchart` | `internal/mcp/flowchart.go` | 24 | `{"workspace", "node"}` |

**Note:** this is actually **14 tools**, not 13 — `memory_flowchart` lives in a separate file (`internal/mcp/flowchart.go`) and was not visible in a `tools.go`-only grep. The phase description's own additional-context note ("get the exact list+line numbers") is satisfied by this table; the planner should treat `flowchart.go:24` as an equal-priority edit site alongside the 13 in `tools.go`.

**Tools that do NOT need this edit (do not have `"workspace"` in required-fields, or don't take a `workspace` param at all):**
| Tool | File:Line | Required list | Why excluded |
|------|-----------|----------------|---------------|
| `memory_status` | `tools.go:1463` | `nil` (no required fields) | Takes no `workspace` param — global health check |
| `memory_workspaces_resolve` | `tools.go:2045` | `{"path"}` | Takes `path`, not `workspace` — this IS the discovery tool this phase makes optional to call |
| `memory_workspaces_list` | `tools.go:2333` | `{}` (empty) | Takes no params at all |
| `memory_ticket` | `tools.go:2371` | `{"ticket"}` | Takes `ticket`, not `workspace` |

For each of the 14 edit sites, the mechanical change is: remove `"workspace"` from the `[]string{...}` slice passed as the last argument to `toolSchema(...)`, and update that tool's `"workspace"` property description (currently `"Workspace identifier — name (e.g. 'nano-brain') or full hash"` verbatim across all sites) to append: `" Optional if the MCP connection was configured with a default workspace via the ?workspace= URL query param; otherwise required."` per D-06.

### requireWorkspace fallback (target state)
```go
// Source: internal/mcp/tools.go:149-162 (current, HIGH confidence — read directly this session)
func (a *Adapter) requireWorkspace(ctx context.Context, args map[string]any) (string, *mcpsdk.CallToolResult) {
	input := argString(args, "workspace")
	if input == "" {
		if v, ok := ctx.Value(ctxKeyDefaultWorkspace{}).(string); ok && v != "" {
			input = v
		}
	}
	if input == "" {
		return "", errResult("workspace is required")
	}
	if input == "all" {
		return "all", nil
	}
	hash, err := storage.ResolveWorkspaceParam(ctx, a.queries, input)
	if err != nil {
		return "", errResult(err.Error())
	}
	return hash, nil
}
```

### requireRegisteredWorkspace (target state — restructured to avoid Pitfall 2)
```go
// Source: internal/mcp/tools.go:169-190 (current, HIGH confidence — read directly this session)
func requireRegisteredWorkspace(ctx context.Context, a *Adapter, args map[string]any) (string, *mcpsdk.CallToolResult) {
	if argString(args, "workspace") == "all" {
		return "", errResult("workspace_all_not_supported: this tool does not accept the 'all' workspace scope; provide a specific registered workspace name or hash")
	}
	ws, errRes := a.requireWorkspace(ctx, args) // now includes context fallback + empty-check
	if errRes != nil {
		return "", errRes
	}
	if _, err := a.queries.GetWorkspaceByHash(ctx, ws); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errResult(fmt.Sprintf("workspace_not_registered: workspace %q is not registered; use POST /api/v1/init to register it first", ws))
		}
		return "", errResult(fmt.Sprintf("workspace_lookup_failed: %v", err))
	}
	return ws, nil
}
```
Note the error message on the `workspace_not_registered` branch now references `ws` (the resolved hash) instead of `input` (the raw arg) — acceptable since `ws` is either the resolved hash or, when resolution failed inside `requireWorkspace`, that error would have already returned before reaching this line. Minor behavior difference from current code (which references the raw `input` string in the error) — planner should verify this doesn't break `tools_security_test.go`'s existing string-match assertions (`strings.Contains(text, "workspace_not_registered")` — checked, only substring-checks that literal error code, not the interpolated value, so this is safe).

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Agent must call `memory_workspaces_list` → `memory_workspaces_resolve` → `memory_wake_up` before any query | `.mcp.json` URL carries `?workspace=<name-or-hash>`; agent omits `workspace` arg entirely for the common single-project case | This phase | Real session-data cited in CONTEXT.md's `<specifics>` shows only ~22% of nano-brain's own sessions call `memory_wake_up` despite it being documented as mandatory — this phase removes the friction point, not the memory of calling it |

**Deprecated/outdated:** None — this is purely additive; no existing behavior is deprecated. The multi-step discovery flow documented in `AGENTS.md`'s "Session Workflow" section remains valid and necessary for multi-workspace connections or connections without a bound default.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `mcp.NewStreamableHTTPHandler`'s returned `http.Handler` can be wrapped by an arbitrary middleware function before `echo.WrapHandler` without breaking SSE/streaming behavior (chunked responses, long-lived connections) | Architecture Patterns / Pattern 1 | If SSE/streaming semantics depend on `echo.WrapHandler` seeing the *exact* handler type (unlikely, since `echo.WrapHandler` accepts any `http.Handler` interface), wrapping could subtly change buffering/flushing behavior. Low risk — `http.HandlerFunc` composition is a standard no-op passthrough for streaming as long as the wrapper doesn't buffer the body itself (this one doesn't; it only touches `r.URL.Query()` and `r.Context()`), but should be verified with a manual streamable-HTTP smoke test during execution, not just unit tests. |
| A2 | Removing `"workspace"` from a tool's JSON-schema `required` array is safe for all MCP clients currently in use (Claude Code, OpenCode) — i.e., no client validates required-ness client-side in a way that would suppress the parameter from being offered to the LLM even when the server allows omitting it | Standard Stack / Architecture Patterns | If a client aggressively hides optional-but-present schema fields from its own tool-calling UI, agents might not realize they *can* still pass `workspace` explicitly to override the connection default. This is a UX risk, not a correctness risk — D-06 already accounts for keeping the parameter in the schema (only required-ness changes), so the parameter remains visible to any spec-compliant client. |

**If this table is empty:** N/A — two assumptions logged above; both are low-risk architectural assumptions about third-party (SDK/client) behavior that could not be fully verified without live end-to-end testing against a real MCP client, which is out of scope for research and belongs in the phase's manual verification step.

## Open Questions

1. **Should the middleware wrapper function live in `internal/mcp` (package `mcp`) or `internal/server/middleware` (package `middleware`)?**
   - What we know: `internal/server/middleware/` currently holds Echo-native middleware (`Auth`, `CSRF`) used across the whole server, not just MCP routes. The new middleware is MCP-transport-specific (only relevant to `/mcp`) and needs privileged access to the unexported `ctxKeyDefaultWorkspace` type that `requireWorkspace` in package `mcp` reads.
   - What's unclear: Whether the project prefers all "middleware-shaped" code centralized in `internal/server/middleware` regardless of scope, for discoverability.
   - Recommendation: Keep it in `internal/mcp/streamable.go`, colocated with `NewStreamableHTTPHandler` and the context key it defines — this avoids an import cycle risk (package `middleware` would need to import package `mcp` to share the context-key type, or the key would need to move to a third shared location) and keeps the "reads its own context key" invariant local to one package, matching Go convention for context-key ownership (the package that defines a context key should also be the package that reads it).

2. **Does `memory_flowchart`'s separate file (`flowchart.go`) indicate other tool-registration files exist that weren't checked?**
   - What we know: `RegisterTools` in `tools.go:28-47` calls 17 `register*` functions; 16 are defined in `tools.go` itself, 1 (`registerMemoryFlowchart`) is defined in `flowchart.go`. A full `ls internal/mcp/*.go` was run and cross-referenced — no other `register*` function definitions exist outside these two files.
   - What's unclear: Nothing — this was fully resolved during research (see Code Examples enumeration, 14 total edit sites confirmed complete).
   - Recommendation: No further action needed; the enumeration above is complete and verified.

## Environment Availability

Skipped — this phase has no external tool/service dependencies beyond what's already running (Go toolchain, Postgres for `nanobrain_test`, both already verified working via a passing `go build ./...` and `go test -race -short ./internal/mcp/... ./internal/server/...` in this research session).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `httptest`, project convention (`-race -short` for unit, integration-tagged tests skip in `-short` via `testing.Short()` guard) |
| Config file | none — Go's built-in test runner, no separate config |
| Quick run command | `go test -race -short ./internal/mcp/... ./internal/server/...` |
| Full suite command | `go test -race -tags=integration ./internal/mcp/... ./internal/server/...` (against `nanobrain_test` on the isolated test DSN — see `mcpSecTestDSN()` in `tools_security_test.go`, defaults to `NANO_BRAIN_TEST_DATABASE_URL` or the hardcoded `nanobrain_test` connection string) |

### Phase Requirements → Test Map
No formal REQ IDs are mapped to this phase (backlog-adjacent feature). Test coverage below is derived from CONTEXT.md's locked decisions (D-01 through D-07):

| Decision | Behavior | Test Type | Automated Command | File Exists? |
|----------|----------|-----------|---------------------|-------------|
| D-03 | Explicit `workspace` arg wins over connection default | unit | `go test -race -short ./internal/mcp/... -run TestRequireWorkspace` | ❌ Wave 0 — new test in `internal/mcp/tools_internal_test.go` (package `mcp`, has access to unexported `requireWorkspace`) |
| D-04 | Neither arg nor context default present → unchanged `"workspace is required"` error | unit | same as above | ❌ Wave 0 — same file |
| D-05 | Context default applies to both read (`requireWorkspace`) and write (`requireRegisteredWorkspace`) paths | unit | same as above | ❌ Wave 0 — same file; extend existing `tools_security_test.go` for the write-path MCP-transport-level version (uses `mcpsdk.NewInMemoryTransports`, does NOT exercise the real HTTP middleware — see next row for that) |
| D-01/D-07 | Full HTTP round-trip: `?workspace=` on the URL reaches the tool handler through the real `echo.WrapHandler`-wrapped streamable handler (catches Pitfall 1) | integration | `go test -race -tags=integration ./internal/mcp/... -run TestStreamableHTTP` or equivalent in `internal/server` | ❌ Wave 0 — new test needed; must use `httptest.NewServer` with the real Echo instance + real query string on `/mcp?workspace=...`, NOT the in-memory transport helper (`setupMCPSecClient`) which bypasses HTTP entirely |
| D-06 | Schema `required` no longer lists `"workspace"` for the 14 tools enumerated above | unit | `go test -race -short ./internal/mcp/... -run TestToolSchema` | ❌ Wave 0 — new test asserting `tools/list` response schema for a sample of the 14 tools |

### Sampling Rate
- **Per task commit:** `go test -race -short ./internal/mcp/... ./internal/server/...`
- **Per wave merge:** `go test -race -tags=integration ./internal/mcp/... ./internal/server/...` (requires `nanobrain_test` Postgres reachable — see `AGENTS.md`/CLAUDE.md testing-isolation rule: **never** point this at the dev DB on :3100/:3199 confusion — tests use :3199 config per project convention)
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/mcp/tools_internal_test.go` (package `mcp`, unexported access) — new unit tests for `requireWorkspace`/`requireRegisteredWorkspace` context-fallback precedence (D-03, D-04, D-05 read-path)
- [ ] Extend `internal/mcp/tools_security_test.go` — add a context-fallback variant of the existing `TestMemoryWrite_AcceptsRegisteredWorkspace` pattern (D-05 write-path via in-memory transport, does not need real HTTP)
- [ ] New integration test exercising the real HTTP path with `?workspace=` query param against a `httptest.NewServer`-hosted Echo instance (D-01/D-07, catches Pitfall 1 — the single most important test in this phase since it's the only one that would catch an `echo.MiddlewareFunc`-vs-plain-`http.Handler` wiring mistake)
- [ ] Schema assertion test confirming the 14 enumerated tools no longer require `"workspace"` while 4 excluded tools are unaffected (D-06)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | This phase does not touch authentication; MCP connections are already trusted/local per existing `Auth` middleware scope (bypassable for `/mcp` per current config — out of scope for this phase to change) |
| V3 Session Management | No | Transport is explicitly stateless (`Stateless: true`); no session tokens introduced |
| V4 Access Control | Yes | The workspace binding is itself an access-scoping mechanism — a query-param-derived default must NOT be able to grant access beyond what an explicit `workspace` arg would grant. D-03 (explicit arg always wins) and D-05 (uniform application, no new bypass) are the controls; `requireRegisteredWorkspace`'s existing registration check (unchanged) remains the actual authorization gate for writes |
| V5 Input Validation | Yes | `r.URL.Query().Get("workspace")` is untrusted input from the URL; it MUST be passed through the exact same `storage.ResolveWorkspaceParam` validation as an explicit tool argument — this phase must not introduce a second, laxer validation path. Malformed/malicious query values (e.g., SQL-injection-shaped strings) are already handled safely today because `ResolveWorkspaceParam` uses parameterized `sqlc`-generated queries; the new call site inherits this for free by reusing the same function |
| V6 Cryptography | No | No cryptographic operations introduced; workspace hashes are SHA-256 content hashes already computed elsewhere, not used as a security boundary here |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Query-param injection used to bypass workspace registration check on write tools | Elevation of Privilege | `requireRegisteredWorkspace` must apply its registration check (`GetWorkspaceByHash`) to the *resolved* value regardless of whether it came from the explicit arg or the context default — this is automatic if the restructuring in Code Examples (delegating fully to `requireWorkspace` then checking registration) is followed correctly; it would NOT be automatic if a shortcut trusted the context value without the same resolution+registration path |
| Cross-workspace data leak via a stale/wrong `?workspace=` value cached in a long-lived `.mcp.json` config after a workspace is deleted/re-hashed | Information Disclosure | Unchanged from today's behavior: `storage.ResolveWorkspaceParam`'s hash/name lookup already fails cleanly with "workspace not found" for unregistered/deleted workspaces — the connection-default path inherits this identical failure mode, no new risk introduced |
| Malicious `.mcp.json` config value used to enumerate valid workspace hashes via error-message timing/content differences | Information Disclosure (low severity) | Not new to this phase — the same enumeration surface already exists via the explicit `workspace` tool argument today; binding via URL query param does not change the trust boundary (both are attacker-controlled-if-attacker-controls-the-config, and `.mcp.json` is already a locally-trusted file per this project's threat model, same as noted in `docs/DASHBOARD_SPLIT_PLAN.md`'s PNA/localhost threat-model framing) |

## Sources

### Primary (HIGH confidence)
- Vendored SDK source, directly read in this session: `/Users/tamlh/go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v0.8.0/mcp/streamable.go` (lines 78-105, 270-312) — confirms `NewStreamableHTTPHandler` signature, `StreamableHTTPOptions.Stateless`, and the exact `server.Connect(req.Context(), transport, connectOpts)` call with its "Pass req.Context() here, to allow middleware to add context values" comment
- Vendored SDK source: `/Users/tamlh/go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v0.8.0/mcp/server.go` (lines 1076-1101) — confirms `ServerSession.handle(ctx, ...)` receives and propagates the connection-scoped context through to tool dispatch
- Direct codebase inspection: `internal/mcp/tools.go` (full grep + targeted reads), `internal/mcp/flowchart.go`, `internal/mcp/streamable.go`, `internal/server/routes.go`, `internal/server/middleware.go`, `internal/mcp/tools_security_test.go`, `internal/server/middleware_test.go` — all read directly in this session on the current branch state
- `go.mod` — confirms `github.com/modelcontextprotocol/go-sdk v0.8.0` pin
- `go build ./...` and `go test -race -short ./internal/mcp/... ./internal/server/...` — both run in this session, both pass, establishing a clean baseline before planning

### Secondary (MEDIUM confidence)
- `.planning/phases/09-.../09-CONTEXT.md` — user-approved decisions from `/gsd-discuss-phase`, already grounded in the same SDK source re-verified above; treated as authoritative for product decisions (D-01 through D-07), re-validated rather than re-derived per task instructions

### Tertiary (LOW confidence)
None — no unverified web sources were needed for this phase; it is entirely internal-codebase and vendored-dependency research.

## Metadata

**Confidence breakdown:**
- Standard Stack: HIGH - no new dependencies; existing SDK version confirmed via go.mod and local module cache inspection
- Architecture: HIGH - middleware wiring subtlety (Echo vs plain http.Handler) independently discovered and verified by reading both the routing code and the SDK source in this session, not inferred
- Pitfalls: HIGH - both pitfalls (middleware layering, duplicate empty-check) derived from direct code reading, not speculation
- Package Legitimacy: N/A - no new packages

**Research date:** 2026-07-01
**Valid until:** 30 days (stable internal codebase + pinned SDK version; re-check if `go-sdk` is upgraded past v0.8.0 before this phase executes, since the `req.Context()` extension point, while unlikely to change, is SDK-version-specific behavior)
