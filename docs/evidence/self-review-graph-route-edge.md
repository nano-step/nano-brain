# Self-Review — Issue #569 (#542 F6: memory_graph reverse route edge)

Change-type: bug-fix · Lane: tiny · Branch: `fix/graph-route-edge`
Author: kokorolx.

## Actions Taken

- `memory_graph(direction="in")` now surfaces the route→handler `http` edge for a
  handler queried by its qualified `file::symbol` node. Added a third disjunct to
  `GetIncomingEdges`: `OR (strpos($2::text,'::') > 0 AND target_node =
  split_part($2::text,'::',2))` — bridging a qualified query against a BARE stored
  target (http edges store `target_node` as the bare handler name).
- Edited both the source (`internal/storage/queries/graph.sql`) and the generated
  const (`internal/storage/sqlc/graph.sql.go`) identically, hand-synced because
  sqlc isn't installed here; no signature change so a future `sqlc generate` is a
  no-op. Fixes memory_graph in/both across MCP + REST + neighborhood (all share
  the query).

## Files Changed

- `internal/storage/queries/graph.sql` + `internal/storage/sqlc/graph.sql.go` —
  `GetIncomingEdges` third disjunct (byte-identical WHERE clause in both).
- `internal/mcp/graph_route_edge_569_integration_test.go` — e2e through the
  memory_graph handler: qualified handler node, direction=in → http + contains.

## Findings Summary

- The `strpos($2,'::') > 0` guard makes the new disjunct fire only for qualified
  queries, so a bare query never spuriously matches `target_node = ''`.
- Broadening the reverse match is semantically correct for every caller (the
  reverse graph SHOULD include the route→handler edge) — same fix shape as impact
  #553 (which also touched MCP + REST).
- **Red-green proven**: with the SQL fix stashed the integration test fails (only
  `contains` returned); with it, both `http` + `contains` are returned.
- `TestGetIncomingEdges_SymbolFallback` (the query's existing bare↔qualified test)
  still passes.

## Resolution Status

- In scope resolved. No critical/major issues.
- `go build ./...` clean; `go test -race -short ./...` all ok.
- Integration (nanobrain_test): route-edge e2e test PASS.
- Pre-existing (NOT this change): `TestMemoryGraph_Relative*` fail — confirmed
  failing identically on clean master with this change stashed (tracked as #556).
- smoke:e2e: `docs/evidence/smoke-e2e-graph-route-edge.md` (MCP-over-HTTP on :3199
  — direction=in returns http + contains). Dev DB never touched.

## Gemini Verification Triage

_Pending — populate after the Gemini bot reviews the PR._

| Comment ref | Agent verdict | Reasoning | Action |
| --- | --- | --- | --- |
| _(none yet)_ | | | |
