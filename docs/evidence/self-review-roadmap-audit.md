## Self-Review: roadmap-audit-2026-05-30 (PR #215)
Date: 2026-05-30
Reviewer: Sisyphus orchestrator + Gemini PR review

## Findings
| # | Severity | File | Description | Status |
|---|----------|------|-------------|--------|
| 1 | High | docs/ROADMAP.md:85 | "9 MCP tools" — actual count is 13 (verified `grep -c '"memory_' internal/mcp/tools.go` = 13) | FIXED |
| 2 | Medium | docs/ROADMAP.md:141 | "MCP tools (9 tools)" in Implementation Order — same issue | FIXED |
| 3 | Medium | docs/ROADMAP.md:50, 69 | References to `output_dir` config field — deprecated since harvest-summary-only (May 2026); summaries live only in PG. README.md already notes this. | FIXED — removed output_dir line + updated max_tokens 4096→8000 to match shipped default |

## Verification
- Re-counted MCP tools in `internal/mcp/tools.go`: memory_query, memory_search, memory_vsearch, memory_get, memory_write, memory_tags, memory_status, memory_update, memory_wake_up, memory_graph, memory_trace, memory_impact, memory_symbols = **13**
- `internal/config/defaults.go:71` confirms MaxTokens=8000 default
- README.md "Note: as of harvest-summary-only (May 2026), summaries are no longer written to disk" confirms output_dir deprecated

## Summary
- High: 1 found, 1 fixed
- Medium: 2 found, 2 fixed
- Minor: 0
