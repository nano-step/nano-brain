# Phase 9: MCP workspace config binding - Context

**Gathered:** 2026-07-01
**Status:** Ready for planning

<domain>
## Phase Boundary

Let a `.mcp.json` MCP server entry bind a single default workspace to its connection (via a URL query param on the Streamable HTTP URL), so every tool call from that connection can omit the `workspace` argument and still resolve correctly. Explicit `workspace` args continue to work exactly as today and take precedence when supplied. No change to workspace resolution semantics (name/hash/`"all"`) beyond adding a fallback source.

</domain>

<decisions>
## Implementation Decisions

### Config surface
- **D-01:** Bind the default via a URL query parameter on the existing MCP URL: `"url": "http://localhost:3100/mcp?workspace=<name-or-hash>"`. Rejected alternatives: custom HTTP header (depends on client-side schema support for custom headers, not guaranteed across all MCP clients) and a separate env var (HTTP transport has no client-supplied env passthrough, unlike stdio transport). A URL query param works with literally any HTTP-based MCP client with zero schema support required, and — critically — survives the transport's deliberate statelessness (`internal/mcp/streamable.go:9-20`) because the client resends the identical configured URL on every request; no server-side session storage needed.
- **D-02:** Param name is `workspace` (matches the existing tool-call argument name exactly) and accepts the same values `requireWorkspace` already accepts today: a workspace name or a full hash. Do not accept `"all"` as a connection default (a connection is meant to pin one project; `"all"` cross-workspace queries remain an explicit per-call opt-in only).

### Resolution precedence
- **D-03:** Per-call `workspace` argument always wins when present — the connection default is purely a fallback for when the argument is omitted. This preserves 100% backward compatibility (every existing caller that already passes `workspace` sees zero behavior change) and still allows a bound connection to make an explicit cross-workspace `"all"` query when the agent asks for one directly.
- **D-04:** When neither the per-call arg nor a connection default is present, behavior is unchanged from today: `requireWorkspace` returns the exact same `"workspace is required"` error (`internal/mcp/tools.go:152`). No silent fallback to a guessed workspace, ever.
- **D-05:** The connection default applies uniformly to both read tools (`requireWorkspace`) and write tools (`requireRegisteredWorkspace`, used by `memory_write`/`memory_update`, `internal/mcp/tools.go:169`). Rationale: a `.mcp.json` entry is inherently scoped to one project by the person/process that wrote that config file — pinning "this connection = this workspace" is not ambiguous the way a shared multi-tenant default would be, so there's no added mis-write risk versus today's explicit-arg-required behavior.

### Schema visibility
- **D-06:** Remove `"workspace"` from each tool's required-fields list in `toolSchema(...)` (currently required across all ~18 `memory_*` tools registered in `internal/mcp/tools.go`) so an LLM agent is not forced to supply it. Update each tool's `workspace` parameter description to note: "Optional if the MCP connection was configured with a default workspace via the `?workspace=` URL query param; otherwise required." Keep the parameter itself in every schema (still overridable per-call) — only the required-ness changes.

### Integration point
- **D-07:** Implement as HTTP middleware wrapping the handler returned by `NewStreamableHTTPHandler` (`internal/mcp/streamable.go:13`, wired at `internal/server/routes.go:132`), reading `r.URL.Query().Get("workspace")` and injecting it into `r.Context()` via a package-private context key before delegating to the SDK's handler. This is the SDK's own documented, sanctioned extension point — `go-sdk@v0.8.0/mcp/streamable.go:297`: *"Pass req.Context() here, to allow middleware to add context values."* `requireWorkspace`/`requireRegisteredWorkspace` read that context key as the fallback source, reusing the existing `storage.ResolveWorkspaceParam` resolution — no duplicate resolution logic.

