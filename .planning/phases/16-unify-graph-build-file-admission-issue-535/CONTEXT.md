# Phase 16 — Unify graph-build file admission (issue #535)

Lane: high-risk · Change type: bug-fix · Issue: #535

## Problem

nano-brain has two graph-build walks with divergent file-admission filters:

- `scanCollection` (startup/rescan) honors the nested `.gitignore` / `.nano-brainignore`
  stack (`GitignoreStack`) **and** the `storage.max_file_size` guard.
- `ReextractEdges/SymbolsForWorkspace` (reached via the `memory_update` MCP tool /
  `POST /api/v1/update`) consulted **only** the workspace-root `col.filter` — no nested
  gitignore stack, no size guard.

Result: in multi-repo workspaces, the re-extract path walks into each nested repo's
gitignored build output and oversized generated files, creating thousands of orphan
`graph_edges` (edges with no `documents` row) that poison `memory_trace/graph/symbols`.
`graph_edges` / `function_flowcharts` have **no FK to `documents`**, so document cleanup
never removed them, and `memory_update` re-created them on every refresh.

## Fix (this phase)

1. **Single admission gate.** New `walkAdmitter` (filter.go) encapsulates the nested
   ignore stack + `col.filter`. `scanCollection` and both `Reextract*` walks now share
   it, and the `Reextract*` walks also apply the `max_file_size` guard — the update path
   can no longer index a file the startup scan would skip.
2. **Cleanup deletes graph rows.** `cleanupIgnoredDocument`, `cleanupDeletedDocument`,
   `cleanupPathPrefix` now delete `graph_edges` + `function_flowcharts` for the file/prefix,
   matching both stored `source_file` formats (workspace-relative and absolute).
3. **Orphan sweep on re-extract.** `ReextractEdgesForWorkspace` records every admitted file
   and, after a clean walk, deletes graph rows for files not admitted (gitignored/oversized/
   deleted). No-op on an empty admitted set; skipped if any collection walk errored.

## Decisions (autonomous)

- **D-1 — Reuse one `walkAdmitter` across all three walks** rather than duplicating the
  gitignore-stack logic. Directly satisfies the issue's "single admission gate" and prevents
  future drift. `scanCollection` keeps its cleanup/watchDir side effects; only the
  skip-decision moved into the helper. The old per-file "loaded nested .gitignore" debug log
  was dropped (cosmetic).
- **D-2 — Raw SQL (`= ANY`/`<> ALL($n::text[])`) for graph-row deletes**, not new sqlc
  queries. `sqlc` is not on PATH here; `cleanupPathPrefix` already uses raw `ExecContext`;
  pgx v5 stdlib binds a Go `[]string` to `text[]` (same path the generated `ANY(...)`
  queries use). Smaller diff, no codegen dependency.
- **D-3 — Sweep runs once per workspace with the UNION of all collections' admitted paths.**
  Conservative: a row is kept if its file was admitted in any collection. Guards: empty-set →
  no-op; any walk error → skip (an under-populated admitted set must never wipe live rows).
- **D-4 — `deleteGraphRowsForFile` no-ops when `w.db == nil`.** Production `New()` always
  supplies a DB; only the mock-querier unit tests pass nil. Graph-delete behavior is covered
  by integration tests against real Postgres.

## Deferred (follow-up, NOT in this PR)

- **#4 path-format canonicalization** between `documents.source_path` (absolute) and
  `graph_edges.source_file` (relative). A data migration + broad change; ties into #501.
- **#5 first-class prune** MCP tool / `nano-brain cleanup-graph` CLI. New feature surface.

## Evidence

- Unit: `TestWalkAdmitter_NestedGitignore` (nested .gitignore honored during walk).
- Integration (nanobrain_test): `TestSweepOrphanGraphRows`, `TestDeleteGraphRowsForFile`.
- `go build ./...`, `go test -race -short ./...`, `go test -race -tags=integration ./internal/watcher/` all green.
