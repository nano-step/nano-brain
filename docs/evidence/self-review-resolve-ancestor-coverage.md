# Self-Review — Issue #565 (#542 F5: resolve surfaces ancestor coverage)

Change-type: bug-fix · Lane: tiny · Branch: `fix/resolve-ancestor-coverage`
Author: kokorolx.

## Actions Taken

- `memory_workspaces_resolve` now surfaces `covered_by` when the queried path is
  not itself registered but a registered **ancestor** workspace covers it.
- Added `mostSpecificAncestor` (pure, path-boundary matcher — longest ancestor
  wins) + `mcpCoveringAncestor` (thin `ListWorkspaces` wrapper) in
  `internal/mcp/tools.go`; the `sql.ErrNoRows` branch adds
  `covered_by:{workspace_hash,workspace_name,root_path,use}` when an ancestor
  exists. Read-only; no new query, no schema change.

## Files Changed

- `internal/mcp/tools.go` — `mostSpecificAncestor` + `mcpCoveringAncestor` + wire
  into the not-registered branch.
- `internal/mcp/resolve_ancestor_test.go` — white-box unit (sub-repo, deeper
  most-specific, trailing-slash, **shared-prefix sibling boundary**, unrelated,
  exact-skip).
- `internal/mcp/resolve_ancestor_565_integration_test.go` — e2e through the
  resolve handler (register parent → resolve child → covered_by; control unrelated).

## Findings Summary

- Boundary safety: `HasPrefix(absPath, TrimRight(w.Path,"/")+"/")` requires
  segment-aligned containment, so `/src/monorepo` does NOT match
  `/src/monorepo-api/x` (unit-tested). Exact path is skipped (that's the
  hash-resolved registered case).
- **Red-green proven**: with the matcher forced empty the integration test fails
  ("covered_by missing"); with it, `covered_by` points at the ancestor.
- No regression: registered (hash-hit) branch unchanged; `covered_by` added only
  when an ancestor exists (control asserts an unrelated path omits it).

## Resolution Status

- In scope resolved. No critical/major issues.
- `go build ./...` clean; `go test -race -short ./...` all ok (incl. white-box unit).
- Integration (nanobrain_test): unit + e2e handler test PASS.
- smoke:e2e: `docs/evidence/smoke-e2e-resolve-ancestor-coverage.md` (MCP-over-HTTP
  on :3199 — child path → covered_by ancestor). Dev DB never touched.

## Gemini Verification Triage

_Pending — populate after the Gemini bot reviews the PR._

| Comment ref | Agent verdict | Reasoning | Action |
| --- | --- | --- | --- |
| _(none yet)_ | | | |
