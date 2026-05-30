# Design: Fix Summary Workspace-Registration Leaks

## 1. Problem Statement

Currently, the summary feature trusts the `workspace_hash` field across multiple layers without verifying it corresponds to a row in the `workspaces` table. This creates orphan documents that:

- Pollute cross-workspace queries (`workspace: "all"`)
- Bypass workspace isolation (multi-tenant correctness)
- Cannot be cleaned up via the standard `DELETE /api/v1/workspaces/:hash` endpoint
- Survive across server restarts indefinitely

The fix must close the leak at **all entry points** because any unfixed path is exploitable.

## 2. Architecture Decision Records

### ADR-1: Defense-in-depth at 5 layers, not 1

**Decision:** Validate workspace registration at HTTP middleware, MCP tool handlers, harvester init, Persister.Save, AND PostgreSQL FK constraint.

**Rationale:** A single validation point is fragile. Future code paths added by other developers will likely skip a centralized check. Layering ensures:
- HTTP layer catches external API abuse early (cheap rejection, no DB hit beyond the registration lookup)
- MCP tool handlers catch MCP-transport writes (MCP bypasses HTTP middleware — `/mcp` and `/sse` routes register the streamable handler directly without going through `workspaceMiddleware`, verified at `routes.go:73-78` and `internal/mcp/tools.go:485-659`)
- Harvester layer catches configuration errors at startup (operator sees warning immediately)
- Persister layer catches internal API misuse (defense against future code paths)
- DB layer catches everything else (last line of defense; cannot be bypassed by application code)

**Alternative considered:** Centralize all validation in middleware only.
- Rejected because: bypasses harvester paths (harvester does not go through HTTP); bypasses MCP transport (MCP tools execute in their own handler chain); creates a "single point of failure" anti-pattern.

**Alternative considered:** FK constraint only.
- Rejected because: produces opaque database errors at the worst possible time (mid-transaction during summary persist). Application-level errors at the entry point are much better UX.

**Implementation pattern:** No shared `WorkspaceQuerier` interface. Each consumer uses on-demand `q := sqlc.New(db)` to match the existing codebase pattern (e.g., `internal/harvest/opencode_sqlite.go:115`). Sharing an interface across `internal/server/` and `internal/summarize/` would risk circular imports and add abstraction without benefit. Inline usage is simple and consistent.

### ADR-2: Per-route opt-in for middleware enforcement, not global

**Decision:** Add a new middleware `workspaceRegisteredMiddleware()` that is applied per-route, not globally to all workspace-scoped endpoints.

**Rationale:** Some endpoints legitimately accept the `all` scope (e.g., `POST /api/v1/query` with `workspace: "all"` is documented in the README). Forcing global registration enforcement would break that contract. Per-route opt-in is explicit and surgical.

**Applied to (write endpoints only):**
- `POST /api/v1/summarize`
- `POST /api/v1/write`
- `POST /api/v1/embed`
- `POST /api/v1/reindex`
- `POST /api/v1/update`

**NOT applied to (read endpoints + management):**
- `POST /api/v1/query`, `/search`, `/vsearch` — accept `all` for cross-workspace search
- `POST /api/v1/get`, `/multi-get` — read-only
- `GET /api/v1/wake-up`, `/workspaces`, `/tags`, `/collections` — read-only or management
- `DELETE /api/v1/workspaces/:hash` — operates on workspaces table itself
- `POST /api/v1/init` — creates the workspace

### ADR-3: OpenCode orphan sessions are SKIPPED, and auto-registration is REMOVED

**Decision:** Two changes to `internal/harvest/opencode_sqlite.go:141-164`:

1. **Skip orphan sessions** — When an OpenCode session has empty `worktree` OR its `WorkspaceHash(worktree)` is not registered, the harvester logs WARN and skips the session entirely. No fallback workspace is created.
2. **Remove auto-registration** — The existing call to `UpsertWorkspace` (currently at lines 155-163) is REMOVED. The harvester no longer silently creates workspace entries for arbitrary worktree paths it discovers in the OpenCode SQLite DB.

