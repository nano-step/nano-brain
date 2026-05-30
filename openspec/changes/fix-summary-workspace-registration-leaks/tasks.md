# Tasks: Fix Summary Workspace-Registration Leaks

Tracking: #238

Tasks are ordered to minimize risk. Each task is independently committable; running validate:quick + relevant tests after each task is encouraged.

## Phase A — Foundations (no behavior change)

- [ ] **A1** — Add `GetWorkspaceByHash` sqlc query if not already present in `internal/storage/queries/workspaces.sql`. Verify by running `sqlc generate` and confirming the method exists in `internal/storage/sqlc/`.

- [ ] **A2** — Add `WorkspaceQuerier` interface in `internal/server/middleware.go` (or shared package) for dependency injection of `GetWorkspaceByHash` into middleware + persister. Verify: `go build ./...` green.

## Phase B — Persister defense (closes Leak #3 + #4)

- [ ] **B1** — Modify `internal/summarize/persist.go`: add workspace registration check at top of `Save()`. Return `workspace_not_registered` error if hash absent.

- [ ] **B2** — Write `internal/summarize/persist_security_test.go`:
  - Test: Save with unregistered hash → returns error matching `workspace_not_registered`
  - Test: Save with registered hash → succeeds (smoke check, mocks Pipeline + DB)
  - Test: Save with DB error during lookup → propagates wrapped error

- [ ] **B3** — Run `go test ./internal/summarize -race -short` → all green. Commit: `fix(summary): validate workspace registration in Persister.Save (#238)`

## Phase C — Harvester defenses (closes Leak #2 + #5)

- [ ] **C1** — Modify `internal/harvest/opencode_sqlite.go`:
  - Remove fallback workspace logic (no `WorkspaceHash(dbPath)` fallback)
  - Skip session with empty `worktree` (WARN log, return nil)
  - Look up `GetWorkspaceByHash(computed_hash)` per session; skip if unregistered (WARN log)

- [ ] **C2** — Extend `internal/harvest/opencode_sqlite_integration_test.go`:
  - Test: session with empty worktree → 0 documents created, WARN logged
  - Test: session with unregistered worktree → 0 documents created, WARN logged
  - Test: session with registered worktree → harvested as before

- [ ] **C3** — Modify `cmd/nano-brain/main.go` Claude Code harvester init block:
  - Add `GetWorkspaceByHash(wsHash)` lookup after `WorkspaceHash(SessionDir)`
  - On `ErrNoRows`: WARN log "session_dir not registered", do NOT add harvester to runner
  - On other error: WARN log, do NOT add harvester

- [ ] **C4** — Add unit/integration test for Claude Code init path:
  - Test: enabled=true + session_dir absent → no harvester started (existing)
  - Test: enabled=true + session_dir present + computed hash unregistered → no harvester started
  - Test: enabled=true + session_dir present + computed hash registered → harvester started

- [ ] **C5** — Run `go test ./internal/harvest ./cmd/nano-brain -race -short` → all green. Commit: `fix(harvest): skip unregistered workspaces in OpenCode + Claude Code harvesters (#238)`

## Phase D — Middleware enforcement (closes Leak #1)

- [ ] **D1** — Add `workspaceRegisteredMiddleware(q WorkspaceQuerier)` to `internal/server/middleware.go`. Implementation per `design.md` §3.4.

- [ ] **D2** — Extend `internal/server/middleware_test.go`:
  - Test: registered hash → next handler invoked, captured workspace matches
  - Test: unregistered hash → HTTP 400 + error="workspace_not_registered"
  - Test: workspace="all" → HTTP 400 + error="workspace_all_not_supported"
  - Test: DB error during lookup → HTTP 500
  - Test: empty workspace string (should not happen post-workspaceMiddleware) → HTTP 400 + error="workspace_required"

- [ ] **D3** — Modify `internal/server/routes.go`: apply `workspaceRegisteredMiddleware` to write group:
  - `POST /api/v1/summarize`
  - `POST /api/v1/write`
  - `POST /api/v1/embed`
  - `POST /api/v1/reindex`
  - `POST /api/v1/update`

- [ ] **D4** — Extend handler tests (`summarize_test.go`, `document_test.go`, etc.) to verify:
  - Unregistered workspace hash to write endpoints returns HTTP 400 (not 503 or 200 with empty result)
  - Registered workspace hash continues to work

