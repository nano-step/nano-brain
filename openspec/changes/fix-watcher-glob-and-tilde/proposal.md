## Why

Two bugs prevent the watcher from indexing files:

**Bug 1 — filepath.Glob does not support `**`**
`scanCollection` in `watcher.go` builds the scan pattern as `filepath.Join(col.dirPath, col.globPattern)` → `"/path/to/project/**/*"` and calls `filepath.Glob()`. Go's stdlib `filepath.Glob` does NOT support `**` (recursive glob). It only matches one level deep, so ~90% of codebase files are never discovered.

**Bug 2 — Tilde `~` not expanded in collection paths**
`initWorkspace` stores `"~/.nano-brain/memory/"` and `"~/.nano-brain/sessions/"` literally in the DB. `filepath.Abs("~/.nano-brain/memory/")` does NOT expand tilde — it produces `"<cwd>/~/.nano-brain/memory/"`, a non-existent path. Both watcher attachment and file scanning fail silently for these collections.

## What Changes

- **`watcher.go` `scanCollection`**: Replace `filepath.Glob` with `fs.WalkDir` for recursive traversal. Apply `filter.shouldSkip` at directory level to short-circuit entire subtrees (preserves performance). No new dependencies — uses stdlib `io/fs`.
- **`workspace.go` `initWorkspace`**: Expand tilde before storing `memoryPath`/`sessionsPath` in the DB. Uses `os.UserHomeDir()` — same pattern already used in `internal/config/config.go:expandPaths`.
- **`workspace.go` `InitWorkspace` handler**: The tilde fix in `initWorkspace` means DB paths are now absolute, so the watcher `WatchWithFilter` calls receive correct paths automatically.

## Capabilities

### Fixed Capabilities

- `watcher-recursive-scan`: Watcher now indexes all files in subdirectories, not just 1 level deep.
- `watcher-memory-sessions`: `memory` and `sessions` collections resolve to real home-relative paths and are correctly watched/indexed.

## Impact

- **`internal/watcher/watcher.go`**: `scanCollection` — replace `filepath.Glob` + loop with `fs.WalkDir`
- **`internal/server/handlers/workspace.go`**: `initWorkspace` — expand tilde in `memoryPath`/`sessionsPath`
- **`internal/server/handlers/workspace_test.go`**: Likely no change needed (mocked querier)
- No API changes, no schema changes, no new dependencies