**Rationale:** Auto-registering would silently extend the trust boundary AND silently re-introduce orphan-equivalent state (a workspace exists but the operator never approved it). Skipping is conservative and visible to operators via logs. Operators who want a workspace harvested MUST use `POST /api/v1/init` or `nano-brain init --root=<path>` explicitly.

**Migration implication (breaking change for some operators):**
- Operators who never explicitly ran `nano-brain init` for their OpenCode worktrees have been relying on implicit auto-registration. After this change, their sessions will be skipped until they manually register the workspace(s).
- This is called out in release notes with a clear command sequence to register each worktree found in OpenCode.
- The `harvester.opencode.db_root` mode already filters by registered workspaces at discovery time (see `main.go:441` `ScanOpenCodeDBRoot`), so multi-DB deployments are unaffected. Only `db_path` mode (single DB) deployments need to register manually.

**Alternative considered:** Keep `UpsertWorkspace` for `db_path` mode but not for `db_root` mode.
- Rejected: split behavior is confusing and surface-area-expanding. Same enforcement everywhere is simpler.

**Alternative considered:** Keep fallback workspace but auto-register it.
- Rejected: extends trust boundary silently, defeats purpose of registration.

**Alternative considered:** Keep fallback workspace but mark it as "unregistered" in metadata.
- Rejected: adds DB schema complexity for a corner case; better to just skip.

### ADR-4: FK constraints with ON DELETE CASCADE, not RESTRICT

**Decision:** Migration 00011 uses `ON DELETE CASCADE` for `documents.workspace_hash → workspaces.hash` and `chunks.workspace_hash → workspaces.hash`.

**Rationale:** The current `DELETE /api/v1/workspaces/:hash` handler performs application-layer cascade. The FK with CASCADE simply formalizes this — same observable behavior, but enforced at DB layer for safety.

**Alternative considered:** `ON DELETE RESTRICT`.
- Rejected: would require the handler to delete documents first, then the workspace. More handler code, no benefit.

**Alternative considered:** `ON DELETE SET NULL`.
- Rejected: would create the very orphan state we are trying to prevent.

### ADR-5: Cleanup command is separate from migration

**Decision:** Provide `nano-brain cleanup-orphan-workspaces [--dry-run]` as a standalone command. The migration does NOT auto-cleanup.

**Rationale:**
- Cleanup deletes data; operators must consent (dry-run first, then real).
- Migration should be deterministic and reversible. Data deletion is not.
- Splitting allows operators to inspect orphans before deletion.

**Implementation:**
```
nano-brain cleanup-orphan-workspaces --dry-run
# Output:
# Found 1234 documents under 5 unregistered workspace_hash values.
# Would delete:
#   workspace_hash=9420b967... → 42 docs (likely OpenCode fallback)
#   workspace_hash=3215c9d7... → 1187 docs (likely OpenCode storage fallback)
#   ...
# Run without --dry-run to apply.
```

The migration's `Up` SQL includes a comment block as the first lines:
```sql
-- IMPORTANT: Before applying this migration, run:
--   nano-brain cleanup-orphan-workspaces
-- This migration adds FK constraints. If orphans exist, this migration WILL FAIL
-- with a PostgreSQL foreign key violation. Run cleanup first.
```

## 3. Component Changes

### 3.1 `internal/summarize/persist.go`

**Change:** Add workspace registration check at the top of `Persister.Save()`.

