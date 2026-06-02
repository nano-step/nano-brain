# Self-Review: feat-158-incremental-reindex

Date: 2026-06-02
PR: #339
Reviewer: Gemini Code Assist + manual triage

## Findings

| # | Severity | File | Description | Verdict | Status |
|---|----------|------|-------------|---------|--------|
| 1 | critical | `internal/server/handlers/reindex.go` | `walkCollectionFiles` swallows root-inaccessible error → returns empty map → all docs deleted | VALID:critical | FIXED |
| 2 | medium | `internal/server/handlers/reindex.go` | `TriggerRescanByName` called per-file inside loop instead of once per collection | VALID:medium | FIXED |
| 3 | medium | `internal/server/handlers/reindex.go` | `deleted` counter incremented even when DB delete fails | VALID:medium | FIXED |

## Fix details

**Finding 1 (critical):** Added `os.Stat(col.Path)` check before `WalkDir`. If root is inaccessible, return error immediately so `triggerIncremental` skips the collection rather than treating it as empty. Also propagated root-level `walkErr` back instead of swallowing it.

**Finding 2 (medium):** Introduced `colHasChanges bool` flag. `TriggerRescanByName` now called once after the per-file loop, only when at least one new or changed file was detected.

**Finding 3 (medium):** Split delete logic — `chunksErr` and `docErr` captured separately. `deleted` only incremented when both are nil.

## Summary

- Critical: 1 found, 1 fixed
- Medium: 2 found, 2 fixed
- Minor: 0
- All fixes: `go build ./...` ✅, `go test -race -short ./...` ✅
