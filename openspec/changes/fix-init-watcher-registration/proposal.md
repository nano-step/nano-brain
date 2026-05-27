## Why

`POST /api/v1/init` registers a workspace and its default collections in the DB, but does **not** register them with the running `watcher.Watcher` instance. The watcher only loads collections from DB at server startup. Any workspace registered after the server starts is invisible to the watcher until the server restarts.

This forces users to restart the server after every `nano-brain init`, which breaks the expected UX of `init --force` as a single self-contained setup command.

## What Changes

- `InitWorkspace` handler accepts `fw *watcher.Watcher` and `watcherCfg config.WatcherConfig` (same pattern as `AddCollection` handler).
- After the DB transaction commits, the handler iterates the default collections and calls `fw.WatchWithFilter(...)` for each — mirroring the startup watcher registration in `main.go`.
- `routes.go` passes `s.watcher` and `s.currentConfig().Watcher` to `InitWorkspace`.
- Test files updated to pass `nil` watcher (safe — watcher.Watch guards on nil).

## Capabilities

### Modified Capabilities

- `init-workspace`: `POST /api/v1/init` now registers new collections with the live watcher immediately, no server restart required.

## Impact

- **`internal/server/handlers/workspace.go`**: `InitWorkspace` signature gains `fw *watcher.Watcher`, `watcherCfg config.WatcherConfig`; post-commit loop calls `fw.WatchWithFilter`
- **`internal/server/routes.go`**: Pass `s.watcher`, `s.currentConfig().Watcher` to `InitWorkspace`
- **`internal/server/handlers/workspace_test.go`** / **`workspace_integration_test.go`**: Pass `nil, config.WatcherConfig{}` for new params
- No schema changes, no API contract changes, no new endpoints
