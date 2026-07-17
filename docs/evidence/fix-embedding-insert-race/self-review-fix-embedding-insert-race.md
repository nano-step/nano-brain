# Author Self-Review — fix-embedding-insert-race

## Scope reviewed

- `InsertEmbedding` SQL and regenerated sqlc output.
- Queue and direct endpoint stale-result handling.
- New storage, queue, and direct-endpoint regressions.

## Acceptance criteria

| Criterion | Review result |
|---|---|
| A deleted `(chunk_id, workspace_hash)` produces `sql.ErrNoRows`, not SQLSTATE 23503 | PASS — the CTE selects the live chunk with `FOR KEY SHARE`; the isolated PostgreSQL regression passes. |
| A delete that begins during persistence waits for the CTE key-share lock and cascades the committed embedding | PASS — a trigger pauses the insert after CTE locking, `pg_stat_activity` observes the delete waiting on a lock, and the isolated integration regression proves no SQLSTATE 23503 then a zero embedding count after delete. |
| Queue finishes a no-row write without retrying or marking the chunk embedded | PASS — the no-row branch decrements pending work, clears retries, and returns; the unit regression passes. |
| Direct batch skips only the stale chunk, persists the next chunk, and reports actual pending work | PASS — the no-row branch continues, stale skips are excluded from non-paginated `remaining`, and the regression verifies both insert attempts and only the live chunk is marked embedded. |
| Generated API remains compatible | PASS — sqlc preserves `ChunkID` and `WorkspaceHash`; only internal parameter order changed. |

## Red → green proof

- Red storage test: the old query returned `embeddings_chunk_id_fkey` / SQLSTATE `23503`, not `sql.ErrNoRows`.
- Red queue test: stale retry state remained present.
- Red direct-handler test: response was `{Embedded:0 Remaining:2}` rather than continuing to the second chunk.
- Green focused tests: storage `1.869s`, embed `1.648s`, handlers `2.114s`, all with `-race`.
- Green package scopes: `go test -count=1 -race -tags=integration ./internal/storage` (`2.125s`) and `go test -count=1 -race ./internal/embed ./internal/server/handlers` (embed `1.481s`, handlers `2.495s`), using only `nanobrain_test`.
- Red accounting correction: the revised direct-handler regression observed `{Embedded:1 Remaining:1}` where the stale chunk was already deleted.
- Green accounting correction: the focused handler test passed, then storage integration (`2.343s`), embed (`1.715s`), and handlers (`2.711s`) passed with `-race` using only `nanobrain_test`.
- Concurrent delete ordering: the test-only insert trigger provides a deterministic post-CTE barrier. The focused isolated test passed once (`2.134s`) and five consecutive `-race` runs (`1.994s`); the full storage integration package passed (`2.196s`). A red run was not possible because the regression was added after the CTE fix; removing the production CTE solely to manufacture one would violate scope.

## Finding disposition

- Resolved — non-paginated `remaining` now subtracts stale skips. The direct-handler regression expects and proves `{Embedded:1 Remaining:0}` after one stale skip and one successful write.

## Conclusion

**PASS.** The requested storage integrity, stale-result handling, batch continuation, and response accounting are implemented and verified. No unresolved author-review findings remain.