```go
func (p *Persister) Save(ctx context.Context, summaryMarkdown string, meta SessionMetadata) error {
    q := sqlc.New(p.db)

    // Defense-in-depth: reject unregistered workspace_hash.
    // Closes leaks #3 + #4 (Persister + harvest_adapter trust passthrough).
    if _, err := q.GetWorkspaceByHash(ctx, meta.WorkspaceHash); err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            p.logger.Warn().
                Str("workspace_hash", meta.WorkspaceHash).
                Str("session_id", meta.SessionID).
                Msg("persist: refusing to save summary for unregistered workspace")
            return fmt.Errorf("persist: workspace_not_registered: %s", meta.WorkspaceHash)
        }
        return fmt.Errorf("persist: workspace lookup failed: %w", err)
    }

    // ... existing logic (SHA-256 idempotency check, upsert, etc.)
}
```

**Test:** New `internal/summarize/persist_security_test.go` with cases for:
- Unregistered hash → returns `workspace_not_registered` error, no DB write
- Registered hash → proceeds with existing behavior

**LoC added:** ~12 lines + ~50 lines test

### 3.2 `internal/harvest/opencode_sqlite.go`

**Change:** Two changes in lines 141-164 region:
1. Skip orphan sessions (no fallback to `WorkspaceHash(dbPath)`)
2. Remove auto-registration via `UpsertWorkspace`

**Pattern:** Use on-demand `q := sqlc.New(h.pgDB)` to match existing codebase pattern (see line 115 of the same file). The `OpenCodeSQLiteHarvester` struct has NO `queries` field — do NOT add one.

Current (lines 141-164, simplified):
```go
worktree := session.Worktree
if worktree == "" {
    // Fallback for orphan sessions
    worktree = h.dbPath
}
workspaceHash, err := storage.WorkspaceHash(worktree)
if err != nil {
    return err
}

// Existing auto-registration block (lines ~155-163) — REMOVE THIS:
// q := sqlc.New(h.pgDB)
// _, err = q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
//     Hash:     workspaceHash,
//     RootPath: worktree,
//     ...
// })
// if err != nil { return err }

// ... proceeds to upsert document under workspaceHash
```

After:
```go
if session.Worktree == "" {
    h.logger.Warn().
        Str("session_id", session.ID).
        Msg("opencode harvest: skipping orphan session (no worktree)")
    return nil  // skip, do not error — continue to next session
}
workspaceHash, err := storage.WorkspaceHash(session.Worktree)
if err != nil {
    return err
}

// Verify registered before proceeding (no auto-registration)
q := sqlc.New(h.pgDB)
if _, err := q.GetWorkspaceByHash(ctx, workspaceHash); err != nil {
    if errors.Is(err, sql.ErrNoRows) {
        h.logger.Warn().
            Str("session_id", session.ID).
            Str("worktree", session.Worktree).
            Str("workspace_hash", workspaceHash).
            Msg("opencode harvest: skipping session for unregistered workspace; run nano-brain init --root=<worktree> to register")
        return nil
    }
    return err
}

// REMOVED: UpsertWorkspace call. Workspaces must be pre-registered.
// ... proceeds to upsert document
```

**Cache optimization (nice-to-have):** The harvester has a `wsCache` (line 116) caching worktree→hash. Combine the registration check with this cache: if worktree is in wsCache, it was already verified registered for this harvest cycle.

**Test:** Modify `internal/harvest/opencode_sqlite_integration_test.go` to add cases for:
- Session with empty worktree → skipped, no docs created, no workspace auto-registered
- Session with worktree pointing to unregistered path → skipped, no docs created, no workspace auto-registered
- Session with worktree matching registered workspace → harvested as before
- Test fixtures: create temporary SQLite DB matching OpenCode schema (`sessions`, `session_projects` tables) with the three session variants above. Reference existing fixtures in `internal/harvest/opencode_sqlite_integration_test.go` for the schema.

**LoC added:** ~25 lines + ~60 lines test

### 3.3 `cmd/nano-brain/main.go` + new `cmd/nano-brain/claudecode_init.go`

**Change:** Extract Claude Code harvester init into a testable function, then add workspace registration lookup. The current monolithic `main.go` is untestable without process-level integration tests; extracting the init logic enables unit testing.

