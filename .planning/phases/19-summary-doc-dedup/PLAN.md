# Phase 19 — Plan (#552 dedup code-summary docs + retry 524)

Issue #552 · high-risk (data-model) · bug-fix. Branch: `fix/552-summary-doc-dedup` (checked out).

## Task 1 — stable source_path (D1/D2)
- `internal/codesummarize/service.go:437`: drop the `&hash=%s` segment and the `shortHash` computation (lines 432-435). New:
  `sourcePath := fmt.Sprintf("%s?symbol=%s&kind=%s&summary=true", symbol.File, symbol.Name, symbol.Kind)`
- Keep `metadata.source_content_hash = symbol.ContentHash` and `ContentHash: computeContentHash(summary)` on UpsertDocument (unchanged). Now re-summarize UPDATEs in place.
- Verify build.

## Task 2 — runtime cleanup of legacy hash-path dups (D3)
- New sqlc query in `internal/storage/queries/documents.sql`:
  ```sql
  -- name: DeleteLegacySummaryDocsForSymbol :exec
  DELETE FROM documents
  WHERE workspace_hash = @workspace_hash
    AND starts_with(source_path, @path_prefix);
  ```
  (`starts_with`, NOT LIKE — symbol/file names contain `_`/`%`.) Chunks cascade via FK (migrations/00001).
- `sqlc generate`.
- In `upsertSummaryDocument`, AFTER the successful UpsertDocument + UpsertChunk, call it with
  `path_prefix = fmt.Sprintf("%s?symbol=%s&kind=%s&hash=", symbol.File, symbol.Name, symbol.Kind)`.
  The new no-hash doc has no `&hash=` so it is never matched; only legacy variants are removed. Log the deleted count at debug.
- Order matters: upsert the canonical doc FIRST, then delete legacy — never leave the symbol with zero summary docs.

## Task 3 — retry 524 (D5)
- `internal/codesummarize/retry.go:18`: add `524` to `transientRegex` → `\b(429|408|500|502|503|504|524)\b`.

## Task 4 — tests
- `internal/codesummarize/retry_test.go` (or existing): table test — `524`, `503`, `429` → transient; `400`, `404` → not. (AC-4)
- Integration (`//go:build integration`, `testutil.SetupTestDB`): 
  - AC-1: call upsertSummaryDocument (or the service path) for one symbol twice with different `summary` content → assert exactly 1 doc for that symbol, `source_path` has no `&hash=`, content = latest.
  - AC-2: pre-insert a legacy `<file>?symbol=X&kind=Y&hash=deadbeef&summary=true` doc + a chunk; run upsert for symbol X; assert the legacy doc AND its chunk are gone, only the no-hash doc remains.
- `go test -race -short ./internal/codesummarize/... ./internal/storage/...` then `-tags=integration`.

## Task 5 — smoke:e2e (gate 3.12)
- `nanobrain_test`/:3199 only, PID-scoped kill, NEVER dev/:3100. Build binary, start :3199 server (config.test.yml has code_summarization enabled), index this repo (or a tiny fixture), `curl -i POST /api/v1/code/summarize` twice (force), then query documents (or SQL) and assert one summary doc per symbol / no `&hash=` dup paths. Capture `curl`+`HTTP/` into `docs/evidence/smoke-e2e-552-summary-doc-dedup.md`.

## Gates
validate:quick → integration → smoke:e2e → independent R88 reviewer (NOT executor) → ship PR to master (author kokorolx, no AI footer, Closes #552).
Evidence files must key off branch slug `552`: `smoke-e2e-552-*`, `self-review-552*`, `review-552.md` (Review Verdict: PASS + clean Reviewer: line).
