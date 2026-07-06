# Self-Review: Issue #535 — graph extractor bypasses document-indexer file filters → orphan edges

Change type: **bug-fix** · Lane: **high-risk** · Scope: `internal/watcher`

## Actions Taken
- Diagnosed (code + issue evidence) that the re-extract path
  (`ReextractEdges/SymbolsForWorkspace`, reached via the `memory_update` MCP tool /
  `POST /api/v1/update`) applied a weaker file filter than `scanCollection`: it
  consulted only the workspace-root `col.filter`, missing the nested
  `.gitignore`/`.nano-brainignore` stack (`GitignoreStack`) and the
  `storage.max_file_size` guard. In multi-repo workspaces it indexed each nested
  repo's gitignored build output + oversized generated files, creating orphan
  `graph_edges` (edges with no `documents` row) that poison
  `memory_trace/graph/symbols` — and re-created them on every `memory_update`.
- **Single admission gate:** extracted `walkAdmitter` (`filter.go`) holding the
  nested ignore stack + collection filter. `scanCollection` and both `Reextract*`
  walks now share it; the `Reextract*` walks also apply the `max_file_size` guard.
  The update path can no longer index a file the startup scan would skip.
- **Cleanup deletes graph rows:** `graph_edges`/`function_flowcharts` have no FK to
  `documents`, so `cleanupIgnoredDocument`/`cleanupDeletedDocument`/`cleanupPathPrefix`
  now delete them explicitly, matching both stored `source_file` formats
  (workspace-relative and absolute).
- **Orphan sweep on re-extract:** `ReextractEdgesForWorkspace` records every admitted
  file and, after a clean walk, deletes graph rows for files not admitted
  (gitignored/oversized/deleted).

## Files Changed
- `internal/watcher/filter.go` — `walkAdmitter` type + `ignore` method, `collRelFile` helper.
- `internal/watcher/watcher.go` — `scanCollection` refactored onto `walkAdmitter`;
  `ReextractSymbols/EdgesForWorkspace` use `walkAdmitter` + `max_file_size` guard;
  new `sweepOrphanGraphRows` + `deleteGraphRowsForFile`; graph-row deletion wired into
  the three cleanup functions.
- `internal/watcher/filter_test.go` — `TestWalkAdmitter_NestedGitignore` (unit).
- `internal/watcher/orphan_sweep_integration_test.go` — `TestSweepOrphanGraphRows`,
  `TestDeleteGraphRowsForFile` (integration).

## Findings Summary
- **Transient read / TOCTOU (reviewer MEDIUM):** originally `admitted[...]` was recorded
  after `os.ReadFile`; a transient read failure would drop a legit file from the set and
  the sweep would delete its live edges. **RESOLVED** — `admitted[...]` is now recorded
  immediately after the gate + size checks pass, before `ReadFile`.
- **Multi-collection sweep safety:** sweep runs once per workspace with the UNION of all
  collections' admitted paths (conservative — keeps a row if admitted in any collection);
  guarded by empty-set no-op and a `sweepSafe` flag that skips the sweep if any collection
  walk errored (ctx cancel / fatal walk error under-populates the set). No path deletes a
  legitimately-admitted file's rows.
- **Path-format handling:** deletes/sweep match both workspace-relative and absolute
  `source_file` forms, so the documents↔edges format split (deferred #4) does not cause
  missed rows.
- **nil-db guard:** `deleteGraphRowsForFile` no-ops when `w.db == nil` (mock-querier unit
  tests only; production `New()` always supplies a DB). Graph-delete behavior is covered
  by integration tests against real Postgres.
- **Deferred (out of scope, documented):** path-format canonicalization (#4, ties into
  #501) and a first-class prune MCP/CLI tool (#5).

## Resolution Status
- All findings: **RESOLVED**. No open critical/major issues.
- Independent review (R88, spawned `gsd-code-reviewer`, ≠ author): **Review Verdict: PASS**
  — no Critical/High blockers; two MEDIUM findings fixed (above).

## Validation
- `CGO_ENABLED=0 go build ./...` — PASS
- `go test -race -short ./...` — PASS (all packages)
- `go test -race -tags=integration ./internal/watcher/` — PASS (incl. the 3 new tests)
- Pre-existing/unrelated: `internal/summarize/persist_*` integration tests fail identically
  with this diff stashed (verified) — Phase 8 `sessions`/`session-summary` territory, and
  already present on `origin/master`; not caused by this change, out of scope.

## smoke:e2e
**N/A — no REST endpoint added or changed** (R19 applies to endpoint contract changes).
This bug-fix is internal to `internal/watcher`; `POST /api/v1/update` keeps its 202-queued
contract and the change is in the async re-extract behavior it triggers, which curl cannot
observe. The equivalent end-to-end verification is at the watcher layer, against real
Postgres (`nanobrain_test`): `TestSweepOrphanGraphRows` (orphans removed, admitted kept,
empty-set no-op), `TestDeleteGraphRowsForFile` (both path forms removed, unrelated files
untouched), and `TestWalkAdmitter_NestedGitignore` (nested `.gitignore` honored during a
real filesystem walk). Same treatment as the #497 watcher-internal bug-fix.

## Gemini Verification Triage

Gemini (gemini-code-assist) posted a review with 2 inline comments on PR #538. Both valid, both fixed.

| Comment ref       | Agent verdict  | Reasoning                                                                                                    | Action                        |
| ----------------- | -------------- | ------------------------------------------------------------------------------------------------------------ | ----------------------------- |
| line watcher.go:1218 | VALID:high     | A per-entry `WalkDir` error (e.g. EACCES on a subdir) returned nil without disabling the sweep; the skipped files never entered `admitted`, so the orphan sweep would delete their live graph rows. Real silent-data-loss bug. | fixed in commit b58ab0d — set `sweepSafe = false` on any per-entry walk error |
| line filter.go:100   | VALID:medium   | Repo convention: don't `os.Stat` before `gitignore.CompileIgnoreFile`; compile directly and check the error to drop a redundant syscall and not swallow non-NotExist errors.                                                  | fixed in commit b58ab0d — removed the `os.Stat` pre-checks |
