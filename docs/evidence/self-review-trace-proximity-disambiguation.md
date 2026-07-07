# Self-Review — Issue #575 (#542 F2: trace bare-call proximity disambiguation)

Change-type: bug-fix · Lane: normal (search-quality-adjacent) · Branch: `fix/trace-proximity-disambiguation`
Author: kokorolx.

## Actions Taken

- `memory_trace` no longer fans a bare call out to every same-named symbol in a
  monorepo. When a bare calls-target resolves to multiple candidates, it now
  resolves to the definition **nearest the caller in the directory tree**.
- `internal/mcp/graph_paths.go`: `sharedPathDepth(a,b)` (common leading path
  segments) + `nearestSymbolMatch(callerFile, wsRoot, matches)` — returns the
  single candidate with the strictly-deepest shared prefix, or `(zero,false)` on
  a tie / empty caller. Candidate `source_path` (absolute in prod) is normalized
  via `stripWorkspacePrefix`.
- `internal/mcp/tools.go` (`registerMemoryTrace`): on `len(matches)>1`, a unique
  nearest → resolve to it + clear `ambiguous`; a tie → unchanged (emit all,
  ambiguous). No new query, no re-index.

## Files Changed

- `internal/mcp/graph_paths.go` — `sharedPathDepth` + `nearestSymbolMatch`.
- `internal/mcp/tools.go` — wire into the trace bare-call resolution block.
- `internal/mcp/trace_proximity_575_test.go` — white-box unit (cross-repo /
  same-file / tie / empty).
- `internal/mcp/trace_proximity_575_integration_test.go` — e2e through the
  handler (cross-repo `foo` collision → backend resolved, frontend dropped, not
  ambiguous).

## Findings Summary

- **Recall-safe by design**: narrowing happens ONLY when a unique nearest exists;
  a tie (two candidates at equal max depth) keeps today's emit-all-ambiguous
  behavior, so a legitimate cross-dir call is never dropped. The heuristic risk
  (a nearer same-named collision masking a farther real target) is bounded and
  strictly better than today's all-ambiguous fan-out.
- **Red-green proven**: without the fix the integration test returns both
  `foo` nodes (ambiguous); with it, only `backend/svc.go::foo`.
- No regression: `len(matches)==1` and `matches==0` (external-drop) paths
  untouched; the existing `TestMemoryTrace_AmbiguousSameNameSymbolsYieldDistinctNodes`
  still passes (its fixture is a genuine tie → stays ambiguous).

## Scope / honest limits (from deep-design of root-cause C)

- Covers **memory_trace** (the reported repro). `memory_impact`/`memory_flow`
  ambiguity lives in the flow builder — a parallel follow-up on #542 F2.
- Path-proximity, not import-graph-precise: import edges are file/module-level
  and JS/TS targets aren't resolved in current data (needs a re-index), and the
  import graph is useless for Go same-package calls — so proximity is the
  correct no-re-index disambiguator. Import-precise JS/TS is a later increment.

## Resolution Status

- In scope resolved. No critical/major issues.
- `go build ./...` clean; `go test -race -short ./...` all ok.
- Integration (nanobrain_test): unit + 6 trace tests PASS (pre-existing
  `TestMemoryTrace_RelativeInputAndOutput` panic is #556, unrelated).
- smoke:e2e: `docs/evidence/smoke-e2e-trace-proximity-disambiguation.md`.

## Gemini Verification Triage

_Pending — populate after the Gemini bot reviews the PR._

| Comment ref | Agent verdict | Reasoning | Action |
| --- | --- | --- | --- |
| _(none yet)_ | | | |
