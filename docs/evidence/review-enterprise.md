# Review: enterprise roadmap docs (#754)

Reviewer: oh-my-claudecode:code-reviewer (Opus 5.5, separate review agent)
Scope: docs/ROADMAP-ENTERPRISE.md (new), docs/ROADMAP.md, nano-brain-observe docs/ROADMAP.md and docs/DESIGN.md

## Round 1 — FAIL
- MAJOR-1: the doc said MCP tools are not recorded, but every `tools/call` writes to `telemetry_logs` (#678, `internal/mcp/telemetry.go`). Fixed in both repos.
- MAJOR-2: wrong `routes.go` line numbers, and `/code/summarize/failures` was missing. Fixed (`routes.go:75,76,112`).
- MINOR-3 to MINOR-12: permission matrix overlaps and missing routes; `memory_update` moved to maintain; `/harvest` moved to admin; Q7 and Q8 moved to resolved; O1 scope aligned between the repos; MCP spec details (path-suffixed RFC 9728 URL, client registration, Basic challenge kept); `workspace_ids` cited. All fixed.

## Round 2 — PASS
- All 14 items verified against the source.
- MINOR-A (stale Q7/Q8 references) and MINOR-B (double space, `/ui` missing from the matrix) fixed after round 2.
- Privacy grep is clean.

Review Verdict: PASS
