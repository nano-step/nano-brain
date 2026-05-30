# Oracle Review — opencode-multi-db-discovery

**Date**: 2026-05-29
**Reviewer**: Oracle (claude-opus-4.5-thinking)
**Verdict**: APPROVE-WITH-FIXES
**Duration**: 7m 16s

## Critical Issues — All Fixed
- **C1**: proposal.md line 24 contradicted design.md Decision 4 (claimed per-tick rescan).
  - **Fix**: Replaced with "discovery runs once at daemon startup; live rescan deferred."

## Major Issues — All Fixed
- **M1 (deferred)**: N harvesters each call `ListWorkspaces` per tick = N redundant PG queries.
  - **Decision**: Accept for v1 (N<20 typical). Add backlog item for shared cache.
- **M2 (fixed)**: Status endpoint needs Runner reference — tasks.md Task 6.4 expanded to specify injection at `srv.SetHealth(...)` and a new `Runner.HarvesterCount()`.
- **M3 (fixed)**: Trailing-slash in existing `HarvestAll` worktree lookup → silent miss.
  - **Fix**: Task 3.6b adds `filepath.Clean(sess.Worktree)` to the existing per-DB harvester (benefits all three modes).

## Minor — Applied
- Task 4.5b: log Warn for explicit `db_root` miss, Info for auto-detected.

## Missing Scenarios — Added
- Empty `db_root` (zero candidates) → fall-through scenario added
- Same worktree across multiple DBs → dedup-via-content-hash scenario added
- Multi-row `project` table → LIMIT 1 graceful handling scenario added

## Implementation Order — Adopted
- Wave 1 (parallel): Tasks 1, 2, 3 — independent files
- Wave 2: Task 4 — integration point
- Wave 3 (parallel): Tasks 5, 6, 7
- Wave 4 (parallel): Tasks 8, 9, 10

## Verdict
APPROVE — proceed to implementation with the fixes above applied.
