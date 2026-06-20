## Why

The `deriveServiceName` function in `internal/flow/builder.go` always returns `"Backend"` (the fallback) instead of the actual service name (e.g., `"tradeit-backend"`). This happens because the file watcher passes **absolute** file paths to edge extractors, but `deriveServiceName` assumes **relative** paths when splitting on `/`.

Root cause: `filepath.WalkDir` returns absolute paths like `/Users/tamlh/projects/tradeit-backend/server/trade.js`. The extractors store this as `source_file`. When `deriveServiceName` does `strings.SplitN(path, "/", 2)`, the first component is `""` (empty string before the leading `/`), so no service name is derived.

Contrast: CFG extraction (`extractAndUpsertCFGs`) already normalizes to relative paths via `filepath.Rel(col.dirPath, filePath)` at `watcher.go:835`. Edge extraction does not.

## What Changes

- **Fix watcher edge extraction**: Normalize file paths to workspace-relative before passing to extractors (matching CFG extraction behavior)
- **Defense-in-depth in deriveServiceName**: Strip leading `/` and absolute path prefixes before extracting service name
- **Reindex**: Existing edges in DB have absolute paths — need reindex to populate with relative paths

## Capabilities

### New Capabilities

_(none — this is a bug fix)_

### Modified Capabilities

- `flow-sequence-diagram`: Service name derivation now works correctly with relative paths

## Impact

- **Files**: `internal/watcher/watcher.go` (edge extraction path), `internal/flow/builder.go` (`deriveServiceName`)
- **API**: No contract change — sequence diagram response improves (correct service name)
- **Data**: Existing `graph_edges.source_file` values are absolute paths; reindex needed to fix
- **Breaking**: No — fallback to `"Backend"` still works if paths are somehow empty
