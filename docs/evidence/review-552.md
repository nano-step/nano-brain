Reviewer: gsd-code-reviewer (Sonnet)
Review Verdict: PASS

Re-review of `fix/552-summary-doc-dedup` after remediation of the round-1 BLOCKER. All prior findings are resolved; no new blocking issues found. Independently re-verified by reading the applied diff, tracing the upsert path, building/vetting the package, and running the integration + unit tests against `nanobrain_test`.

## Round-1 findings — resolution status

### BLOCKER (chunk-level duplication survived the doc-level fix) — RESOLVED

**File:** `internal/codesummarize/service.go:470-479` (new `DeleteChunksByDocumentID` call), interface addition at `service.go:27`.

The fix inserts `DeleteChunksByDocumentID(result.ID, workspaceHash)` after `UpsertDocument` and before `UpsertChunk`. Traced mechanically:
- Call 1 (new symbol): `UpsertDocument` INSERTs doc D1 → `DeleteChunksByDocumentID(D1)` deletes 0 → `UpsertChunk` inserts C1. Result: 1 chunk.
- Call 2 (same symbol, different content): `UpsertDocument` hits `ON CONFLICT (source_path, workspace_hash)` → returns the same D1 → `DeleteChunksByDocumentID(D1)` deletes the stale C1 (`content_hash=H1`) → `UpsertChunk` with `H2` inserts C2. Result: still exactly 1 chunk (the current one).

This closes the exact accumulation path identified in round 1 (`UpsertChunk`'s conflict key `(content_hash, workspace_hash, document_id)` per `migrations/00002_fix_chunk_unique_constraint.sql:3` no longer strands the previous chunk under the now-stable document_id). Confirmed correct on the surrounding invariants:
- Scope is correct — the delete is keyed on `result.ID` (this symbol's unique document; distinct symbols have distinct `source_path` → distinct documents), so it can never clobber another symbol's chunks.
- Orphaned embeddings are cleaned up too — `embeddings.chunk_id REFERENCES chunks(id) ON DELETE CASCADE` (`migrations/00001_initial_schema.sql:48`), so deleting the stale chunk cascades to its embedding.
- Error handling is correct — the delete failure returns a wrapped error (`service.go:478`, `fmt.Errorf("delete stale summary chunks: %w", err)`) and aborts before the chunk upsert, so a failed delete cannot silently leave a dup.

Regression is now locked in test: `TestUpsertSummaryDocument_552_AC1` (`dedup_552_integration_test.go:133-145`) asserts `chunkCount == 1` after the two-call different-content sequence — the assertion that was missing in round 1. Without the fix this two-call sequence yields 2 chunks; with it, 1.

### WARNING (`permanentRegex` gained `404` — undocumented scope creep) — RESOLVED

`DESIGN.md` D5 (`.planning/phases/19-summary-doc-dedup/DESIGN.md:20`) now explicitly documents both `524 → transient` and `404 → permanent`, with rationale (a 404 from the LLM provider = wrong endpoint/model, so retrying is pointless — fail fast rather than burn `MaxRetries` via the default-transient fallthrough). `retry.go` is net-unchanged from its round-1 reviewed state. This is correct behavior and now in-scope per the locked design. Accepted.

### INFO (non-transactional upsert → delete-chunks → upsert-chunk → delete-legacy) — accepted as documented

The remediation slightly widens the previously-noted eventual-consistency window (there is now a delete-chunks/upsert-chunk gap in addition to the delete-legacy gap). A crash in that narrow window would leave the canonical document momentarily chunk-less; it self-heals on the next resummarization or full reindex, consistent with D3/D4's documented "converges on reindex" posture and the inline comment at the delete site. Not a blocker; acceptable for this change.

## Independent verification evidence

- `CGO_ENABLED=0 go build ./internal/codesummarize/` → exit 0; `go vet ./internal/codesummarize/` → exit 0. The new `DeleteChunksByDocumentID` interface method matches the generated `sqlc` signature (`func (q *Queries) DeleteChunksByDocumentID(ctx, DeleteChunksByDocumentIDParams{DocumentID, WorkspaceHash}) error`); no consumer of `ServiceQuerier` (`cascade.go`, `cmd/nano-brain/main.go` via real `sqlc.Queries`) breaks.
- `go test -race -tags=integration -count=1 -run TestUpsertSummaryDocument_552 ./internal/codesummarize/` → `ok` (AC-1 incl. chunk-count==1, and AC-2 legacy-doc + cascaded-chunk deletion, canonical survives) against `nanobrain_test`.
- `go test -race -count=1 -run TestClassifyError ./internal/codesummarize/` → `ok` (524/503/429 transient, 400/404 permanent).

## Still-valid round-1 correctness confirmations (unchanged)

- D1 stable `source_path` correctly hits `UpsertDocument`'s `ON CONFLICT (source_path, workspace_hash)` and updates in place.
- D3 `starts_with(source_path, @path_prefix)` is a literal-prefix match (not `LIKE`) built from exact symbol/kind values — no wildcard/injection risk, and can never match the canonical no-hash path, so the canonical doc is never deleted by the legacy cleanup.
- No other code path reads/writes the `&hash=...&summary=true` path scheme (grep-verified) — dropping the hash breaks no consumer.
- Staleness detection (`graph_context_hash`, per source-code chunk ID) is unaffected by the path change.
