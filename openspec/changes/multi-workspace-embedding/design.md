## Context

nano-brain's `startServer()` binds to a single workspace at startup — determined by `process.cwd()`. The `Store`, file watcher, and embedding interval all operate on that one workspace's SQLite DB. When running as a remote SSE daemon via `npx nano-brain serve`, only the startup workspace gets background embedding. Other configured workspaces (listed in `config.yml` → `workspaces` map) accumulate pending embeddings indefinitely.

Current architecture:
- `startServer()` → resolves workspace from cwd → opens one `{dirName}-{hash}.sqlite`
- `startWatcher()` → receives one `store`, one `projectHash`, one `workspaceRoot`
- Embed interval → calls `embedPendingCodebase(store, embedder, 50, projectHash)` for that one workspace
- Harvest interval → harvests sessions into one workspace's DB
- File watcher → watches one workspace's codebase directory

## Goals / Non-Goals

**Goals:**
- Background embedding processes pending documents across ALL configured workspaces in `config.yml`
- A single `npx nano-brain serve` daemon handles embedding for every workspace without manual intervention
- No changes to MCP tool behavior — tools still operate on the workspace determined by the connected client

**Non-Goals:**
- Multi-workspace MCP tool routing (Level 2 — separate change)
- Dynamic workspace discovery (only workspaces in config.yml are processed)
- Concurrent embedding across workspaces (sequential is fine — embedding is I/O-bound on the API)
- Per-workspace file watching for codebase changes (keep watching only the startup workspace for now; embedding catches up via the interval regardless)

## Decisions

### 1. Store factory instead of store pool

**Decision:** Create a `createStoreForWorkspace(config, workspacePath)` helper that opens a workspace's DB, returns a store, and the caller closes it when done. No persistent pool.

**Why:** A pool of open SQLite connections for all workspaces wastes file descriptors and memory. Most workspaces only need their DB open for a few seconds during the embed interval. Opening/closing better-sqlite3 is cheap (~1ms).

**Alternative considered:** Keep all stores open in a `Map<string, Store>`. Rejected — unnecessary resource usage for a background task that runs every 60s.

### 2. Embed interval iterates workspaces sequentially

**Decision:** The embed interval loop becomes:
```
for each workspace in config.workspaces:
  if workspace.codebase.enabled:
    open store for workspace
    embedPendingCodebase(store, embedder, 50, projectHash)
    close store
```

**Why:** Embedding is bottlenecked by the embedding API (Voyage AI rate limit), not SQLite. Sequential iteration is simpler and avoids concurrent SQLite access issues. Each workspace gets up to 50 docs per cycle.

**Alternative considered:** Parallel embedding across workspaces. Rejected — the embedding provider is shared and rate-limited. Parallel would just hit rate limits faster without throughput gain.

### 3. Harvest across all workspaces

**Decision:** Session harvesting already writes to per-workspace DBs based on session file paths. The harvester just needs to open the correct DB for each workspace's sessions. Same pattern as embedding — open store, harvest, close.

### 4. Keep file watcher single-workspace

**Decision:** The chokidar file watcher continues watching only the startup workspace's codebase directory. Other workspaces rely on the embed interval to pick up changes when `memory_index_codebase` or `memory_update` is called from their respective OpenCode sessions.

**Why:** Watching all workspace directories from a single daemon adds complexity (multiple chokidar instances, debouncing per workspace) for marginal benefit. The embed interval already catches pending docs every 60s. File watching for other workspaces can be added later if needed.

## Risks / Trade-offs

- **[SQLite locking]** If a local MCP instance is running for a workspace AND the serve daemon tries to embed for the same workspace, they'll contend on the SQLite DB. → **Mitigation:** better-sqlite3 uses WAL mode, which allows concurrent reads. Write contention is brief (embedding writes are small batches). The singleton guard already handles this for the same workspace — cross-workspace is fine since they use different DB files.
- **[Embed cycle duration]** With 8 workspaces × 50 docs each, a single embed cycle could take several minutes if the API is slow. → **Mitigation:** The interval timer only starts after the previous cycle completes (not fixed interval). If a cycle takes 5 minutes, the next one starts 60s after it finishes.
- **[Config changes]** If workspaces are added/removed from config.yml while the daemon is running, the embed interval won't pick them up until restart. → **Mitigation:** Acceptable for Level 1. Config reload can be added later.
