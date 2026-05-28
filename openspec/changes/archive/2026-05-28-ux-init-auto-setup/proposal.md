## Why

Two UX gaps exist in the current nano-brain server + CLI:

1. **`init --force` does not trigger harvest or reindex** after resetting the workspace. Users must manually run `nano-brain harvest` and `nano-brain reindex` — steps that belong to the init flow.
2. **`migration_version` in `/api/status` is hardcoded to `1`** regardless of actual DB state, making the status endpoint misleading and hindering diagnosis of migration issues.

The server already auto-runs migrations on startup (`RunMigrations` line 203 in `main.go`), so the migration story is correct — the status reporting is just wrong.

## What Changes

- **`init --force`**: After reset + register, the CLI calls `triggerInitBackground()` (same as the non-force path). This triggers reindex + harvest automatically.
- **`/api/status` → `migration_version`**: Query `goose.GetDBVersionContext()` at request time (or cache it at server startup) and return the real version instead of the hardcoded `1`.
- No new flags, no new endpoints, no schema changes.

## Capabilities

### Modified Capabilities

- `init-force-full-reset`: `init --force` now completes the full init lifecycle (reset → register → reindex → harvest) in one command.
- `status-migration-version`: `/api/status` returns the real goose migration version from the database.

## Impact

- **`cmd/nano-brain/commands.go`**: Add `triggerInitBackground(result.WorkspaceHash, root)` call inside the `forceFlag` branch after the register POST.
- **`internal/server/handlers/health.go`**: Inject real migration version — query via `storage.GetMigrationVersion()` (new thin wrapper) or pass version in at handler construction time.
- **`internal/storage/migrate.go`**: Add exported `GetCurrentVersion(ctx, pool)` helper returning `int64`.
- No API contract changes. No migration needed.
