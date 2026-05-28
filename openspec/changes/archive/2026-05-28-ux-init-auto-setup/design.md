## Fix 1: `init --force` triggers harvest

### Current flow

```
init --force --root <path>
  → POST /api/v1/reset-workspace   (delete docs for workspace)
  → POST /api/v1/init              (register workspace)
  → print result
  → EXIT                           ← missing: triggerInitBackground()
```

### Fixed flow

```
init --force --root <path>
  → POST /api/v1/reset-workspace
  → POST /api/v1/init
  → print result
  → triggerInitBackground(hash, root)   ← ADD THIS
      → POST /api/v1/reindex
      → POST /api/harvest
  → print "Indexing codebase in background..."
```

### Change

In `cmd/nano-brain/commands.go`, the `forceFlag` branch falls through to the register POST and then parses the response — but then `return`s before `triggerInitBackground`. 

**Fix**: move `triggerInitBackground(result.WorkspaceHash, root)` so it runs on both force and non-force paths. The simplest approach: call it unconditionally after parsing the register response (outside the `forceFlag` block — it already does this on the non-force path, the `forceFlag` branch doesn't change that).

Looking at the actual code: the `forceFlag` block only runs before the register POST (reset step). The register POST and `triggerInitBackground` call already exist after it. So the real bug may be that `forceFlag` path exits early — need to verify.

After reading `commands.go` in full: the `forceFlag` block does NOT return early. The register POST and `triggerInitBackground` call happen unconditionally after. This means `triggerInitBackground` **is** called on force path.

**Re-diagnosis**: the actual UX issue is that harvest fails silently with `errors: 1` due to SQLite WAL lock from container. The CLI succeeds but harvest errors are swallowed. This is by design (WARN-only). The real fix is:

- Harvest errors on `init --force` should print a warning to stdout so user knows.
- Harvest running from container against host SQLite will WAL-lock — this is an environment constraint, not a code bug.

**Revised fix for Fix 1**: Print harvest result (including errors) to stdout during init, not just log it silently.

---

## Fix 2: Real `migration_version` in `/api/status`

### Current

```go
MigrationVersion: 1,  // hardcoded in health.go:125
```

### Design

Add `GetCurrentVersion(ctx context.Context, pool *pgxpool.Pool) (int64, error)` to `internal/storage/migrate.go`. This wraps `goose.GetDBVersionContext`.

Pass `pool` to the health handler at construction time (it already has it via `h.queries` — the pool is available via `h.pool` if we add it, or we can query via `sqlc` raw if needed).

**Simplest approach**: store the migration version at server startup after `RunMigrations` completes, pass it as a `int64` field to the handler constructor. This avoids a DB query on every `/api/status` call.

```go
// main.go after RunMigrations
migrationVersion, _ := storage.GetCurrentVersion(ctx, pool)

// pass to handler
h := handlers.NewStatusHandler(..., migrationVersion)
```

```go
// health.go
MigrationVersion: h.migrationVersion,  // field set at construction
```

This is correct because migrations only run at startup — the version won't change while the server is running.

---

## Files Touched

| File | Change |
|---|---|
| `internal/storage/migrate.go` | Add `GetCurrentVersion()` |
| `internal/server/handlers/health.go` | Accept `migrationVersion int64` in constructor; use it in response |
| `cmd/nano-brain/main.go` | Call `GetCurrentVersion` after `RunMigrations`; pass to handler |
| `cmd/nano-brain/commands.go` | Print harvest result/errors to stdout during `triggerInitBackground` |
