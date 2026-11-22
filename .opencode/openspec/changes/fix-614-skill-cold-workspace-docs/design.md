# Design

## Verification Against Current Server Code

1. **Unregistered / unknown workspace (`internal/server/middleware.go:223-236, 305-311`, `internal/mcp/tools.go:161-210, 2485-2523`, `internal/storage/resolve.go:28-57`):**
   - REST `/api/v1/*` routes validate workspace registration in `workspaceMiddleware` and return HTTP `404` `workspace_not_found` (`"workspace %q is not registered; use POST /api/v1/init to register it first"`), with `workspaceRegisteredMiddleware` returning HTTP `400` `workspace_not_registered` as fallback.
   - MCP write tools (`memory_write`, `memory_update`) return `isError: true` with `"workspace_not_registered: workspace %q is not registered; use POST /api/v1/init to register it first"`; MCP read tools return `isError: true` on unknown workspace names/prefixes (`"workspace %q not found"` / `"no workspace matching prefix %q"`), while `memory_workspaces_resolve` reports `registered: false` (and optional `covered_by` ancestor workspace, #565). Note: `ResolveWorkspaceParam` returns a 64-hex hash without DB lookup on read MCP tools, so agents MUST call `memory_workspaces_resolve` first rather than passing raw unverified hashes.
   - Recovery: `POST /api/v1/init` (`{"root_path": "<abs_path>"}`) or `nano-brain init --root <path>` (space-separated `--root`, without `--yes`; `cmd/nano-brain/commands.go:38-44, 88-95`).

2. **Freshly registered workspace & async incremental reindex (`internal/server/handlers/workspace.go:58-106, 165-183`, `internal/server/handlers/reindex.go:153-316`, `internal/server/handlers/stats.go:70-85`):**
   - `POST /api/v1/init` registers default collections (`memory`, `sessions`, `code`) and attaches the file watcher for async initial ingestion (#161).
   - Incremental `POST /api/v1/reindex` returns HTTP `202` with `status: "queued"`, `files_pending_ingest`, and `watcher_triggered: true`, while `embedded: 0` and `chunks_enqueued: 0` are expected at response time because ingestion runs asynchronously in the watcher (#622/#694) — NOT `"cannot fix via API"`. Verify progress via `GET /api/v1/stats?workspace=<hash>` (`docs_total`, `chunks_total`, `graph_edges_by_type`) and `memory_status` (`queue_pending`).

3. **Out-of-date symbols / graph edges / flowcharts (`internal/server/handlers/reindex.go:436-460`, `internal/watcher/watcher.go:1332-1387, 1537-1624`):**
   - `POST /api/v1/update` (`{"workspace": "<hash>"}`) runs `ReextractEdgesForWorkspace` (graph edges + `function_flowcharts` + orphan sweep) and `ReextractSymbolsForWorkspace` asynchronously across watched collections without wiping `memory`/`sessions` or re-embedding chunks. Both `memory_symbols` and `memory_impact` read PostgreSQL (neither reads raw unindexed files).

4. **Empty impact response (`internal/server/handlers/impact.go:32-36, 65-87`, `internal/mcp/tools.go:2284-2382`):**
   - Both `POST /api/v1/graph/impact` and `memory_impact` always emit `impacted: []` (never `null`) plus `node_found: true|false` (and `count` on MCP) to distinguish a known leaf node (`node_found: true`, zero callers) from an unknown/unindexed node (`node_found: false`).
