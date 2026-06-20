## Context

The nano-brain file watcher indexes code by walking the workspace directory and extracting edges (function calls, HTTP routes, integrations) from source files. These edges are stored in the `graph_edges` PostgreSQL table with a `source_file` column.

Two extraction paths exist:
1. **Edge extraction** (`extractAndUpsertEdges`): Receives absolute paths from `filepath.WalkDir`, passes them directly to extractors. Extractors store absolute paths as `source_file`.
2. **CFG extraction** (`extractAndUpsertCFGs`): Receives absolute paths, but normalizes to relative via `filepath.Rel(col.dirPath, filePath)` before storing.

The `deriveServiceName` function in `internal/flow/builder.go` assumes relative paths when extracting the service name from `source_file`. For absolute paths like `/Users/tamlh/projects/tradeit-backend/server/trade.js`, `SplitN(path, "/", 2)` returns `["", "Users/tamlh/..."]` — the first component is empty, so the function falls back to `"Backend"`.

## Goals / Non-Goals

**Goals:**
- Fix edge extraction to store workspace-relative paths (matching CFG behavior)
- Make `deriveServiceName` defensive against absolute paths as defense-in-depth
- Ensure existing workspaces can be re-indexed to fix stale data

**Non-Goals:**
- Changing the `graph_edges` table schema
- Modifying the flow handler API response format
- Refactoring the entire watcher path handling

## Decisions

### Decision 1: Normalize in watcher (root fix)

**Choice**: Add `filepath.Rel(col.dirPath, filePath)` in `extractAndUpsertEdges`, matching `extractAndUpsertCFGs` at line 835.

**Rationale**: This is the root cause fix. CFG extraction already does this correctly. Edge extraction should follow the same pattern.

**Alternative considered**: Normalize in each extractor individually — rejected because it duplicates logic across 13+ extractors and is error-prone.

### Decision 2: Defense-in-depth in deriveServiceName

**Choice**: Strip leading `/` and any absolute path prefix before splitting on `/`.

**Rationale**: Even after the watcher fix, old data or other code paths might still produce absolute paths. The function should handle both gracefully.

**Alternative considered**: Only fix the watcher — rejected because it leaves the function fragile to future regressions.

### Decision 3: Reindex via existing endpoint

**Choice**: Use `POST /api/v1/reindex-cfg` with `wipe: true` to re-extract all edges with correct paths.

**Rationale**: The endpoint already exists and handles full re-indexing. No new migration needed.

## Risks / Trade-offs

- **[Risk] Reindex takes time on large workspaces** → Mitigation: The endpoint is async and logs progress. 1747 files took ~19s in testing.
- **[Risk] Old absolute paths in DB until reindex** → Mitigation: Defense-in-depth in `deriveServiceName` handles absolute paths gracefully.
- **[Trade-off] Normalizing in watcher vs extractors** → Watcher normalization is centralized (one place to maintain) but means extractors receive relative paths (which is actually cleaner).
