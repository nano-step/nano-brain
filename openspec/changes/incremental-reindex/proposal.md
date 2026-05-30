# Incremental Reindex

## Issue
[#158 — feat(indexing): incremental reindex — only re-index changed files](https://github.com/nano-step/nano-brain/issues/158)

## Lane
high-risk — touches data-model (new column/table on `chunks`), indexing core, watcher integration.

## Why
Current `reindex` wipes ALL chunks for a workspace and re-embeds every file. On the nano-brain repo itself this is ~2700+ documents × N chunks each × Ollama embedding round-trip. Slow + wasteful + hits embedding API rate limits.

## Desired Outcome
Repeated `POST /api/v1/reindex` on an unchanged codebase should be effectively a no-op (no embeddings generated, no chunks rewritten). On a partially-changed codebase, only the changed files are re-chunked + re-embedded; unchanged files are preserved.

## Constraints
- Backward compatible: existing chunks/embeddings stay valid.
- No new external dependencies.
- Watcher hot-path (fsnotify event → debounce → upsert) must not regress in latency.
- The full-wipe behavior of `POST /api/v1/reset-workspace` must remain — that's intentionally distinct from reindex.

## Out of Scope
- Cross-workspace dedup (different feature).
- Reindex parallelism / sharding (separate perf work).
- File deletion handling beyond "delete chunks for removed files" (covered here, not extended).

## Acceptance Criteria
1. **Unchanged file**: `reindex` on a workspace where nothing changed → 0 embeddings generated, 0 chunks rewritten. Visible via `/api/status` embed counters before/after.
2. **Modified file**: changing 1 file out of 1000 → exactly that file's chunks are re-embedded; other 999 untouched.
3. **Deleted file**: removing a file from disk + reindex → chunks for that file are deleted; orphan rows = 0.
4. **New file**: adding 1 file + reindex → that file's chunks are embedded; existing 1000 untouched.
5. **Schema migration is reversible** (down migration provided) AND additive (no breaking change to existing chunks/documents tables — only adds columns or a sibling table).
6. **No regression** to watcher latency (fsnotify event → DB upsert): p95 must not increase by more than 5%.
