## Self-Review: feat/160-tags-filter-query-search
Date: 2026-05-30
Reviewer: Sisyphus orchestrator

## Findings
| # | Severity | File | Description | Status |
|---|----------|------|-------------|--------|
| - | none | - | Pattern mirrors existing BM25 handler with tags (already production). Extends to vector + hybrid via Option A (add `tags []string` to HybridSearch signature). | n/a |

## E2E smoke (PG @ host.docker.internal:5432, 13 workspaces, 2700+ docs with 'go' tag, 286 with 'summary')
- `query "function" --scope=all --tags=summary --json` → **20 results**, all paths under `summary://opencode/` (correctly filtered to summary collection)
- `query "function" --scope=all --tags=opencode --json` → 20 results (correctly filtered)
- `query "test" --scope=all` (no --tags) → 20 baseline results
- Direct HTTP `POST /api/v1/query {"tags":["summary"]}` confirmed: filter applied at SQL layer (`AND d.tags && ARRAY['summary']::text[]`)
- All 4 dispatch paths in `service.HybridSearch` verified via unit tests:
  - workspace=X, tags=[] → `BM25Search` + `VectorSearch`
  - workspace=X, tags=non-empty → `BM25SearchWithTags` + `VectorSearchWithTags` (NEW)
  - workspace=all, tags=[] → `BM25SearchAll` + `VectorSearchAll`
  - workspace=all, tags=non-empty → `BM25SearchAllWithTags` + `VectorSearchAllWithTags` (NEW)

## Unit tests
- 4 new tests covering tag dispatch paths in `internal/search/service_test.go`
- New tests in `handlers/query_test.go`, `handlers/search_test.go` for tag-aware handlers
- 6 new tests in `cmd/nano-brain/commands_scope_test.go` for CLI parsing
- Full suite: `go test -race -short ./...` → 20 packages OK

## Build
- `CGO_ENABLED=0 go build ./...` → exit 0
- sqlc generate clean (added VectorSearchWithTags + VectorSearchAllWithTags queries to embeddings.sql)

## Summary
- Critical: 0, Major: 0, Minor: 0
- E2E proves filter works at SQL level (20 results vs 0 depending on tag overlap)

## Gemini PR #221 Review — Findings Addressed (2026-05-30)

| # | Finding | Severity | Verdict | Fix |
|---|---------|----------|---------|-----|
| 1 | Vector search handler did double mapping (rows → intermediate vsearchRow → SearchResult) | Medium | VALID | Removed intermediate struct; map directly from sqlc rows to SearchResult in both branches |
| 2 | CLI tag parser didn't trim whitespace or filter empty tags (`--tags=a, ,b` produced `["a", " ", "b"]`) | Medium | VALID | Added `parseTagList` helper: TrimSpace each element, drop empties, return nil if all empty (which means "no filter"). 4 new unit tests cover edge cases. |

## Re-verified E2E
- `query "function" --scope=all "--tags= summary , , opencode "` → 20 results (whitespace + empty handled)
- `query "function" --scope=all "--tags= , , "` → 20 baseline results (all-empty drops to nil; no filter)
- All existing tests + 4 new TestParseTagList_* tests pass
