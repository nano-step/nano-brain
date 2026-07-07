# Self-Review — Issue #567 (#542 F8: memory_flow builtin pollution)

Change-type: bug-fix · Lane: tiny · Branch: `fix/flow-builtins`
Author: kokorolx.

## Actions Taken

- `memory_flow` now drops `RoleExternal` nodes whose bare name resolves to no
  workspace symbol — the JS/TS builtins & keywords (`Number`, `catch`, `toFixed`,
  `push`, `for`, `get`) that polluted the graph. Added `dropExternalBuiltins`
  (`internal/mcp/tools.go`): resolves each distinct external name once via
  `ResolveSymbolByName`, drops zero-match nodes + the edges referencing them,
  keeps real leaves and non-external roles. Gated by a new `include_external`
  param (default false) for parity with `memory_trace`.
- `flow.BuildFlow` unchanged (still pure/edges-only); filtering is in the handler.
  Read-path only, no extraction/schema change.

## Files Changed

- `internal/mcp/tools.go` — `dropExternalBuiltins` + `include_external` param +
  schema + call after BuildFlow/stitch.
- `internal/mcp/flow_builtins_567_integration_test.go` — e2e through the handler:
  builtin dropped by default, real leaf kept, builtin returns with include_external.

## Findings Summary

- The right discriminator is symbol resolution, not a hand-maintained denylist: a
  workspace function named `push` resolves (kept) while the builtin `push` does
  not (dropped). Mirrors `memory_trace`.
- **Red-green proven**: with the filter disabled the integration test fails
  (`Number` leaks into default output); with it, `Number` is dropped and only
  reappears under `include_external`.
- No regression: `RoleHandler`/`Middleware`/`Integration` nodes untouched; real
  leaf functions kept; on a resolve error the node is kept (safe default).

## Resolution Status

- In scope resolved. No critical/major issues.
- `go build ./...` clean; `go test -race -short ./...` all ok.
- Integration (nanobrain_test): builtins-drop e2e test PASS; existing flow tests
  (#563/#564) still pass.
- smoke:e2e: `docs/evidence/smoke-e2e-flow-builtins.md` (MCP-over-HTTP on :3199 —
  builtin `Number` dropped by default, real leaf kept, returns with
  include_external). Dev DB never touched.

## Gemini Verification Triage

_Pending — populate after the Gemini bot reviews the PR._

| Comment ref | Agent verdict | Reasoning | Action |
| --- | --- | --- | --- |
| _(none yet)_ | | | |