**New file** `cmd/nano-brain/claudecode_init.go` exposes:
```go
// initClaudeCodeHarvester returns a configured Claude Code harvester, or nil if
// it should not be started (e.g., disabled, session_dir missing, or workspace not registered).
// Returning (nil, nil) means "don't start, no error".
func initClaudeCodeHarvester(ctx context.Context, cfg ClaudeCodeConfig, db *sql.DB, logger zerolog.Logger) (*harvest.ClaudeCodeHarvester, error) {
    if !cfg.Enabled { return nil, nil }
    if _, err := os.Stat(cfg.SessionDir); os.IsNotExist(err) {
        logger.Warn().Str("session_dir", cfg.SessionDir).
            Msg("claude code session_dir does not exist; harvester disabled")
        return nil, nil
    }
    wsHash, err := storage.WorkspaceHash(cfg.SessionDir)
    if err != nil { return nil, fmt.Errorf("compute workspace hash: %w", err) }

    q := sqlc.New(db)
    if _, err := q.GetWorkspaceByHash(ctx, wsHash); err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            logger.Warn().
                Str("session_dir", cfg.SessionDir).
                Str("computed_workspace_hash", wsHash).
                Msg("claude code session_dir is not a registered workspace; harvester disabled. Run nano-brain init --root=<path> to register.")
            return nil, nil
        }
        logger.Error().Err(err).Msg("claude code workspace lookup failed (DB error); harvester disabled")
        return nil, nil
    }

    return harvest.NewClaudeCodeHarvester(db, logger, cfg.SessionDir, wsHash), nil
}
```

**Then in `main.go`**, replace lines 371-393 with:
```go
if ch, err := initClaudeCodeHarvester(ctx, cfg.Harvester.ClaudeCode, db, logger); err != nil {
    return fmt.Errorf("claude code harvester init: %w", err)
} else if ch != nil {
    if hr == nil {
        hr = harvest.NewRunner(ch, eq, interval, logger)
    } else {
        hr.AddHarvester(ch)
    }
    logger.Info().Str("session_dir", cfg.Harvester.ClaudeCode.SessionDir).
        Dur("interval", interval).Msg("claude code session harvester started")
}
```

**Note:** This was previously inline at lines 371-393. The extraction is the key change for testability.

**Test:** New `cmd/nano-brain/claudecode_init_test.go` covers:
- enabled=false → returns (nil, nil), no error
- enabled=true + session_dir absent → returns (nil, nil), WARN logged
- enabled=true + session_dir present + computed hash unregistered → returns (nil, nil), WARN logged with workspace_hash + "run nano-brain init" guidance
- enabled=true + session_dir present + computed hash registered → returns valid harvester
- DB lookup error → returns (nil, nil), ERROR logged

**LoC added:** ~40 lines (extracted func) + ~80 lines test

### 3.3b `internal/mcp/tools.go` — MCP write path enforcement (new layer)

**Why:** MCP `memory_write` and `memory_update` execute outside the Echo HTTP middleware chain. The `/mcp` and `/sse` transports register the streamable handler directly in `routes.go:73-78` without `workspaceMiddleware` or `workspaceRegisteredMiddleware`. Therefore the HTTP middleware does NOT protect MCP writes — verified by exploration of `internal/mcp/tools.go:485-659` (memory_write calls `UpsertDocument` at line 590 and 623, with only a non-empty workspace check).

**Change:** Inside `registerMemoryWrite` (around line 505), after extracting the `workspace` argument and validating it is non-empty, add a registration lookup BEFORE calling `UpsertDocument`:

