# Self-Review: Rails capability score and agent-oriented benchmark (Issue #489)

Date: 2026-06-24
Reviewer: Sisyphus orchestration + independent review gate

## Findings

| # | Severity | File | Description | Status |
|---|----------|------|-------------|--------|
| 1 | major | `benchmarks/*/capability/runner.go` | Fixed single-tool benchmark did not reflect agent-oriented nano-brain usage | FIXED |
| 2 | major | `benchmarks/rails/capability/dataset.json` | Rails ground truth contained stale status/notifier names and undercounted query snippets | FIXED |
| 3 | major | `internal/mcp/tools.go` | MCP `memory_flow` still validated entries as HTTP-only, unlike HTTP flow handler | FIXED |
| 4 | major | `internal/storage/queries/graph.sql` | Impact lookups matched target nodes exactly and missed bare Rails `Class#method` targets | FIXED |
| 5 | major | `internal/symbol/ruby_extractor.go` | Ruby constants were not indexed as symbols | FIXED |
| 6 | major | changed tests/evidence | Private-like fixture strings appeared in changed files during review | FIXED |

## Verification

- `go test -c -tags=capbench ./benchmarks/capability` ✅
- `go test -c -tags=capbench ./benchmarks/rails/capability` ✅
- `go build ./... && go test -race -short ./...` ✅
- `openspec validate improve-rails-capability-score --strict --no-interactive` ✅
- `./scripts/harness-check.sh in-progress --issue 489 --no-color` ✅
- Privacy grep over changed benchmark/story/evidence/OpenSpec/internal paths ✅
- Rails agent-oriented score-only run: overall `0.795` ✅
- nano-brain agent-oriented score-only run: overall `1.000` ✅

## Gemini Verification Triage

No Gemini comments were present at the time of local review.

| Comment ref | Agent verdict | Reasoning | Action |
|-------------|---------------|-----------|--------|

## Summary

- Major: 6 found, 6 fixed.
- Independent review gate passed after targeted fixes.
- Remaining full integration/lint blockers are pre-existing and documented separately if the pre-merge gate requires override evidence.
