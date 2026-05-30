# Fix Summary Workspace-Registration Leaks

## Issue
[#238 — fix(summary): close 6 workspace-registration leak points in summary feature](https://github.com/nano-step/nano-brain/issues/238)

## Lane
high-risk — touches data-model (new FK migration) + audit-security (workspace registration enforcement) + existing behavior (harvester orphan handling).

## Why
The nano-brain summary feature has **6 confirmed leak points** where workspace registration is not enforced. When `summarization.enabled: true`, any of these paths can create `documents` rows with `workspace_hash` values that do not exist in the `workspaces` table — producing orphan summary documents that bypass workspace isolation and pollute cross-workspace queries.

User report: many `session-summary` documents observed in DB whose `workspace_hash` does not match any registered workspace. RRI-T test cycle at `ai/test-case/rri-t/summary/` confirmed 5 of 6 leaks at code level (static analysis) and inferred the 6th from supporting evidence.

The 6 leak points + 1 follow-up gap (MCP path) found during deep-design:

1. **Handler trust** (`internal/server/middleware.go:100-133` + `handlers/summarize.go:54`) — middleware only checks workspace is non-empty; no registration lookup.
2. **Claude Code harvester** (`cmd/nano-brain/main.go:371-393`) — computes `WorkspaceHash(SessionDir)` and harvests without verifying the hash is registered.
3. **Persister.Save** (`internal/summarize/persist.go:42-128`) — trusts `meta.WorkspaceHash` from caller; zero validation calls.
4. **HarvestSummarizer** (`internal/summarize/harvest_adapter.go:22-48`) — pure passthrough of `meta.WorkspaceHash` to persister.
5. **OpenCode fallback workspace + auto-registration** (`internal/harvest/opencode_sqlite.go:141-164`) — has TWO issues: (a) orphan sessions (no `worktree`) fall back to `WorkspaceHash(dbPath)`, an ephemeral hash; (b) the harvester also AUTO-REGISTERS the worktree via `UpsertWorkspace` (line 155-163), silently extending the trust boundary without operator consent.
6. **Missing FK constraint** (migrations 00001-00010) — no DB-level enforcement that `documents.workspace_hash` references a row in `workspaces`.
7. **MCP write tools** (`internal/mcp/tools.go:485-659`) — `memory_write` calls `UpsertDocument`/`UpsertChunk` directly. The MCP transport (`/mcp`, `/sse` in `routes.go:73-78`) does NOT go through Echo HTTP middleware, so `workspaceRegisteredMiddleware` does not protect it. Validation must be added directly inside the MCP tool handler.

## Desired Outcome
After this change, it MUST be impossible to create a `documents` row (any collection, but especially `session-summary`) under a `workspace_hash` that does not exist in the `workspaces` table. Enforcement happens at five layers (defense-in-depth):

1. **HTTP middleware** — rejects unregistered workspace_hash with HTTP 400 before reaching handlers (per-route flag for endpoints that need it).
2. **MCP tool handler** — `memory_write` and `memory_update` validate workspace registration before calling UpsertDocument/UpsertChunk (MCP transport bypasses HTTP middleware).
3. **Harvester init** — refuses to start harvesting when configured session_dir maps to an unregistered hash. No auto-registration of unregistered worktrees.
4. **Persister.Save** — rejects writes with unregistered workspace_hash at the persistence layer.
5. **PostgreSQL FK constraint** — rejects orphan inserts at the DB layer; cascades deletes when workspace is removed.

## Constraints
- Backward compatible for registered workspaces — no behavior change for the happy path.
- Migration 00011 MUST run cleanly on existing production DBs. Pre-migration cleanup of existing orphans is required (provided as a separate command + dry-run flag).
- No regression in harvester normal flow: registered worktrees continue to harvest + summarize as before.
- **Breaking change accepted:** Operators who relied on implicit auto-registration via the OpenCode harvester (which previously called `UpsertWorkspace` for every worktree it discovered) MUST now explicitly register workspaces via `POST /api/v1/init` or `nano-brain init --root=<path>`. This is called out in release notes.
- Localhost-only binding (current default) is sufficient for trust boundary — no new auth layer added.
- No new external dependencies.
- Tests MUST cover both positive (registered workspace succeeds) and negative (unregistered workspace fails) cases for each layer.
- Code patterns MUST match existing codebase: on-demand `q := sqlc.New(db)` (no shared `WorkspaceQuerier` interface), plain `func cmd(args []string) error` signatures (no Cobra), inline error code strings consistent with existing style.

## Out of Scope
- Authentication/authorization for HTTP endpoints (separate concern; current localhost binding is the trust boundary).
- Fixing the embed queue backpressure issue (operational, separate investigation).
- Auto-running cleanup as part of `db:migrate` (operator must explicitly run cleanup → migrate → start new binary).
- Changes to the LLM summarization pipeline itself (strip/chunk/map/reduce logic unchanged).
- FK constraints on `embeddings.workspace_hash` and `collections.workspace_hash` — embeddings are covered transitively via `chunks(id) → embeddings(chunk_id)` cascade; collections are operator-managed and low-risk.
- Telemetry metric for rejected-workspace events (WARN logs are sufficient for v1; metric deferred).
- Extracting all error codes into shared `const` block (style follow-up; not blocking).
- Adding `--output-json` flag to cleanup command for backup (operator MUST run `--dry-run` and redirect output to a file as a manual backup; first-class backup deferred to follow-up).
- RRI-T Tier 3 regression (30 test cases) — moved to a SEPARATE follow-up task post-merge to keep PR scope bounded.

## Acceptance Criteria
1. **Persister rejects unregistered workspace_hash**: `Persister.Save()` returns a non-nil error containing `workspace_not_registered` when called with a hash absent from the `workspaces` table. Unit test asserts this; integration test against real PG asserts schema correctness end-to-end.
2. **OpenCode harvester skips orphan sessions AND stops auto-registering**: (a) When an OpenCode session has no `worktree` OR its computed worktree hash is not registered, the session is logged at WARN and skipped. (b) The harvester NO LONGER calls `UpsertWorkspace` (auto-registration removed). (c) No fallback to `WorkspaceHash(dbPath)`. Integration test asserts no documents created for orphan sessions and no workspaces auto-registered.
3. **Claude Code harvester refuses unregistered session_dir**: When `harvester.claudecode.enabled: true` and the computed `WorkspaceHash(session_dir)` is not in the `workspaces` table, the harvester logs WARN and does NOT start. The Claude Code harvester init function is extracted from `main.go` into a testable function `initClaudeCodeHarvester(ctx, cfg, db, logger) (harvester, error)`. Integration test asserts no documents created.
4. **HTTP middleware rejects unregistered workspace** (opt-in per route): A new middleware variant `workspaceRegisteredMiddleware()` is applied to HTTP write endpoints (`/api/v1/summarize`, `/api/v1/write`, `/api/v1/embed`, `/api/v1/reindex`, `/api/v1/update`). Unit test asserts HTTP 400 with `error: workspace_not_registered` for unregistered hash. Read endpoints (query/search/vsearch/get/multi-get/wake-up) and management endpoints (init/workspaces) are NOT affected.
5. **MCP tool handlers reject unregistered workspace**: `memory_write` and `memory_update` in `internal/mcp/tools.go` call `GetWorkspaceByHash` after extracting workspace from tool args and return a clear error before invoking `UpsertDocument`/`UpsertChunk`. Unit/integration tests assert that MCP tool calls with unregistered workspace return an error with message `workspace_not_registered`.
6. **FK constraint enforced**: Migration 00011 adds `FOREIGN KEY (workspace_hash) REFERENCES workspaces(hash) ON DELETE CASCADE` to both `documents` and `chunks` tables. Integration tests assert: (a) orphan INSERT is rejected with PG error 23503 referencing `fk_documents_workspace`; (b) orphan UPDATE (changing workspace_hash to an unregistered value) is also rejected; (c) workspace deletion cascades to documents AND chunks; (d) down migration cleanly drops constraints without deleting data.
7. **Pre-migration cleanup**: New command `nano-brain cleanup-orphan-workspaces [--dry-run]` lists/deletes documents AND chunks whose `workspace_hash` is not in `workspaces`. Output reports counts for documents + chunks + (transitively-deleted) embeddings. Pre-flight `/health` check warns if server is running. Migration 00011's `Up` block has operator-facing comment requiring cleanup first.
8. **No regression**: Existing test suite (`internal/summarize/*_test.go`, `internal/harvest/*_test.go`, `internal/server/handlers/*_test.go`, `internal/mcp/*_test.go`) passes unchanged.
9. **User-flow test (non-LLM)**: Apply branch on port 8899 instance, run cleanup → migrate → start. Send write to unregistered workspace via HTTP, MCP, and via OpenCode harvester orphan session. All three paths must be rejected/skipped. Evidence in `docs/evidence/fix-summary-workspace-registration-leaks/`.
10. **Validate ladder**: `validate:quick` + `test:integration` + `smoke:e2e` all green.
11. **Review Gate**: 5 parallel review sub-agents (review-work skill) all return PASS.
12. **Release notes**: Document (a) operator upgrade sequence (stop → cleanup → migrate → start new), (b) auto-registration removal as breaking change, (c) HTTP status change 503→400 for unregistered workspace on write endpoints.

## Risk Flags
- [x] Data model (FK constraint addition; rejects existing orphans if cleanup not run first)
- [x] Audit/security (workspace registration enforcement)
- [x] Existing behavior (changes OpenCode fallback workspace, Claude Code init)
- [x] Weak proof (no existing tests for leak conditions — added in this change)

4 flags + 2 hard gates (data-model + audit-security) → HIGH-RISK confirmed.

## Migration Strategy

**The full upgrade sequence is: STOP → CLEANUP → MIGRATE → START NEW BINARY.** Skipping any step risks re-introducing orphans via a stale binary.

```bash
# 1. STOP the running nano-brain server (prevents stale binary from re-creating orphans)
# Example: kill $(cat /tmp/nano-brain/server.pid) or via your process manager

# 2. CLEANUP — dry-run first to see what will be deleted
nano-brain cleanup-orphan-workspaces --dry-run > /tmp/nano-brain-orphans-backup.txt
cat /tmp/nano-brain-orphans-backup.txt  # review

# Apply cleanup (deletions are PERMANENT — save dry-run output as your record)
nano-brain cleanup-orphan-workspaces

# 3. MIGRATE — add FK constraints (will FAIL if orphans remain)
nano-brain db:migrate

# 4. START the new binary with the fix applied
nano-brain
```

**Critical operator notes:**
- **DO NOT** run cleanup while the old binary is running — concurrent harvests could re-create orphans between listing and deletion.
- **DO NOT** start the old binary after migration — the old harvester's `UpsertWorkspace` call will auto-register orphan worktrees, defeating the fix. Always start the NEW binary after migration.
- **Orphan summaries are permanently deleted.** They contain LLM-generated content that cannot be re-generated (the underlying sessions will now be skipped). If you need to preserve them, back up the database before cleanup or save the `--dry-run` output.
- **On large tables (>1M documents):** migration 00011 validates every existing row and acquires a `ShareRowExclusiveLock`. Expect 30-60 seconds of blocked writes. For very large deployments, consider using `ADD CONSTRAINT ... NOT VALID` + `VALIDATE CONSTRAINT` as a manual two-step (not the default; document as advanced option).

If migration 00011 runs without cleanup, it will fail with a PostgreSQL FK violation error identifying constraint `fk_documents_workspace` and at least one violating workspace_hash. Re-run the cleanup command and retry the migration.

## Test Evidence (Pre-Implementation)
RRI-T test cycle at `ai/test-case/rri-t/summary/` (already on `b-main`):
- 10 of 40 tests executed; 5 leak points statically confirmed
- 30 tests deferred to post-fix regression cycle
- Release gate verdict: NO-GO until fixes applied
