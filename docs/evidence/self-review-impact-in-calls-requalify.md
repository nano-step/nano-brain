# Self-Review — Issue #553 (impact-in calls requalify, Phase 2 PR-A)

## Actions Taken

- Added `impactFrontierWithBareSuffix` in `internal/mcp/tools.go`: expands the
  `memory_impact` `"in"` frontier with the bare symbol suffix of qualified
  `file::symbol` entries (reusing `splitNodeSymbol`), so `GetImpactorsByTargets`
  matches bare-stored `calls`-edge targets. Applied at the initial frontier and
  every depth-loop batch (transitive callers).
- No SQL/schema/migration change — the reverse query and `target` index already
  existed; only the handler's frontier seeding was wrong.
- Added `internal/mcp/impact_bare_calls_integration_test.go` (3 tests) seeding
  bare `calls` edges against `nanobrain_test`.

- REST parity: `POST /api/v1/graph/impact` (`collectImpact`) had the identical
  bug. Extracted the frontier-expansion into a shared, self-contained
  `internal/symbol/impact_frontier.go::ExpandImpactFrontier` (no import cycle;
  leaf package), refactored the MCP tool to use it, and applied it to the REST
  handler. One implementation, both surfaces. Surfaced by the smoke:e2e gate.

## Files Changed

- `internal/symbol/impact_frontier.go` (new): shared `ExpandImpactFrontier`.
- `internal/mcp/tools.go`: use shared helper (local helper removed).
- `internal/server/handlers/impact.go`: use shared helper at seed + each batch.
- `internal/mcp/impact_bare_calls_integration_test.go` (new): MCP-tool tests.
- `internal/server/handlers/impact_bare_calls_integration_test.go` (new): REST tests.

## Findings Summary

- Root cause was the impact handler never probing the bare name calls edges are
  stored under (not "reverse edges missing" as #378 assumed).
- G1 (Gate 1.7): bare match is intentionally workspace-wide; cross-repo
  over-inclusion deferred to root-cause C (#542 F2). No per-repo scoping added.
- Minor non-blocking: a bare name may be re-queried across depth iterations
  (handler `seen` tracks qualified SourceNodes only); results still deduped,
  loop bounded by `maxDepth`.

## Resolution Status

- All findings resolved or explicitly deferred (G1 → root-cause C). No unresolved
  critical/major items.
- validate:quick 31 pkgs ok; integration `-run Impact` 4 PASS; build exit 0. Full
  independent review verdict PASS in `docs/evidence/review-553.md`.

## Gemini Verification Triage

PR #554. Gemini review state COMMENTED (non-blocking); 3 inline suggestions, all
perf, no correctness/security. All applied.

| Comment ref | Agent verdict | Reasoning | Action |
| --- | --- | --- | --- |
| PR#554 `internal/symbol/impact_frontier.go:22` | VALID:low | Prealloc `expanded`/`seen` at `2*len` + empty fast-path — genuine micro-opt, zero risk (helper doubles when nodes are qualified). | fixed in commit e146169 |
| PR#554 `internal/server/handlers/impact.go:81` | VALID:medium | `queried` map to skip re-querying targets across depths — real (bounded) inefficiency; safe since results already deduped by `seen` and the query is deterministic. Matters for the tool's 50ms latency goal. | fixed in commit e146169 |
| PR#554 `internal/mcp/tools.go:2071` | VALID:medium | Same `queried`-map dedup on the MCP path; kept consistent with the REST handler. | fixed in commit e146169 |
