# Review — Issue #747

Reviewer: oh-my-claudecode:code-reviewer (Opus 5.5, separate agent)
Review Verdict: PASS

## Scope
`git diff` of fix/747-bm25-or-time-filter: the SQL OR queries, the regenerated sqlc code, the two `search/service.go` fallback blocks, the `mcp/tools.go` memory_search relaxation, and the unit test.

## Findings
- **MEDIUM (accepted, documented):** `hybridSearchInner` runs the config leg of DebugSearch with config tags. It now gets no OR fallback. Before, it got an untagged one that leaked non-config docs. This is recorded in the PR body; tag-aware OR variants are a possible follow-up.
- **MEDIUM (fixed):** nothing tested the SQL filter at the DB level. I added the integration test `TestTimeFilter_BM25SearchOR_RespectsTimeRange`. It checks that one in-window doc comes back and the out-of-window doc does not, for both OR queries. A control case without a filter returns both docs.
- **LOW (accepted):** the mock does not record `BM25SearchAllOR` params, and the `memory_search` OR path has no unit test. The integration test covers the AllOR SQL, and the smoke test covers the end-to-end path.
- **LOW (accepted):** the fallback block is duplicated between HybridSearch and hybridSearchInner. Removing the duplication is outside the scope of this issue.

Callers checked across the whole repo: all 6 real call sites of `BM25SearchOR`/`BM25SearchAllOR` pass the time range. No caller is missing.
