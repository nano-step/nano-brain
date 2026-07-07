# Self-Review — Issue #544 (N1/N2 already fixed, N3: memory_delete)

Change-type: user-feature · Lane: normal · Branch: `feat/memory-delete`
Author: kokorolx.

## Status of #544's 3 findings

- **N1 (`memory_impact direction:in` returns empty)** — already fixed by #570
  earlier this session (`GetIncomingEdges`/`GetImpactorsByTargets` bare-name
  fallback). Verified live: `TestMemoryImpact_RelativeInputAndOutput`
  (`internal/mcp/graph_paths_integration_test.go`) is exactly N1's scenario —
  `direction` defaults to `in`, `edge_type=calls`, a real caller edge — and it
  passes today (`go test -tags=integration -run TestMemoryImpact_RelativeInputAndOutput`).
- **N2 (`memory_flowchart` needs an exact line span; nothing surfaces it)** —
  already fixed: `memory_symbols` now returns `start_line`/`end_line`
  (`TestMemorySymbols_ExposesLineSpan` passes today), so the span is
  discoverable via `memory_symbols` before constructing the
  `file::start-end` flowchart node. The "accept `file::functionName`
  directly" half of N2 is a nice-to-have, not blocking (the tool's
  description already directs callers to `memory_symbols` first) — left as
  a future polish, not part of this PR.
- **N3 (no `memory_delete`; only `supersede`, which leaves a permanent
  tombstone)** — fixed by this PR.

## Actions Taken (N3)

- **`internal/mcp/tools.go`** — added `registerMemoryDelete`: resolves `path`
  via the same forms `memory_get` accepts (bare UUID, `#<uuid>` doc-or-chunk
  id, `source_path`) minus the `file::Symbol` graph-node form (that's
  code-derived, not a user-authored memory note — deleting it serves no
  purpose since the watcher recreates it). Reuses `resolveDocumentByAnyID`
  and the existing `DeleteDocumentByIDAndWorkspace :execrows` query (no new
  migration — `chunks.document_id` already has `ON DELETE CASCADE`).
  Workspace-scoped: the delete's `WHERE ... AND workspace_hash = $2` means a
  UUID belonging to a different workspace resolves to "not found", not a
  cross-workspace delete.
- **`internal/mcp/memory_delete_544_integration_test.go`** (new) — 4 tests:
  delete by UUID (chunks cascade, `memory_get` afterward cleanly errors),
  delete by `#<chunk-id>` (resolves to parent), unknown path (clean error),
  wrong workspace (does not cross-delete, document still exists in its own
  workspace).
- **`internal/mcp/tools_test.go`, `internal/mcp/concurrent_test.go`** — the
  two exhaustive tool-count/name assertions updated from 18→19 tools
  (`memory_delete` added). `internal/mcp/tools_schema_test.go`'s list is a
  fixed historical D-06 subset, not exhaustive — untouched, and
  `memory_delete`'s schema already matches that convention (workspace
  optional, matching `memory_get`'s exact pattern) by construction.

## Findings Summary

- No migration needed — reused an existing `:execrows` query and the
  existing cascade.
- Scoped to the memory-note use case in N3 (agent cleaning up its own
  `memory_write` output), not a general document-management API — matches
  what was asked, no speculative extra surface (no bulk-delete, no
  collection-wide delete).
- Red-green proven: all 4 new tests pass; `go build`/`go test -race -short
  ./...` green; two pre-existing exhaustive-count tests correctly caught the
  new tool and were updated (expected fallout, not a bug).
- No regression: full `internal/mcp` unit + integration suites green.

## Resolution Status

- N3 in scope resolved; N1/N2 verified already fixed by prior work.
- `go build ./...` clean; `go test -race -short ./...` all green.
- Integration (nanobrain_test): new `memory_delete` tests PASS; full
  graph+mcp integration suite PASS (excluding the pre-existing, unrelated
  #580).

## Gemini Verification Triage

_Pending — populate after the Gemini bot reviews the PR._

| Comment ref | Agent verdict | Reasoning | Action |
| --- | --- | --- | --- |
| _(none yet)_ | | | |