- [ ] **D5** — Run `go test ./internal/server/... -race -short` → all green. Commit: `fix(server): reject unregistered workspace in write endpoint middleware (#238)`

## Phase E — Cleanup command (data hygiene before migration)

- [ ] **E1** — Add cleanup query to `internal/storage/queries/workspaces.sql`:
  - `CountOrphanDocumentsByWorkspace` — returns hash + count per orphan
  - `DeleteOrphanDocuments` — deletes documents where workspace_hash NOT IN workspaces
  - `DeleteOrphanChunks` — same for chunks

- [ ] **E2** — Implement `cmd/nano-brain/cleanup_orphan_workspaces.go` per `design.md` §3.6.

- [ ] **E3** — Register command in `cmd/nano-brain/commands.go` (or main cobra setup).

- [ ] **E4** — Write `cmd/nano-brain/cleanup_orphan_workspaces_test.go`:
  - Test: empty DB → "No orphan documents found"
  - Test: DB with orphans + --dry-run → reports counts, no changes
  - Test: DB with orphans + apply → orphans deleted, registered untouched
  - Test: DB with registered workspaces + orphans → only orphans deleted

- [ ] **E5** — Update README with cleanup command usage. Add to CLI commands table.

- [ ] **E6** — Run `go test ./cmd/nano-brain/... -race -short` → all green. Commit: `feat(cli): add cleanup-orphan-workspaces command (#238)`

## Phase F — DB constraint (closes Leak #6)

- [ ] **F1** — Create `migrations/00011_add_fk_documents_workspace.sql` per `design.md` §3.5. Include operator-facing comment at top.

- [ ] **F2** — Run `nano-brain db:migrate` against local PostgreSQL (clean schema first) → migration applies cleanly.

- [ ] **F3** — Verify FK enforcement via direct SQL:
  ```sql
  INSERT INTO documents (workspace_hash, source_path, content, content_hash, ...) VALUES ('not-registered-xyz', ...);
  -- Expected: ERROR: violates foreign key constraint "fk_documents_workspace"
  ```

- [ ] **F4** — Test migration on a DB with intentional orphans → migration fails with FK violation error message including the orphan hash(es). Confirms cleanup must run first.

- [ ] **F5** — Run `nano-brain cleanup-orphan-workspaces` on that DB → orphans removed → re-run migration → succeeds.

- [ ] **F6** — Verify down migration cleanly removes FK constraints. Commit: `feat(migration): add FK constraints documents/chunks → workspaces (#238)`

## Phase G — User-flow tests + evidence

- [ ] **G1** — On port 8899 isolated instance:
  - Apply this branch
  - Run `nano-brain cleanup-orphan-workspaces --dry-run` → expect 0 orphans (instance is currently clean per RRI-T Tier 0)
  - Run `nano-brain db:migrate` → migration 00011 succeeds

- [ ] **G2** — Flip `summarization.enabled: true` + set real `NANO_BRAIN_SUMMARIZE_API_KEY` → reload config

- [ ] **G3** — Trigger summarize against registered nano-brain workspace:
  ```bash
  curl -X POST http://localhost:8899/api/v1/summarize \
    -d '{"workspace":"7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f","source":"opencode","limit":1}'
  ```
  → Expect success, summary doc created.

- [ ] **G4** — Trigger summarize against UNREGISTERED workspace:
  ```bash
  curl -X POST http://localhost:8899/api/v1/summarize \
    -d '{"workspace":"fake_unregistered_xyz","source":"opencode","limit":1}'
  ```
  → Expect HTTP 400, error="workspace_not_registered"

- [ ] **G5** — Configure Claude Code harvester with unregistered session_dir + restart:
  - Expect server log: "claude code session_dir is not a registered workspace; harvester disabled"
  - Expect no documents created under that hash
  - Query verification:
    ```bash
    curl -X POST http://localhost:8899/api/v1/search \
      -d "{\"workspace\":\"$(echo -n /tmp/unregistered-cc | shasum -a 256 | awk '{print $1}')\",\"query\":\"session\"}"
    ```
    → 0 results

