# Design: Fix Summary Workspace-Registration Leaks

## 1. Problem Statement

Currently, the summary feature trusts the `workspace_hash` field across multiple layers without verifying it corresponds to a row in the `workspaces` table. This creates orphan documents that:

- Pollute cross-workspace queries (`workspace: "all"`)
- Bypass workspace isolation (multi-tenant correctness)
- Cannot be cleaned up via the standard `DELETE /api/v1/workspaces/:hash` endpoint
- Survive across server restarts indefinitely

The fix must close the leak at **all entry points** because any unfixed path is exploitable.

## 2. Architecture Decision Records

### ADR-1: Defense-in-depth at 4 layers, not 1

**Decision:** Validate workspace registration at HTTP middleware, harvester init, Persister.Save, AND PostgreSQL FK constraint.

**Rationale:** A single validation point is fragile. Future code paths added by other developers will likely skip a centralized check. Layering ensures:
- HTTP layer catches external API abuse early (cheap rejection, no DB hit beyond the registration lookup)
- Harvester layer catches configuration errors at startup (operator sees warning immediately)
- Persister layer catches internal API misuse (defense against future code paths)
- DB layer catches everything else (last line of defense; cannot be bypassed by application code)

**Alternative considered:** Centralize all validation in middleware only.
- Rejected because: bypasses harvester paths (harvester does not go through HTTP); creates a "single point of failure" anti-pattern.

**Alternative considered:** FK constraint only.
- Rejected because: produces opaque database errors at the worst possible time (mid-transaction during summary persist). Application-level errors at the entry point are much better UX.

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

### ADR-3: OpenCode orphan sessions are SKIPPED, not auto-registered

**Decision:** When an OpenCode session has empty `worktree` OR its `WorkspaceHash(worktree)` is not registered, the harvester logs WARN and skips the session entirely. No fallback workspace is created.

**Rationale:** Auto-registering would silently extend the trust boundary. Skipping is conservative and visible to operators via logs. Operators who actually want a workspace registered must use `POST /api/v1/init` explicitly.

**Migration implication:** Production deployments may have orphan sessions in OpenCode SQLite DBs that previously got harvested under the fallback hash. After this change, those sessions will be skipped. The cleanup command will remove the orphan summaries already in the nano-brain DB.

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

**Change:** Skip orphan sessions instead of falling back to `WorkspaceHash(dbPath)`.

Current (lines 141-164):
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
// ... proceeds to upsert under workspaceHash
```

After:
```go
if session.Worktree == "" {
    h.logger.Warn().
        Str("session_id", session.ID).
        Msg("opencode harvest: skipping orphan session (no worktree)")
    return nil  // skip, do not error
}
workspaceHash, err := storage.WorkspaceHash(session.Worktree)
if err != nil {
    return err
}