### Claude's Discretion
- Exact context-key type/name (unexported struct-type key per Go convention, e.g. `type ctxKeyDefaultWorkspace struct{}`) — implementation detail, no product-facing impact.
- Whether the middleware resolves the query-param value to a hash eagerly (once per request) or leaves it as a raw string for `requireWorkspace` to resolve lazily — either works given `storage.ResolveWorkspaceParam` is cheap and idempotent; planner/executor should pick whichever keeps the diff smallest.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### MCP transport & tool resolution
- `internal/mcp/streamable.go` — Streamable HTTP handler factory; stateless-mode comment explains why context injection (not session state) is the right mechanism.
- `internal/mcp/tools.go:149-180` — `requireWorkspace` and `requireRegisteredWorkspace`, the two functions that need the context-fallback added.
- `internal/mcp/tools.go` (all `Name: "memory_*"` registrations) — every tool's `toolSchema(...)` call needs its required-fields list updated to drop `"workspace"`.
- `internal/server/routes.go:132` — where `NewStreamableHTTPHandler` is wired into the HTTP mux; likely where the new middleware wrapping happens.
- `github.com/modelcontextprotocol/go-sdk@v0.8.0/mcp/streamable.go:297` — SDK source comment confirming `req.Context()` is the sanctioned place for middleware-injected values.

### Config & docs to update
- `docs/SETUP_AGENT.md:170-200` — current `.mcp.json` config examples (Claude Code, OpenCode, generic) that need the new `?workspace=` example added.
- `README.md:243` — MCP config example.
- `docs/reference-readme.md:169` — duplicate MCP config example, keep in sync.

No external specs/ADRs found for this phase - requirements fully captured in decisions above.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `storage.ResolveWorkspaceParam` (used by `requireWorkspace` today) — reuse as-is for resolving the context-default value; do not write a parallel resolution path.
- `toolSchema(...)` helper (`internal/mcp/tools.go`) already takes a required-fields slice as its last argument — dropping `"workspace"` from that slice per tool is the exact mechanical change needed, no new helper required.

### Established Patterns
- The SDK's `getServer func(*http.Request) *Server` factory in `NewStreamableHTTPHandler` already receives the raw `*http.Request` per connection (`internal/mcp/streamable.go:14`) — but currently ignores it. The new middleware should wrap the *returned http.Handler*, not this factory function, since the factory only selects which `*Server` to use, not per-request context — confirm during planning which layer actually gets to mutate the request before the SDK dispatches to tool handlers.

### Integration Points
- `internal/server/routes.go:132` is the single call site that constructs the handler — the natural place to apply the new middleware wrapper.

</code_context>

<specifics>
## Specific Ideas

User's own framing: "Tôi muốn thêm workspace name vào config của MCP, từ đó không cần tự discovery hash project nữa" (add workspace name to the MCP config so [the agent] no longer needs to self-discover the project's hash). This phase exists directly because of the earlier finding (real data from harvested Claude Code sessions in `nanobrain_dev`) that only ~22% of active nano-brain sessions in nano-brain's own repo call `memory_wake_up` despite it being documented as the mandatory first step — the multi-step discovery dance (`memory_workspaces_list` -> `memory_workspaces_resolve` -> `memory_wake_up` -> actual query) is friction agents routinely skip. Binding the workspace at the connection level removes the need for discovery entirely for the common single-project case.

</specifics>

<deferred>
## Deferred Ideas

- Proactive session-start context injection (`UserPromptSubmit`-style hook that auto-calls `memory_wake_up` without the agent choosing to) - a separate, larger capability discussed earlier in this session; not part of this phase's scope (this phase only removes the *workspace-identification* friction, not the *remembering-to-call-wake_up-at-all* friction). Candidate for its own future phase.
- Consolidating/deprecating low-usage tools (`memory_ticket` at 0 real-world uses, `memory_workspaces_list`/`memory_tags` at very low usage) - identified from the same session-data analysis, out of scope here.
- New language/framework extractors (PHP, Java) - unrelated capability, out of scope here.

### Reviewed Todos (not folded)
None - discussion stayed within phase scope.

</deferred>

---

*Phase: 9-MCP workspace config binding*
*Context gathered: 2026-07-01*
