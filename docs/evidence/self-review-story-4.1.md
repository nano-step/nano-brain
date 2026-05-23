## Self-Review: Story 4.1
Date: 2026-05-23
Reviewer: Oracle

## Findings
| # | Severity | File | Description | Status |
|---|----------|------|-------------|--------|
| 1 | major | handlers/bm25.go:63-65 | Snippet truncation splits multi-byte UTF-8 characters | FIXED |
| 2 | major | handlers/bm25.go, queries/search.sql | Tag filtering not implemented (spec gap) | FIXED |
| 3 | minor | migrations/00005 | Migration UPDATE may be slow on large tables | DEFERRED |
| 4 | minor | handlers/bm25_test.go | No test for missing workspace context | FIXED |
| 5 | minor | handlers/bm25.go | websearch_to_tsquery edge-case input | DEFERRED |

## Summary
- Critical: 0 found
- Major: 2 found, 2 fixed (UTF-8 rune truncation, tag filtering with BM25SearchWithTags)
- Minor: 3 found, 1 fixed, 2 deferred (migration perf note, query length check)
