# Phase 19 — Design: dedup code-summary docs + retry 524 (#552)

Tracking: #552. Lane: **high-risk** (data-model gate — changes the summary doc source_path key + deletes rows). Change-type: **bug-fix**.

## Root cause (verified in current code + live)

- `upsertSummaryDocument` (`internal/codesummarize/service.go:437`) builds `source_path = "<file>?symbol=<name>&kind=<kind>&hash=<shortContentHash>&summary=true"`.
- `UpsertDocument` conflicts on `(source_path, workspace_hash)` (`internal/storage/queries/documents.sql:4`). Because the content hash is IN the path, a symbol whose content changed → new hash → **new path → no conflict → INSERT a fresh row**. The old summary doc is never updated or removed → duplicates accumulate (report: 9+ per symbol; `SupersedesID` always empty at :468).
- Observed live this session: `HybridSearch` had summary docs at `&hash=52bccba1` and `&hash=afded554` (two dates); `hybridSearchInner` likewise — real duplicates.
- **Staleness is NOT affected by the path**: re-summarize decisions use the chunk's `graph_context_hash` column (`service.go:401`) and the summarization-status table, never the hash parsed from `source_path`. So dropping the hash from the path is safe.
- **524 retry gap**: `retry.go:18` `transientRegex = \b(429|408|500|502|503|504)\b` — **524** (Cloudflare "A Timeout Occurred") is absent, so 524s are treated as permanent (the second half of #552).
- **Chunk cleanup is automatic**: `chunks.document_id REFERENCES documents(id) ON DELETE CASCADE` (`migrations/00001:32`) — deleting a summary document cascades to its chunks + chunk_entities.

## Decisions (locked)

- **D1 — stable source_path.** Drop `&hash=` → `"<file>?symbol=<name>&kind=<kind>&summary=true"`. Re-summarizing the same symbol now hits the ON CONFLICT and UPDATEs in place → exactly one doc per (file, symbol, kind).
- **D2 — content hash retained off-path.** Keep it in `documents.content_hash` (already set via `computeContentHash(summary)`) and `metadata.source_content_hash`. No consumer parses it from the path (verified), so nothing breaks.
- **D3 — runtime cleanup of legacy hash-path variants.** After the upsert, delete pre-existing `&hash=…` variants for THIS symbol via a new query using `starts_with(source_path, $prefix)` where `prefix = "<file>?symbol=<name>&kind=<kind>&hash="` (NOT `LIKE` — symbol/file names contain `_`/`%` which are LIKE wildcards). The new no-hash doc lacks `&hash=` so it is never matched → it survives; only legacy dups are removed (chunks cascade). A full reindex completes cleanup for symbols not otherwise re-summarized.
- **D4 — NO bulk destructive migration.** A global dedupe migration over shared prod data is higher-risk than the scoped, per-symbol runtime cleanup that converges on reindex. Deliberately deferred; note in the PR that operators can reindex to force convergence.
- **D5 — retry classification.** Add `524` (Cloudflare timeout) to `transientRegex` so it retries with the existing backoff. Also add `404` to `permanentRegex`: a 404 from the LLM provider means a wrong endpoint/model — retrying is pointless, so fail fast instead of falling through to the default "transient" and burning `MaxRetries`. (Both are corrections to the shared `ClassifyError`; documented here per R88 review of the initial scope.)

## Non-goals

- Bulk one-shot dedupe migration (D4).
- Reworking the graph-context-hash staleness mechanism (works; untouched).
- Superseding/versioning summary history — overwrite-in-place is correct for regenerated summaries.

## Acceptance criteria

- **AC-1:** Two summarizations of the same symbol with DIFFERENT content produce exactly ONE summary document (source_path has no `&hash=`), with updated content — integration test.
- **AC-2:** After the fix, an upsert removes any legacy `&hash=…&summary=true` docs for that symbol (and their chunks, via cascade) — integration test seeding a legacy-path dup then asserting it's gone.
- **AC-3:** The surviving doc's `content_hash` column + `metadata.source_content_hash` reflect the latest summary (staleness detection intact).
- **AC-4:** `transientRegex` matches `524` (unit test); a 524 error is classified transient/retryable.
- **AC-5:** No regression: existing `internal/codesummarize` + `internal/storage` tests green; `sqlc generate` clean.

## Test plan

- **Unit:** retry.go — table test that `524` (and existing codes) classify as transient, and a non-transient (e.g. `400`) does not.
- **Integration** (`testutil.SetupTestDB`, isolated schema): (a) upsert summary for symbol X twice with different content → assert 1 doc, no `&hash=` in path, latest content; (b) manually insert a legacy `…&hash=abc&summary=true` doc + a chunk for symbol X, run upsert, assert legacy doc + its chunk are deleted and only the no-hash doc remains.
- **smoke:e2e** (`nanobrain_test`/:3199, PID-scoped kill, never dev/:3100): index a tiny repo, trigger `POST /api/v1/code/summarize` twice (force re-summarize), query docs and assert one summary doc per symbol (no hash-path dups). Capture `curl` + `HTTP/` into `docs/evidence/smoke-e2e-552-summary-doc-dedup.md`.
- **Ladder:** `go build ./... && go test -race -short ./...` → `-tags=integration` → smoke → independent R88 review → ship.

