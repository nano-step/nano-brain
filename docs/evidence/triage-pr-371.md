# Bot Review Triage — PR #371

Date: 2026-06-04
Reviewer: gemini-code-assist[bot]
Commit reviewed: 4f322db
Fix commit: 087b7d0

## Triage Table

| # | File | Severity | Verdict | Action |
|---|------|----------|---------|--------|
| 1 | internal/chunker/symbol.go:164 | critical | VALID:critical | Fixed: dedup by `declNode.StartByte()` instead of name — fixed in commit 087b7d0 |
| 2 | internal/chunker/symbol.go:115 | high | VALID:high | Fixed: shift sub-chunk line numbers by symbol start line — fixed in commit 087b7d0 |
| 3 | internal/mcp/tools.go:242 | high | VALID:high | Fixed: propagate chunk_type through HybridSearch interface — fixed in commit 087b7d0 |
| 4 | internal/chunker/dispatcher.go:31 | medium | VALID:medium | Fixed: case-insensitive extension matching via strings.ToLower — fixed in commit 087b7d0 |

## Summary

All 4 findings valid. All fixed in commit 087b7d0. No findings dismissed.