```go
// Inside registerMemoryWrite handler, after extracting `workspace` arg:
if workspace == "" {
    return mcp.NewToolResultError("workspace_required: workspace argument is required"), nil
}
if workspace == "all" {
    return mcp.NewToolResultError("workspace_all_not_supported: memory_write does not accept the 'all' workspace"), nil
}

q := sqlc.New(db)
if _, err := q.GetWorkspaceByHash(ctx, workspace); err != nil {
    if errors.Is(err, sql.ErrNoRows) {
        return mcp.NewToolResultError(
            fmt.Sprintf("workspace_not_registered: workspace_hash %q is not registered; use POST /api/v1/init or memory_init to register first", workspace),
        ), nil
    }
    return mcp.NewToolResultError(fmt.Sprintf("workspace_lookup_failed: %v", err)), nil
}

// ... existing UpsertDocument logic
```

The same guard applies to `registerMemoryUpdate` (around line 745), even though it currently only queues reindex requests (it accepts workspace and could be misused to trigger work for an unregistered workspace).

**Test:** New `internal/mcp/tools_security_test.go`:
- memory_write with registered workspace → succeeds, UpsertDocument called
- memory_write with unregistered workspace → returns error with `workspace_not_registered`, UpsertDocument NOT called
- memory_write with `workspace: "all"` → returns error with `workspace_all_not_supported`
- memory_update with unregistered workspace → returns error

**LoC added:** ~30 lines (per tool) × 2 tools = ~60 lines + ~100 lines test

### 3.4 `internal/server/middleware.go`

**Change:** New middleware `workspaceRegisteredMiddleware(db *sql.DB)` that performs `GetWorkspaceByHash` lookup after extracting workspace string. Existing `workspaceMiddleware()` is unchanged. Uses on-demand `sqlc.New(db)` per request — no shared `WorkspaceQuerier` interface (matches existing codebase pattern).

```go
// workspaceRegisteredMiddleware extends workspaceMiddleware with a registration check.
// Use on write endpoints that should reject unregistered workspaces.
// Returns HTTP 400 with error="workspace_not_registered" if the hash is not in workspaces table.
// The special string "all" is rejected (write endpoints do not support cross-workspace writes).
func workspaceRegisteredMiddleware(db *sql.DB) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            workspace, ok := c.Get("workspace").(string)
            if !ok || workspace == "" {
                return c.JSON(http.StatusBadRequest, map[string]string{
                    "error":   "workspace_required",
                    "message": "workspace identifier is required",
                })
            }
            if workspace == "all" {
                return c.JSON(http.StatusBadRequest, map[string]string{
                    "error":   "workspace_all_not_supported",
                    "message": "this endpoint does not support the 'all' workspace scope; provide a specific registered workspace hash",
                })
            }
            q := sqlc.New(db)
            if _, err := q.GetWorkspaceByHash(c.Request().Context(), workspace); err != nil {
                if errors.Is(err, sql.ErrNoRows) {
                    return c.JSON(http.StatusBadRequest, map[string]string{
                        "error":   "workspace_not_registered",
                        "message": fmt.Sprintf("workspace_hash %q is not registered; use POST /api/v1/init to register it first", workspace),
                    })
                }
                return c.JSON(http.StatusInternalServerError, map[string]string{
                    "error":   "workspace_lookup_failed",
                    "message": err.Error(),
                })
            }
            return next(c)
        }
    }
}
```

No `WorkspaceQuerier` interface needed — middleware constructs `sqlc.New(db)` on demand per request (consistent with `internal/harvest/opencode_sqlite.go:115`). Cost: one PG round-trip per write request (~1-2ms on localhost). No caching layer for v1.

**Applied in `routes.go`** to write endpoints:
```go
write := data.Group("", workspaceRegisteredMiddleware(db))
write.POST("/summarize", handlers.TriggerSummarize(...))
write.POST("/write", handlers.WriteDocument(...))
write.POST("/embed", handlers.TriggerEmbed(...))
write.POST("/reindex", handlers.TriggerReindex(...))
write.POST("/update", handlers.TriggerUpdate(...))
```

