## Why

The `/api/v1/reindex-cfg` endpoint walks the workspace filesystem to re-extract CFGs for all JS/TS files. However, it doesn't respect the workspace's `.nano-brainignore` file — it only uses hardcoded `defaultExcludeDirs`. This causes it to waste time walking irrelevant directories like `docker-data`, `data`, vendor directories, and other project-specific ignores that are already configured in `.nano-brainignore`.

## What Changes

- `/api/v1/reindex-cfg` will load and respect the workspace's `.nano-brainignore` file (matching how the watcher processes ignores)
- Remove hardcoded `defaultExcludeDirs` duplication — reuse existing `fileFilter` logic from `internal/watcher/filter.go`
- Walking progress logs will correctly show paths being skipped due to ignore rules
- **Watcher fix**: `scanCollection` will load nested `.nano-brainignore` per subdirectory during walk (currently only loads nested `.gitignore`)
- **incrementalExtract fix**: add filter check to skip now-ignored files during incremental reindex
- **Bug fix**: `shouldSkip` uses `filepath.Rel(".", path)` (relative to CWD) instead of `filepath.Rel(codeRoot, path)` — incorrect when CWD ≠ codeRoot

## Capabilities

### New Capabilities

- `reindex-cfg-ignores`: The `reindex-cfg` endpoint loads the workspace's `.nano-brainignore` at the collection root and applies it during filesystem walk, skipping ignored paths and directories just like the watcher does.

### Modified Capabilities

- (none)

## Impact

- `internal/server/handlers/reindex_cfg.go` — use `fileFilter` from watcher instead of local ignore logic
- `internal/watcher/filter.go` — may need to export `NewFileFilter` or `LoadGlobalIgnore` for use by handler
- `openspec/changes/reindex-cfg-respect-nano-brainignore/specs/reindex-cfg-ignores/spec.md` — new spec for ignore behavior
