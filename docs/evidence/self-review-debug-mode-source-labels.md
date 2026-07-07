# Self-Review — Issue #558 (memory_query mode:debugging source labels, Phase 3 PR-D)

## Actions Taken

- Added `Source string \`json:"source,omitempty"\`` to `search.Result`.
- `DebugSearch` (`internal/search/service.go`): `tagSource(results, "code"/"session"/"config")`
  applied per leg BEFORE the RRF merge, so each result carries its origin leg.
- Both MCP debug branches surface it — `registerMemoryQuery` (`tools.go:474`) and
  `registerMemorySearch` debug branch (`tools.go:664`) set `Source: r.Source`;
  `filterFields` handles the `source` field.
- Tool description updated to describe source-labeled results accurately.

## Files Changed

- `internal/search/search.go`: `Source` field on `Result`.
- `internal/search/service.go`: `tagSource` + per-leg tagging in `DebugSearch`.
- `internal/mcp/tools.go`: `mcpSearchResultItem.Source` + both branches + filter.
- `internal/mcp/debugsearch_mode_543_integration_test.go` (new) + search-layer tests.
- `.planning/phases/03-search-quality/design.md`: PR-D section (+ tie-rule precision edit).

## Findings Summary

- Root cause: `DebugSearch` RRF-merged 3 legs into a flat `[]Result` and `Result`
  had no source field → advertised source labels never emitted. Fixed by
  tagging pre-merge + surfacing the field.
- Tie rule (multi-leg collision) is deterministic at both dedup levels
  (RRFMerge by chunk-ID, DeduplicateResults by DocumentID; both first-seen,
  legs folded code→session→config).
- Debug mode is MCP-only — no REST caller of DebugSearch (no divergence).

## Resolution Status

- All in-scope resolved. R88 PASS (`docs/evidence/review-558.md`); the one MEDIUM
  (doc precision) fixed in design.md.
- build clean; integration debug/source tests PASS; `go test -race -short ./...` 31 pkgs ok.
- smoke:e2e PASS (MCP tools/call → source labels; `docs/evidence/smoke-e2e-debug-mode-source-labels.md`).
- Out of scope (still open on #543): trace collision (root-cause C), latency (Phase 17), ticket-recall pollution.

## Gemini Verification Triage

_Pending — populate after the Gemini bot reviews the PR (one row per inline
comment; verdict vocabulary per HARNESS.md § PR + Bot Review Loop)._

| Comment ref | Agent verdict | Reasoning | Action |
| --- | --- | --- | --- |
| _(none yet)_ | | | |