**Test:** Extend `middleware_test.go` with cases:
- Registered hash → next handler called
- Unregistered hash → HTTP 400, error="workspace_not_registered"
- `"all"` → HTTP 400, error="workspace_all_not_supported"
- DB error → HTTP 500

**LoC added:** ~45 lines + ~80 lines test

### 3.5 `migrations/00011_add_fk_documents_workspace.sql`

**New file:**
```sql
-- IMPORTANT: Before applying this migration, run:
--   nano-brain cleanup-orphan-workspaces
-- This migration adds FK constraints. If orphans exist, this migration WILL FAIL
-- with a PostgreSQL foreign key violation. Run cleanup first.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE documents
  ADD CONSTRAINT fk_documents_workspace
  FOREIGN KEY (workspace_hash) REFERENCES workspaces(hash) ON DELETE CASCADE;

ALTER TABLE chunks
  ADD CONSTRAINT fk_chunks_workspace
  FOREIGN KEY (workspace_hash) REFERENCES workspaces(hash) ON DELETE CASCADE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE documents DROP CONSTRAINT IF EXISTS fk_documents_workspace;
ALTER TABLE chunks DROP CONSTRAINT IF EXISTS fk_chunks_workspace;
-- +goose StatementEnd
```

**Test:** Integration test in `migrations/migrations_test.go` (or equivalent):
- Up migration on a clean schema → succeeds
- Up migration on a schema with orphans → fails with clear error
- Down migration → drops constraints cleanly

**LoC added:** ~25 lines migration + ~50 lines test

### 3.6 `cmd/nano-brain/cmd_cleanup_orphan_workspaces.go` (new file)

**Pattern reference:** Follow `cmd/nano-brain/cmd_cleanup_stale_raw.go` for command structure (the codebase uses a custom switch-based dispatcher, NOT Cobra).

**New command:**
```go
// cleanup-orphan-workspaces deletes documents/chunks under workspace_hash values
// not present in the workspaces table. Required before migration 00011.
//
// Usage:
//   nano-brain cleanup-orphan-workspaces [--dry-run]
//
// Output (dry-run):
//   Found N documents under M unregistered workspace_hash values:
//     <hash> → <count> docs
//     ...
//   Run without --dry-run to apply.
//
// Output (apply):
//   Deleted N documents + N chunks across M unregistered workspaces.
//   (Embeddings cascade-deleted via existing chunks(id) → embeddings FK.)
```

Implementation outline (matches existing codebase pattern — plain `args []string`, no Cobra):
```go
func runCleanupOrphanWorkspacesCmd(args []string) error {
    var dryRun bool
    fs := flag.NewFlagSet("cleanup-orphan-workspaces", flag.ExitOnError)
    fs.BoolVar(&dryRun, "dry-run", false, "list orphans without deleting")
    if err := fs.Parse(args); err != nil { return err }

    // Pre-flight: warn if server is running (race safety)
    if resp, err := http.Get("http://localhost:3100/health"); err == nil && resp.StatusCode == 200 {
        fmt.Fprintln(os.Stderr, "WARNING: nano-brain server appears to be running on :3100. Concurrent harvests could re-create orphans. Stop the server before cleanup.")
        resp.Body.Close()
    }

    cfg := loadConfig()
    db := openDB(cfg)
    defer db.Close()

    // List orphan workspace_hash values + counts
    rows, err := db.Query(`
        SELECT d.workspace_hash, COUNT(*) AS doc_count
        FROM documents d
        LEFT JOIN workspaces w ON d.workspace_hash = w.hash
        WHERE w.hash IS NULL
        GROUP BY d.workspace_hash
        ORDER BY doc_count DESC`)
    if err != nil { return err }
    defer rows.Close()

    var orphans []struct{ hash string; count int }
    total := 0
    for rows.Next() {
        var o struct{ hash string; count int }
        rows.Scan(&o.hash, &o.count)
        orphans = append(orphans, o)
        total += o.count
    }

    if total == 0 {
        fmt.Println("No orphan documents found. DB is clean.")
        return nil
    }

    fmt.Printf("Found %d documents under %d unregistered workspace_hash values:\n", total, len(orphans))
    for _, o := range orphans {
        fmt.Printf("  %s → %d docs\n", o.hash, o.count)
    }

    if dryRun {
        fmt.Println("\nRun without --dry-run to apply.")
        return nil
    }

    // Delete in transaction (cascades to chunks/embeddings via existing app-level cleanup)
    tx, err := db.BeginTx(ctx, nil)
    if err != nil { return err }
    defer tx.Rollback()

    res, err := tx.ExecContext(ctx, `
        DELETE FROM documents
        WHERE workspace_hash IN (
            SELECT d.workspace_hash FROM documents d
            LEFT JOIN workspaces w ON d.workspace_hash = w.hash
            WHERE w.hash IS NULL
        )`)
    if err != nil { return err }
    deletedDocs, _ := res.RowsAffected()

    // Same for chunks
    res, err = tx.ExecContext(ctx, `
        DELETE FROM chunks
        WHERE workspace_hash IN (
            SELECT c.workspace_hash FROM chunks c
            LEFT JOIN workspaces w ON c.workspace_hash = w.hash
            WHERE w.hash IS NULL
        )`)
    if err != nil { return err }
    deletedChunks, _ := res.RowsAffected()

    if err := tx.Commit(); err != nil { return err }

    fmt.Printf("Deleted %d documents + %d chunks across %d unregistered workspaces.\n",
        deletedDocs, deletedChunks, len(orphans))
    return nil
}
```

