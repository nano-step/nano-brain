## Self-Review: feat/156-scope-all-cli
Date: 2026-05-30
Reviewer: Sisyphus orchestrator

## Findings
| # | Severity | File | Description | Status |
|---|----------|------|-------------|--------|
| - | none | - | Backend already supported workspace="all" (middleware accepts it; service dispatches to BM25SearchAll/VectorSearchAll). Pure CLI wrapper. | n/a |

## E2E smoke (PG @ host.docker.internal:5432, 11 existing workspaces)
- `query "test query" --scope=all --json` → returned 20 cross-workspace results spanning multiple workspace hashes (verified `workspace_hash` field differs across results)
- `search "anything" --scope=all` (no --workspace) → 200 OK with empty results (search index returned nothing for that term, no error)
- `query "x" --scope=invalid --workspace=abc` → exits with usage error: `invalid --scope value "invalid": must be "workspace" or "all"`
- Default behavior preserved: `query "x" --workspace=abc` still works without --scope

## Unit tests
- 10 new tests in commands_scope_test.go (7 parseStubFlags + 3 httptest integration)
- All pass under `go test -race -short -count=1 ./cmd/nano-brain/...`

## Build
- `CGO_ENABLED=0 go build ./...` → exit 0

## Summary
- Critical: 0, Major: 0, Minor: 0
- E2E cross-workspace search verified working with real data