// Verify registered before proceeding
if _, err := h.queries.GetWorkspaceByHash(ctx, workspaceHash); err != nil {
    if errors.Is(err, sql.ErrNoRows) {
        h.logger.Warn().
            Str("session_id", session.ID).
            Str("worktree", session.Worktree).
            Str("workspace_hash", workspaceHash).
            Msg("opencode harvest: skipping session for unregistered workspace")
        return nil
    }
    return err
}
// ... proceeds to upsert
```

**Test:** Modify `internal/harvest/opencode_sqlite_integration_test.go` to add cases for:
- Session with empty worktree → skipped, no docs created
- Session with worktree pointing to unregistered path → skipped, no docs created
- Session with worktree matching registered workspace → harvested as before

**LoC added:** ~20 lines + ~40 lines test

### 3.3 `cmd/nano-brain/main.go`

**Change:** Add workspace registration lookup before initializing Claude Code harvester.

Current (lines 371-393):
```go
if cfg.Harvester.ClaudeCode.Enabled {
    if _, err := os.Stat(cfg.Harvester.ClaudeCode.SessionDir); os.IsNotExist(err) {
        logger.Warn().Msg("session_dir does not exist, skipping")
    } else {
        wsHash, err := storage.WorkspaceHash(cfg.Harvester.ClaudeCode.SessionDir)
        if err != nil {
            logger.Warn().Err(err).Msg("failed to compute workspace hash")
        } else {
            ch := harvest.NewClaudeCodeHarvester(...)
            // ... add to runner
        }
    }
}
```

After:
```go
if cfg.Harvester.ClaudeCode.Enabled {
    if _, err := os.Stat(cfg.Harvester.ClaudeCode.SessionDir); os.IsNotExist(err) {
        logger.Warn().Msg("session_dir does not exist, skipping")
    } else {
        wsHash, err := storage.WorkspaceHash(cfg.Harvester.ClaudeCode.SessionDir)
        if err != nil {
            logger.Warn().Err(err).Msg("failed to compute workspace hash")
        } else {
            // Verify registered before harvesting
            q := sqlc.New(db)
            if _, err := q.GetWorkspaceByHash(ctx, wsHash); err != nil {
                if errors.Is(err, sql.ErrNoRows) {
                    logger.Warn().
                        Str("session_dir", cfg.Harvester.ClaudeCode.SessionDir).
                        Str("computed_workspace_hash", wsHash).
                        Msg("claude code session_dir is not a registered workspace; harvester disabled. Run nano-brain init --root=<path> to register.")
                } else {
                    logger.Warn().Err(err).Msg("claude code workspace lookup failed; harvester disabled")
                }
            } else {
                ch := harvest.NewClaudeCodeHarvester(...)
                // ... add to runner
            }
        }
    }
}
```

**Test:** Add to `cmd/nano-brain/main_test.go` (or wherever main init is tested) — assert that unregistered session_dir results in no harvester added to runner.

**LoC added:** ~15 lines + ~30 lines test

### 3.4 `internal/server/middleware.go`

**Change:** New middleware `workspaceRegisteredMiddleware(queries)` that performs `GetWorkspaceByHash` lookup after extracting workspace string. Existing `workspaceMiddleware()` is unchanged.

```go
// workspaceRegisteredMiddleware extends workspaceMiddleware with a registration check.
// Use on write endpoints that should reject unregistered workspaces.
// Returns HTTP 400 with error="workspace_not_registered" if the hash is not in workspaces table.
// The special string "all" is rejected (write endpoints do not support cross-workspace writes).
func workspaceRegisteredMiddleware(q WorkspaceQuerier) echo.MiddlewareFunc {
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

type WorkspaceQuerier interface {
    GetWorkspaceByHash(ctx context.Context, hash string) (sqlc.Workspace, error)
}
```

**Applied in `routes.go`** to write endpoints:
```go
write := data.Group("", workspaceRegisteredMiddleware(queries))
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

### 3.6 `cmd/nano-brain/cleanup_orphan_workspaces.go` (new file)

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
```

Implementation outline:
```go
func runCleanupOrphanWorkspacesCmd(cmd *cobra.Command, args []string) error {
    dryRun, _ := cmd.Flags().GetBool("dry-run")
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

## 8. Open Questions

1. **Should we add a metric/counter for rejected-unregistered-workspace events?**
   - Lean YES — useful for spotting misconfigured harvesters or attack attempts.
   - Deferred to follow-up if telemetry package needs extension.

2. **Should the cleanup command be invoked automatically by `nano-brain db:migrate`?**
   - Lean NO — deleting data should be explicit operator action.
   - Document in release notes that migration order is: cleanup → migrate.

3. **Should the `"all"` rejection in `workspaceRegisteredMiddleware` apply to all write endpoints, or only summarize?**
   - Lean ALL — write semantics with `"all"` are ambiguous and likely buggy.
   - Could be split into a follow-up if any existing endpoint relies on it.

These are flagged for Metis + Oracle deep-design to confirm.