**Test:** New `cleanup_orphan_workspaces_test.go` with cases:
- Empty DB → reports "No orphan documents found"
- DB with orphans + dry-run → reports counts, no DB changes
- DB with orphans + apply → deletes orphans, leaves registered workspaces untouched

**LoC added:** ~80 lines command + ~60 lines test

## 4. Data Flow After Fix

```
EXTERNAL API CALLS
  ↓
HTTP middleware (workspaceMiddleware)
  ├─ Extract workspace string from body/query
  └─ Pass to next
       ↓
HTTP middleware (workspaceRegisteredMiddleware) — write routes only
  ├─ Validate workspace != "all"
  ├─ GetWorkspaceByHash(workspace) → reject if not found
  └─ Pass to next
       ↓
Handler (e.g., TriggerSummarize)
  └─ Calls SummarizeAndPersist → HarvestSummarizer
       ↓
HarvestSummarizer.SummarizeAndPersist
  └─ Calls Pipeline.Summarize + Persister.Save
       ↓
Persister.Save
  ├─ GetWorkspaceByHash(meta.WorkspaceHash) → reject if not found  ← defense-in-depth
  ├─ SHA-256 idempotency check
  └─ Upsert document (FK constraint enforced by PostgreSQL)        ← last-line defense

HARVESTER PATHS (no HTTP entry)
  ↓
Harvester init (main.go)
  ├─ Compute workspace hash from session_dir/worktree
  ├─ GetWorkspaceByHash(hash) → skip harvester if not found        ← gate at init
  └─ NewXxxHarvester(...)
       ↓
Harvester.Harvest
  ├─ For OpenCode: skip session if worktree empty OR unregistered  ← gate per session
  └─ Call summarizer.SummarizeAndPersist
       ↓
(same as Persister.Save path above — defense-in-depth + FK)
```

## 5. Performance Impact

Each new validation adds 1 PostgreSQL query per call:
- Middleware: 1 query per HTTP request (cached during request)
- Persister: 1 query per Save() call
- Harvester init: 1 query per harvester startup (negligible, runs once)
- FK constraint: 0 additional queries; PG enforces inline during INSERT/UPDATE

For HTTP requests, this adds 1 round-trip to PG. With PG on `host.docker.internal:5432` (local), latency is ~1-2ms. Acceptable.

