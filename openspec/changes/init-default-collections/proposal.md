## Why

After `nano-brain init --root <path>`, the watcher never indexes anything because no collection pointing to `<path>` is registered. The `initWorkspace` helper already creates `memory` and `sessions` collections with hardcoded `~/.nano-brain/` paths — but those paths often don't exist on disk, so the watcher skips them at startup. The user's project directory is never watched or indexed. Issue #161.

## What Changes

- `initWorkspace` in `internal/server/handlers/workspace.go` registers a third collection named `code` pointing to `absPath` (the workspace root), using glob `**/*` and `update_mode = auto`.
- The `code` collection is upserted after `memory` and `sessions` in the same transaction, so init remains atomic.
- The `InitWorkspace` HTTP handler already calls `fw.Watch` indirectly through the collection handler; the watcher will pick up the new collection on next startup via the existing seeding loop in `main.go` (lines 220–237). No watcher-live-registration is needed from the HTTP handler — the `AddCollection` endpoint does live registration but `InitWorkspace` does not currently expose a `fw` dependency.
- `initResponse` is unchanged; the response body is not modified.

## Capabilities

### New Capabilities

- `init-default-code-collection`: On `POST /api/v1/init`, a `code` collection is created (or upserted) pointing to the workspace root path, enabling the watcher to index the user's project files immediately after the next server start.

### Modified Capabilities

- (none — no existing spec-level behavior changes)

## Impact

- **`internal/server/handlers/workspace.go`**: `initWorkspace` adds one `UpsertCollection` call for the `code` collection.
- **`internal/server/handlers/workspace_test.go`**: tests for `initWorkspace` must assert the `code` collection is created.
- No DB schema change (collections table already exists).
- No API contract change (`initResponse` JSON shape unchanged).
- No CLI changes needed.
- No watcher interface changes.

## References

- Issue #161
- Existing collections: `memory` (`~/.nano-brain/memory/`), `sessions` (`~/.nano-brain/sessions/`) — created in same `initWorkspace` function
- Watcher seeding loop: `cmd/nano-brain/main.go` lines 220–237
- `AddCollection` handler for reference on live watcher registration: `internal/server/handlers/collection.go:66`
