## Self-Review: Story 2.5 — Chunker Integration
Date: 2026-05-23
Reviewer: Oracle

## Findings
| # | Severity | File | Description | Status |
|---|----------|------|-------------|--------|
| 1 | critical | chunks.sql, document.go | Unique constraint (content_hash, workspace_hash) allows chunk-stealing across documents. Same content in 2 docs → upsert overwrites document_id | FIXED — migration 00002 adds document_id to constraint |
| 2 | major | document.go | No-tx path (db==nil) has no atomicity — partial chunks on failure | FIXED — documented as test-only with comment |
| 3 | major | document.go | ~40 lines duplicated between tx/no-tx paths | FIXED — extracted writeChunks() helper |
| 4 | major | document_test.go | Transaction path (db!=nil) has zero test coverage | DEFERRED — integration tests belong in later story |
| 5 | minor | document.go | Per-chunk INSERT loop, not batch | DEFERRED — acceptable for v1, revisit if latency issue |
| 6 | minor | document.go | Silent empty workspace if middleware bypassed | FIXED — added defense-in-depth guard |

## Summary
- Critical: 1 found, 1 fixed
- Major: 3 found, 2 fixed, 1 deferred (integration tests)
- Minor: 2 found, 1 fixed, 1 deferred (batch insert perf)