For high-volume harvest paths, batch validation could be added later if needed (out of scope here).

## 6. Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Migration 00011 fails on prod with orphans | Cleanup command + clear migration comment + release notes |
| Persister now rejects formerly-valid orphans → harvest tasks fail | Harvester skips orphans before calling persister; persister rejection becomes unreachable in normal flow |
| Middleware overhead on hot endpoints | ~1ms PG lookup; acceptable for write endpoints |
| FK CASCADE deletes unexpected data when workspace removed | Already the current app-level behavior; FK just formalizes it |
| Test workspaces (alpha/beta/perm_*) get cleaned up | Cleanup only targets orphan rows where `workspaces.hash IS NULL`; registered test workspaces are untouched |
| Cleanup command run during active harvest causes inconsistency | Document: stop server before running cleanup |

## 7. Rollback Plan

If the change breaks production:

1. **Code rollback** — revert PR commit. Persister/harvester/middleware behavior reverts to current (leaky but functional).
2. **Migration rollback** — `goose down` to revert migration 00011. FK constraints dropped. App resumes pre-fix behavior.
3. **Data rollback** — orphans deleted by cleanup command CANNOT be restored (deletions are permanent). Mitigation: dry-run output should be saved to a file before applying, so operators know what was lost.

## 8. Resolved Decisions (from Metis + Oracle deep-design)

1. **Telemetry metric for rejected-workspace events** — DEFERRED. WARN logs are sufficient for v1 (localhost-only single-operator system). Follow-up if log volume warrants it.

2. **Auto-cleanup in `db:migrate`** — REJECTED. Deleting data must be explicit operator action. Migration order documented: STOP → CLEANUP → MIGRATE → START NEW.

3. **`"all"` rejection on write endpoints** — DECIDED YES. All 5 write endpoints reject `"all"`. Verification done: grepped codebase for `workspace: "all"` usage on write paths — no existing client/tool sends this. The MCP `memory_write` also rejects it (per §3.3b).

4. **`WorkspaceQuerier` interface** — REJECTED. Each consumer uses on-demand `sqlc.New(db)` to match existing codebase pattern (`internal/harvest/opencode_sqlite.go:115`). Avoids circular import risk and unnecessary abstraction.

5. **Cobra command framework** — REJECTED. Cleanup command uses plain `func runCleanupOrphanWorkspacesCmd(args []string) error` to match existing dispatcher (`cmd_cleanup_stale_raw.go`).

6. **MCP write path coverage** — ADDED. New §3.3b validates workspace registration inside MCP tool handlers. MCP bypasses HTTP middleware (verified at `routes.go:73-78`).

7. **Auto-registration removal in OpenCode harvester** — DECIDED YES. The existing `UpsertWorkspace` call in `opencode_sqlite.go:155-163` is removed (per ADR-3). Breaking change documented in release notes.

8. **Cleanup `--output-json` flag** — DEFERRED. Operators MUST run `--dry-run` and redirect output as backup. First-class JSON export deferred to follow-up.

## 9. Follow-ups (Not in This PR)

- **MCP `memory_write` shared `init` tool** — currently MCP cannot register workspaces; operators must use HTTP/CLI. Consider adding `memory_init` tool in follow-up.
- **Error code shared constants** — extract to `internal/server/errors.go` in style cleanup PR.
- **WARN vs ERROR log level distinction** — refine per Oracle finding 7.2 in style cleanup PR.
- **Cache registration check in wsCache** — minor perf optimization per Oracle finding 3.1.
- **Simplify `DELETE /api/v1/workspaces/:hash` handler** — remove redundant explicit document delete now that FK CASCADE handles it (Oracle finding 5.1).
- **Auto-cleanup `--output-json`** — first-class backup flag for cleanup command.
- **Telemetry counter** — track rejected-workspace events (Oracle finding 11.1).
- **RRI-T Tier 3 regression** — 30 deferred test cases, runs post-merge.
