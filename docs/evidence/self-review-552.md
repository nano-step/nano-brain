# Self-review — #552 (dedup code-summary docs + retry 524)

Branch: `fix/552-summary-doc-dedup`. Change-type: bug-fix. Lane: high-risk (data-model).

## Summary
- `internal/codesummarize/service.go`: summary doc `source_path` no longer embeds the content hash → `"<file>?symbol=<name>&kind=<kind>&summary=true"`. `UpsertDocument`'s `ON CONFLICT(source_path, workspace_hash)` now UPDATEs in place instead of minting a new row per content change. After the upsert, a new query removes legacy `&hash=…` variants for the symbol (chunks cascade via FK).
- `internal/storage/queries/documents.sql`: `DeleteLegacySummaryDocsForSymbol` using `starts_with(source_path, @path_prefix)` (parameterized — not LIKE; symbol/file names contain `_`/`%`). `sqlc generate` regenerated.
- `internal/codesummarize/retry.go`: added `524` to `transientRegex`.

## Response shape
N/A — no API/response struct changed.

## Staged files
service.go, retry.go, documents.sql, generated sqlc/documents.sql.go, retry_test.go, dedup_552_integration_test.go, docs/evidence/*. No `.opencode/`, no unrelated files.

## Verification
- `sqlc generate` clean; `CGO_ENABLED=0 go build ./...` OK.
- `go test -race -short ./internal/codesummarize/... ./internal/storage/...`: pass.
- integration (`-tags=integration`, nanobrain_test): `TestUpsertSummaryDocument_552_AC1` (two different-content upserts → 1 doc, no `&hash=`, latest content) and `_AC2` (legacy dup + its chunk deleted, canonical survives) both PASS.
- smoke:e2e (`docs/evidence/smoke-e2e-552-summary-doc-dedup.md`, :3199/nanobrain_test).

## Safety notes
- Staleness detection untouched — uses chunk `graph_context_hash` + summarization-status, never the path hash (verified).
- Upsert-then-delete ordering: canonical doc written first, so the symbol is never left with zero summary docs; delete failure is non-fatal (Warn).
- No bulk destructive migration (D4) — per-symbol runtime cleanup converges on reindex; lower risk on shared data.
