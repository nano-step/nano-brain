# Review Gate: Issue #535 — unify graph-build file admission

Review Verdict: PASS
Reviewer: gsd-code-reviewer (independent spawned sub-agent)
Date: 2026-07-06
Commit: ef5493d

The review was performed by a separate `gsd-code-reviewer` agent spawned with only
the diff + issue context (R88 — the implementer did not review its own code).

## Acceptance criteria

| Criterion (from issue #535) | Evidence | Status |
|---|---|---|
| Re-extract path applies the SAME admission gate as `scanCollection` (nested `.gitignore`/`.nano-brainignore` stack + `max_file_size`) | Shared `walkAdmitter` used by `scanCollection` and both `Reextract*` walks; size guard added to the walks. Unit test `TestWalkAdmitter_NestedGitignore` (nested `.gitignore` honored on a real walk). | ✓ |
| A file is in both `documents` and graph, or neither — no file has edges but no document | Admission gate (forward) + orphan sweep (reconcile existing) + cleanup deletes graph rows (lifecycle). | ✓ |
| Cleanup deletes graph rows (no FK to `documents`) | `cleanupIgnoredDocument`/`cleanupDeletedDocument`/`cleanupPathPrefix` delete `graph_edges`+`function_flowcharts`, both path forms. `TestDeleteGraphRowsForFile` (integration). | ✓ |
| Orphan sweep on reindex | `sweepOrphanGraphRows` at end of `ReextractEdgesForWorkspace`; empty-set no-op; skipped on walk error. `TestSweepOrphanGraphRows` (integration). | ✓ |
| Deferred (#4 path canonicalization, #5 prune tool) explicitly out of scope | Documented in PR body + `.planning/phases/16-.../CONTEXT.md`. | ✓ (deferred) |

## Findings

- **MEDIUM — transient `ReadFile` / TOCTOU could drop an admitted file from the set** → orphan sweep would delete its live edges. **FIXED** — `admitted[...]` recorded immediately after the gate + size checks pass, before `ReadFile`.
- **MEDIUM — per-entry `WalkDir` error (later raised as HIGH by the PR bot)** — a localized error left `sweepSafe` true. **FIXED in ef5493d** — `sweepSafe = false` on any per-entry walk error.

No Critical/High blockers. Verdict: **PASS**.

## Validation referenced

- `go build ./...` — PASS
- `go test -race -short ./...` — PASS (all packages)
- `go test -race -tags=integration ./internal/watcher/` — PASS (incl. 3 new tests)