- [ ] **G6** — Direct SQL orphan insertion test (verifies FK):
  ```sql
  -- Connect to nanobrain_dev via psql or DBeaver
  INSERT INTO documents (id, workspace_hash, source_path, content, content_hash, collection, created_at, updated_at)
  VALUES (gen_random_uuid(), 'orphan-attempt-via-sql', '/test', 'test', 'hash', 'test', NOW(), NOW());
  -- Expected: PG error 23503 (foreign_key_violation)
  ```

- [ ] **G7** — Capture all 6 evidence outputs (curl outputs, log lines, SQL errors) to `docs/evidence/fix-summary-workspace-registration-leaks/`. Reference in story.md.

## Phase H — RRI-T Tier 3 regression

- [ ] **H1** — Re-run the 30 deferred RRI-T test cases (TC-001 through TC-040, excluding already-executed) against the fixed instance. Document results in `ai/test-case/rri-t/summary/04-execute.md` (append Tier 3 section).

- [ ] **H2** — Verify TC-001, TC-002, TC-003, TC-004, TC-005, TC-006 now PASS (previously FAIL).

- [ ] **H3** — Verify no regression in TC-009, TC-018 (previously PASS).

- [ ] **H4** — Update `ai/test-case/rri-t/summary/05-analyze.md` with post-fix coverage dashboard + release gate (target: GO).

- [ ] **H5** — Update `ai/test-case/rri-t/summary/summary.md` with verdict: GO.

## Phase I — Validate ladder

- [ ] **I1** — `validate:quick`: `go build ./... && go test -race -short ./...` → green.

- [ ] **I2** — `test:integration`: `go test -race -tags=integration ./...` → green.

- [ ] **I3** — `smoke:e2e`: Build binary → start server → curl summarize + write + reindex endpoints with registered/unregistered hashes → verify expected outcomes.

- [ ] **I4** — `self-review:staged-files`: Run `git status` before each commit; confirm no `.opencode/` or unrelated files staged.

- [ ] **I5** — `self-review:response-shape`: For each new handler error path, verify the JSON error envelope matches existing nano-brain conventions (`{"error":"...","message":"..."}`).

## Phase J — Review gate + PR

- [ ] **J1** — Trigger `review-work` skill (5 parallel sub-agents). Reviewer ≠ implementer. Paste verdict in story Evidence.

- [ ] **J2** — Address any review findings; rerun Phases I + H if implementation changed.

- [ ] **J3** — Push branch: `git push -u origin feat/238-fix-summary-workspace-leaks`

- [ ] **J4** — Open PR with title `fix(summary): close 6 workspace-registration leak points (#238)` and body containing `Closes #238`. Link RRI-T artifacts + evidence dir.

- [ ] **J5** — Bot review loop: address feedback, push again, repeat. Max 3 cycles → escalate to human.

- [ ] **J6** — On merge: verify issue #238 auto-closes via `Closes #238` keyword.

- [ ] **J7** — `openspec archive fix-summary-workspace-registration-leaks` → updates main specs.

- [ ] **J8** — Update `docs/TEST_MATRIX.md` with new test coverage from this change.

## Phase K — Release notes

- [ ] **K1** — Add release note entry:
  ```markdown
  ### Breaking (operator action required)

  - **Workspace registration is now enforced** for the summary feature and write endpoints. If you have orphan documents in your DB (documents whose `workspace_hash` is not in the `workspaces` table), you MUST run the new cleanup command BEFORE applying migration 00011:
    ```bash
    nano-brain cleanup-orphan-workspaces --dry-run  # inspect
    nano-brain cleanup-orphan-workspaces           # apply
    nano-brain db:migrate                          # then migrate
    ```
  - If the cleanup is skipped, migration 00011 will fail with a PostgreSQL foreign key violation error.
  ```

---

## Estimated Effort

| Phase | LoC | Estimate |
|-------|-----|----------|
| A — Foundations | ~10 | 0.5h |
| B — Persister | ~60 | 1h |
| C — Harvesters | ~80 | 2h |
| D — Middleware | ~125 | 2h |
| E — Cleanup command | ~140 | 2h |
| F — Migration | ~75 | 1h |
| G — User-flow tests | — | 1.5h |
| H — RRI-T regression | — | 2h |
| I — Validate | — | 0.5h |
| J — Review + PR | — | 2h (depends on bot review) |
| K — Release notes | ~20 | 0.5h |
| **Total** | **~510 LoC** | **~15h ≈ 2 days** |
