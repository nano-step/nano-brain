# Design — Incremental Reindex

## Approach: content-hash tracking on `documents`

Reuse the existing `documents.content_hash` (SHA-256, already populated by `UpsertDocument`). The dedup is already there at the document level. The gap is that `reindex` doesn't *use* it — it currently deletes all chunks unconditionally.

### Current flow (full wipe)
```
POST /api/v1/reindex
 → DeleteChunksByWorkspace(workspace)
 → DeleteDocumentsByWorkspace(workspace)
 → Walk root directory
 → For each file: read → hash → UpsertDocument → chunk → embed → InsertChunks
```

### New flow (incremental)
```
POST /api/v1/reindex
 → Walk root directory (collect {path, content_hash} for every file)
 → Compute set: { current_paths } = paths found on disk
 → Query: SELECT source_path, content_hash FROM documents WHERE workspace_hash = $1
   → indexed_paths_by_hash = { source_path -> content_hash }
 → For each file (path, hash) on disk:
     - if path not in indexed_paths_by_hash → NEW: chunk + embed + insert
     - if path in indexed_paths_by_hash AND hash == stored → UNCHANGED: skip
     - if path in indexed_paths_by_hash AND hash != stored → CHANGED: delete old chunks for this document_id, re-chunk + re-embed
 → For each (path, hash) in indexed_paths_by_hash where path NOT in current_paths → DELETED: cascade delete (documents → chunks via FK)
```

## Schema impact

**Zero new columns.** `documents.content_hash` already exists and is already populated. The change is purely in handler logic — the `reindex.go` handler shifts from "DELETE then INSERT" to "DIFF then patch".

If telemetry is desired (count of skipped vs embedded files per reindex), add a new sibling table `reindex_runs` (timestamp, workspace_hash, scanned, skipped, embedded, deleted). This is OPTIONAL and can be deferred.

## Watcher integration

The watcher already detects file changes via fsnotify and calls `UpsertDocument` → which already short-circuits when the content_hash matches existing. So watcher-driven incremental indexing already works. This change brings the **explicit `reindex` button** in line with that behavior.

## Trade-offs

| Approach | Pros | Cons |
|---|---|---|
| **A. Content-hash diff (chosen)** | Zero schema change. Reuses existing `content_hash`. Easy to test. | Re-reads + hashes every file on disk — disk I/O bound for huge trees. |
| B. Track mtime per file | Faster (no read+hash; just stat). | mtime is unreliable (touch -t, container layer copy, git checkout all bump it without content change). Skews "changed" set. |
| C. Use fsnotify-only (incremental from watcher events) | Zero disk scan. Real-time. | Misses changes that happen while server is down (e.g. git pull during downtime). Reindex is meant to be the recovery from this. |

**Recommendation:** A (content-hash). C is already covered by the watcher. B is a footgun.

## API shape (no change to the wire contract)

Request body unchanged. Response gains optional counters:

```json
{
  "workspace": "abc...",
  "scanned": 1024,
  "skipped": 1020,
  "embedded": 4,
  "deleted": 0,
  "duration_ms": 87
}
```

These are additive fields — existing clients that ignore them keep working.

## Risks & mitigations

| Risk | Mitigation |
|---|---|
| Hash computation cost dominates on large repos | Mitigation: stream-hash with `io.Copy` into a `sha256.New()` hasher (already how `UpsertDocument` does it). Single pass, no full buffer. |
| Race: file mutated during walk | Accepted. Next reindex picks it up. The watcher path handles real-time. |
| Stale chunks if migration interrupted mid-flight | Transaction wrapping per-document: delete old chunks + insert new chunks in single tx (mirror PR #222 #229 pattern). |
| Reindex with `--force` flag for full wipe is preserved | New flag `--force-wipe` on the reindex CLI; default behavior becomes incremental. |
| Watcher latency regression | Watcher is unchanged. Only `reindex.go` handler logic changes. |

## Test plan

1. **Unit (handler):** mock `Queries` interface; verify diff-skip-vs-embed branches.
2. **Integration:** reindex on empty workspace → adds N docs. Reindex again unchanged → 0 embeds. Touch one file (no content change) → 0 embeds. Modify one file → 1 embed. Delete one file from disk → that doc's chunks gone.
3. **E2E:** run twice on nano-brain repo itself; second run completes in < 5% of first run's duration.
4. **Watcher regression:** `internal/watcher/watcher_test.go` continues to pass (no changes there).
