# Fix Summary Workspace-Registration Leaks

## Issue
[#238 — fix(summary): close 6 workspace-registration leak points in summary feature](https://github.com/nano-step/nano-brain/issues/238)

## Lane
high-risk — touches data-model (new FK migration) + audit-security (workspace registration enforcement) + existing behavior (harvester orphan handling).

## Why
The nano-brain summary feature has **6 confirmed leak points** where workspace registration is not enforced. When `summarization.enabled: true`, any of these paths can create `documents` rows with `workspace_hash` values that do not exist in the `workspaces` table — producing orphan summary documents that bypass workspace isolation and pollute cross-workspace queries.

User report: many `session-summary` documents observed in DB whose `workspace_hash` does not match any registered workspace. RRI-T test cycle at `ai/test-case/rri-t/summary/` confirmed 5 of 6 leaks at code level (static analysis) and inferred the 6th from supporting evidence.

The 6 leak points:
1. **Handler trust** (`internal/server/middleware.go:100-133` + `handlers/summarize.go:54`) — middleware only checks workspace is non-empty; no registration lookup.
2. **Claude Code harvester** (`cmd/nano-brain/main.go:371-393`) — computes `WorkspaceHash(SessionDir)` and harvests without verifying the hash is registered.
3. **Persister.Save** (`internal/summarize/persist.go:42-128`) — trusts `meta.WorkspaceHash` from caller; zero validation calls.
4. **HarvestSummarizer** (`internal/summarize/harvest_adapter.go:22-48`) — pure passthrough of `meta.WorkspaceHash` to persister.
5. **OpenCode fallback workspace** (`internal/harvest/opencode_sqlite.go:141-164`) — orphan sessions (no `worktree`) fall back to `WorkspaceHash(dbPath)`, an ephemeral hash that is never registered.
6. **Missing FK constraint** (migrations 00001-00010) — no DB-level enforcement that `documents.workspace_hash` references a row in `workspaces`.

## Desired Outcome
After this change, it MUST be impossible to create a `documents` row (any collection, but especially `session-summary`) under a `workspace_hash` that does not exist in the `workspaces` table. Enforcement happens at four layers (defense-in-depth):

1. **HTTP middleware** — rejects unregistered workspace_hash with HTTP 400 before reaching handlers (per-route flag for endpoints that need it).
2. **Harvester init** — refuses to start harvesting when configured session_dir maps to an unregistered hash.
3. **Persister.Save** — rejects writes with unregistered workspace_hash at the persistence layer.
4. **PostgreSQL FK constraint** — rejects orphan inserts at the DB layer; cascades deletes when workspace is removed.

## Constraints
- Backward compatible for registered workspaces — no behavior change for the happy path.
- Migration 00011 MUST run cleanly on existing production DBs. Pre-migration cleanup of existing orphans is required (provided as a separate command + dry-run flag).
- No regression in harvester normal flow: registered worktrees continue to harvest + summarize as before.
- Localhost-only binding (current default) is sufficient for trust boundary — no new auth layer added.
- No new external dependencies.
- Tests MUST cover both positive (registered workspace succeeds) and negative (unregistered workspace fails) cases for each layer.

## Out of Scope
- Authentication/authorization for HTTP endpoints (separate concern; current localhost binding is the trust boundary).
- Fixing the embed queue backpressure issue (operational, separate investigation).
- Cleanup of pre-existing orphan summary documents on production deployments (provided as one-off script `nano-brain cleanup-orphan-workspaces`, not auto-run on upgrade).
- Changes to the LLM summarization pipeline itself (strip/chunk/map/reduce logic unchanged).
- Resolving the workspace middleware "all" literal handling for write endpoints (F1 in test report — deferred to follow-up).

## Acceptance Criteria
1. **Persister rejects unregistered workspace_hash**: `Persister.Save()` returns a non-nil error containing `workspace_not_registered` when called with a hash absent from the `workspaces` table. Unit test asserts this.
2. **OpenCode harvester skips orphan sessions**: When an OpenCode session has no `worktree` OR its computed worktree hash is not registered, the session is logged at WARN and skipped. The handler does NOT fall back to `WorkspaceHash(dbPath)`. Integration test asserts no documents created for orphan sessions.
3. **Claude Code harvester refuses unregistered session_dir**: When `harvester.claudecode.enabled: true` and the computed `WorkspaceHash(session_dir)` is not in the `workspaces` table, the harvester logs WARN and does NOT start. Integration test asserts no documents created.
4. **Middleware rejects unregistered workspace** (opt-in per route): A new middleware variant `workspaceRegisteredMiddleware()` is applied to write endpoints (`/api/v1/summarize`, `/api/v1/write`, `/api/v1/embed`, `/api/v1/reindex`, `/api/v1/update`). Unit test asserts HTTP 400 with `error: workspace_not_registered` for unregistered hash.
5. **FK constraint enforced**: Migration 00011 adds `FOREIGN KEY (workspace_hash) REFERENCES workspaces(hash) ON DELETE CASCADE` to both `documents` and `chunks` tables. Integration test asserts orphan insert via direct SQL is rejected.
6. **Pre-migration cleanup**: New command `nano-brain cleanup-orphan-workspaces [--dry-run]` lists/deletes documents whose `workspace_hash` is not in `workspaces`. Migration 00011's `Up` block depends on this command being run first (operator-facing note in migration comment).
7. **No regression**: Existing test suite (`internal/summarize/*_test.go`, `internal/harvest/*_test.go`, `internal/server/handlers/*_test.go`) passes unchanged.
8. **User-flow test**: Enable summarization on port 8899 instance → run harvest on registered workspace → verify summaries created. Then attempt harvest with unregistered Claude Code session_dir → verify NO documents created. Evidence captured in `docs/evidence/fix-summary-workspace-registration-leaks/`.
9. **Validate ladder**: `validate:quick` + `test:integration` + `smoke:e2e` all green.
10. **Review Gate**: 5 parallel review sub-agents (review-work skill) all return PASS.

## Risk Flags
- [x] Data model (FK constraint addition; rejects existing orphans if cleanup not run first)
- [x] Audit/security (workspace registration enforcement)
- [x] Existing behavior (changes OpenCode fallback workspace, Claude Code init)
- [x] Weak proof (no existing tests for leak conditions — added in this change)

4 flags + 2 hard gates (data-model + audit-security) → HIGH-RISK confirmed.

## Migration Strategy
Operators upgrading from a version with orphan documents MUST run the cleanup command first:

```bash
# Dry-run first to see what will be deleted
nano-brain cleanup-orphan-workspaces --dry-run

# Apply
nano-brain cleanup-orphan-workspaces

# Then run migrations (migration 00011 will succeed)
nano-brain db:migrate
```

If migration 00011 runs without cleanup, it will fail with a PostgreSQL FK violation error pointing at the orphan rows. The release notes for this change will document this prerequisite explicitly.

## Test Evidence (Pre-Implementation)
RRI-T test cycle at `ai/test-case/rri-t/summary/` (already on `b-main`):
- 10 of 40 tests executed; 5 leak points statically confirmed
- 30 tests deferred to post-fix regression cycle
- Release gate verdict: NO-GO until fixes applied
