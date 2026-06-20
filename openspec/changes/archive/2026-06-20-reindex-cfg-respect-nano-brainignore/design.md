## Context

`/api/v1/reindex-cfg` walks the workspace filesystem to extract CFGs. Currently it has duplicated ignore logic (`defaultExcludeDirs`, `defaultExcludeFiles`, `gitignoreStack`) that doesn't include workspace-specific rules from `.nano-brainignore`.

The watcher already has correct ignore handling in `internal/watcher/filter.go` via `fileFilter` and `gitignoreStack`. This design reuses that logic instead of duplicating it.

## Goals / Non-Goals

**Goals:**
- `/api/v1/reindex-cfg` respects workspace `.nano-brainignore` and `.gitignore` files
- Remove duplicate ignore logic from `reindex_cfg.go`
- Align behavior between watcher and `reindex-cfg`
- Watcher loads nested `.nano-brainignore` per subdirectory during walk (currently only loads nested `.gitignore`)
- `incrementalExtract` skips now-ignored files

**Non-Goals:**
- `walkCollectionFiles` ignore support (reindex.go) — deferred
- Extension whitelist alignment with watcher config — deferred

## Decisions

### D1: Export filter functionality from watcher package

The watcher already has:
- `fileFilter` struct with `shouldSkip(path, isDir)` method
- `gitignoreStack` for nested `.gitignore` support
- `LoadGlobalIgnore(homeDir)` for global ignores
- `defaultExcludeDirs` / `defaultExcludeFiles`

**Approach:** Export these from `internal/watcher/filter.go` so `reindex_cfg.go` can reuse them.

Specifically:
- `fileFilter` is unexported (package-local) → needs `ShouldSkip(absPath string, isDir bool)` exported method
- `defaultExcludeDirs`, `defaultExcludeFiles`, `gitignoreStack` → export or export constructor
- `LoadGlobalIgnore` is already exported

### D2: Create `fileFilter` in handler using exported constructors

In `reindex_cfg.go`:
1. Call `watcher.LoadGlobalIgnore(homeDir)` to get global ignore
2. Call `watcher.NewFileFilter(rootDir, nil, []string{".js",".jsx",".ts",".tsx"}, globalIgnore)` to create filter
3. In walk: call `filter.ShouldSkip(path, d.IsDir())`

### D3: Keep `gitignoreStack` for nested ignores during walk

`filepath.WalkDir` walks directory tree. For each directory, push nested `.gitignore`/`.nano-brainignore` onto stack, pop when ascending.

This mirrors exactly what `scanCollection` does in watcher.

### D4: Fix watcher to load nested `.nano-brainignore` during walk

Currently `scanCollection` (watcher.go:437-444) only loads nested `.gitignore` files per subdirectory. Nested `.nano-brainignore` files are NOT loaded — only the root-level one via `newFileFilter`.

**Fix:** In `scanCollection`, after loading nested `.gitignore`, also check for `.nano-brainignore` and push it onto the stack. This aligns the watcher with reindex-cfg behavior.

### D5: Add filter check to `incrementalExtract`

Currently `incrementalExtract` iterates all previously-indexed code documents without checking ignore rules. If a file was indexed before `.nano-brainignore` was updated, it will still be re-extracted.

**Fix:** In `incrementalExtract`, create a `FileFilter` and call `ShouldSkip` on each document path. Skip documents that match ignore rules.

### D6: Fix `shouldSkip` base path bug

`reindex_cfg.go` line 111: `filepath.Rel(".", path)` computes relative-to-CWD, not relative-to-`codeRoot`. When the server's CWD differs from `codeRoot`, default exclude dir checks produce incorrect results.

**Fix:** Pass `codeRoot` to `ShouldSkip` (or make it a field on `FileFilter`). The watcher's `fileFilter.shouldSkip` correctly uses `filepath.Rel(f.rootDir, absPath)`.

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| Exporting watcher internals couples handler to watcher | Acceptable — both are internal packages, co-deploy |
| Global ignore file path resolution | Pass `os.UserHomeDir()` at handler init time |
| Large workspaces walk slowly | Progress logging already added; per-file skip is O(1) map lookup |
| Watcher change affects live indexing behavior | Nested `.nano-brainignore` support is additive — files that were indexed before stay indexed |
| incrementalExtract skips now-ignored files | Valid behavioral change — previously-indexed files that are now ignored should be skipped |

## Open Questions

- None remaining — all resolved by user.
