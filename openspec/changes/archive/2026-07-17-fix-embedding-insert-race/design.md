## Context

Both embedding writers read a chunk, call an external embedder, and then persist the vector. Document rewrites, reindexing, force-wipe, workspace removal, and explicit deletion can remove that chunk during the provider call. The queue currently catches the resulting foreign-key error, but PostgreSQL has already emitted the reported error; the direct endpoint treats it as an ordinary failed batch item.

## Goals / Non-Goals

**Goals:**

- Prevent an embedding insert from attempting to reference an already-deleted chunk.
- Preserve deletion throughput by not locking a chunk during the external embedding request.
- Give both production writers the same benign stale-result semantics.

**Non-Goals:**

- Change queue deduplication, retry policy, provider behavior, or the HTTP response schema.
- Introduce a migration or recover embeddings for chunks that no longer exist.

## Decisions

1. Make `InsertEmbedding` select the live `(id, workspace_hash)` row in a CTE with `FOR KEY SHARE`, then insert from that CTE. The short lock makes the existence check and FK write atomic: a deletion that committed first yields no returned row; a later deletion waits until the insert completes and then cascades it away.

   Alternative: re-read the chunk in Go before inserting. Rejected because deletion can still occur between that check and the insert. Catching SQLSTATE 23503 alone is also rejected because it leaves the PostgreSQL ERROR reported in #600.

2. Preserve sqlc's `:one` query contract. A stale chunk returns `sql.ErrNoRows`, which both writers explicitly classify as a benign skip.

   Alternative: add a new result type or separate existence query. Rejected because the existing no-row signal is precise and keeps the storage API small.

3. The direct endpoint continues the current batch after a stale result; non-stale database failures retain the existing stop-and-log behavior. The queue clears its retry state and finishes the stale job as it does for its existing FK handling.

## Risks / Trade-offs

- [A delete waits briefly on the final insert] → The key-share lock exists only for the database statement, never during the provider call.
- [Callers could miss the new no-row result] → Audit both production `InsertEmbedding` callers and add regression tests for each.
- [Generated SQL drifts] → Regenerate sqlc and verify generated output is committed with the query change.

## Migration Plan

1. Deploy the query and both caller updates together; no schema migration is required.
2. Roll back by restoring the prior query and caller handling. Existing vectors remain valid because chunk deletion cascades embedding deletion.

## Open Questions

- None.
