# Tasks: Fix Summary Workspace-Registration Leaks

Tracking: #238

Tasks are ordered to minimize risk. Each task is independently committable; running validate:quick + relevant tests after each task is encouraged.

## Phase A — Foundations (no behavior change)

- [x] **A1** — `GetWorkspaceByHash` already exists at `internal/storage/queries/workspaces.sql:10-11` (verified via deep-design exploration). NO ACTION NEEDED. If `sqlc generate` is re-run, confirm the method signature is unchanged.

- [x] **A2** — DECIDED: NO shared `WorkspaceQuerier` interface. Each consumer (middleware, persister, harvester, mcp tools, cleanup command) uses on-demand `q := sqlc.New(db)` matching the existing codebase pattern (see `internal/harvest/opencode_sqlite.go:115`). This task is a documentation note only.

## Phase B — Persister defense (closes Leak #3 + #4)

- [x] **B1** — Modify `internal/summarize/persist.go`: add `q := sqlc.New(p.db)` + `GetWorkspaceByHash` check at top of `Save()`. Return error containing `workspace_not_registered` if hash absent. Use `errors.Is(err, sql.ErrNoRows)` to distinguish "not found" from "DB error".

- [x] **B2** — Write `internal/summarize/persist_security_test.go` (unit-level, sqlmock):
  - Test: Save with unregistered hash → returns error matching `workspace_not_registered`
  - Test: Save with registered hash → proceeds to existing logic (smoke check)
  - Test: Save with DB error during lookup → propagates wrapped error

- [x] **B2.5** — Write integration test (real PostgreSQL) in `internal/summarize/persist_integration_test.go`:
  - Setup: real DB, insert a workspace row, call Save with that hash → success + document persisted
  - Setup: real DB, no matching workspace row, call Save → returns `workspace_not_registered`, no document persisted

- [x] **B3** — Run `go test ./internal/summarize -race -short` → all green. Commit: `fix(summary): validate workspace registration in Persister.Save (#238)`

## Phase C — Harvester defenses (closes Leak #2 + #5)

- [x] **C1** — Modify `internal/harvest/opencode_sqlite.go` (THREE changes — all required):
  1. **Remove fallback workspace logic** — no `WorkspaceHash(dbPath)` fallback. Empty `session.Worktree` → WARN log + skip (return nil, continue to next session).
  2. **Remove auto-registration** — DELETE the `UpsertWorkspace` call at lines 155-163. This is a breaking change; the harvester no longer silently creates workspace entries.
  3. **Add registration check** — `q := sqlc.New(h.pgDB)` + `GetWorkspaceByHash(ctx, workspaceHash)` per session. On `sql.ErrNoRows` → WARN log + skip. On other DB error → return err.

- [x] **C2** — Extend `internal/harvest/opencode_sqlite_integration_test.go` (fixtures: create temp SQLite DB with OpenCode schema — see existing `opencode_sqlite_integration_test.go` for pattern; seed with 3 session variants):
  - Test: session with empty worktree → 0 documents created, NO workspace auto-registered, WARN logged
  - Test: session with worktree pointing to `/tmp/unregistered-path` → 0 documents created, NO workspace auto-registered, WARN logged
  - Test: session with worktree matching a pre-registered workspace → harvested as before, no extra UpsertWorkspace call observed

- [x] **C3** — Extract Claude Code init from `cmd/nano-brain/main.go` to new file `cmd/nano-brain/claudecode_init.go`. Function signature: `func initClaudeCodeHarvester(ctx context.Context, cfg ClaudeCodeConfig, db *sql.DB, logger zerolog.Logger) (*harvest.ClaudeCodeHarvester, error)`. Returns `(nil, nil)` if harvester should not be started (disabled, dir missing, or workspace unregistered).

- [x] **C4** — Write `cmd/nano-brain/claudecode_init_test.go`:
  - Test: enabled=false → returns (nil, nil)
  - Test: enabled=true + session_dir absent → returns (nil, nil), WARN logged
  - Test: enabled=true + session_dir present + unregistered hash → returns (nil, nil), WARN logged with "run nano-brain init" guidance
  - Test: enabled=true + session_dir present + registered hash → returns valid harvester
  - Test: DB error during lookup → returns (nil, nil), ERROR logged

- [x] **C5** — Run `go test ./internal/harvest ./cmd/nano-brain -race -short` → all green. Commit: `fix(harvest): skip unregistered workspaces in OpenCode + Claude Code harvesters; remove auto-registration (#238)`

## Phase D — HTTP Middleware enforcement (closes Leak #1)

- [x] **D1** — Add `workspaceRegisteredMiddleware(db *sql.DB)` to `internal/server/middleware.go`. Implementation per `design.md` §3.4. Uses on-demand `q := sqlc.New(db)` per request — no shared interface.

- [x] **D2** — Extend `internal/server/middleware_test.go`:
  - Test: registered hash → next handler invoked, captured workspace matches
  - Test: unregistered hash → HTTP 400 + error="workspace_not_registered"
  - Test: workspace="all" → HTTP 400 + error="workspace_all_not_supported"
  - Test: DB error during lookup → HTTP 500
  - Test: empty workspace string (should not happen post-workspaceMiddleware) → HTTP 400 + error="workspace_required"

- [x] **D3** — Modify `internal/server/routes.go`: apply `workspaceRegisteredMiddleware` to write group:
  - `POST /api/v1/summarize`
  - `POST /api/v1/write`
  - `POST /api/v1/embed`
  - `POST /api/v1/reindex`
  - `POST /api/v1/update`

- [x] **D4** — Extend handler tests (`summarize_test.go`, `document_test.go`, etc.) to verify:
  - Unregistered workspace hash to write endpoints returns HTTP 400 (not 503 or 200 with empty result)
  - Registered workspace hash continues to work

- [x] **D5** — Run `go test ./internal/server/... -race -short` → all green. Commit: `fix(server): reject unregistered workspace in write endpoint middleware (#238)`

## Phase D' — MCP tool enforcement (closes new leak #7 — MCP bypass)

- [x] **D'1** — Modify `internal/mcp/tools.go`:
  - In `registerMemoryWrite` (around line 505): after non-empty workspace check, reject `workspace == "all"` and add `GetWorkspaceByHash` lookup before any UpsertDocument call at lines 590/623. Return `mcp.NewToolResultError("workspace_not_registered: ...")` if not found.
  - In `registerMemoryUpdate` (around line 745): same pattern (even though current implementation only queues reindex, validate workspace before queuing).

- [x] **D'2** — Write `internal/mcp/tools_security_test.go`:
  - Test: memory_write with registered workspace → succeeds, UpsertDocument called
  - Test: memory_write with unregistered workspace → returns error containing `workspace_not_registered`, UpsertDocument NOT called
  - Test: memory_write with `workspace: "all"` → returns error containing `workspace_all_not_supported`
  - Test: memory_update with unregistered workspace → returns error
  - Test: memory_write with DB lookup failure → returns error containing `workspace_lookup_failed`

- [x] **D'3** — Run `go test ./internal/mcp -race -short` → all green. Commit: `fix(mcp): reject unregistered workspace in memory_write and memory_update tool handlers (#238)`

## Phase E — Cleanup command (data hygiene before migration)

- [x] **E1** — Add cleanup query to `internal/storage/queries/workspaces.sql`:
  - `CountOrphanDocumentsByWorkspace` — returns hash + count per orphan
  - `DeleteOrphanDocuments` — deletes documents where workspace_hash NOT IN workspaces
  - `DeleteOrphanChunks` — same for chunks

- [x] **E2** — Implement `cmd/nano-brain/cmd_cleanup_orphan_workspaces.go` per `design.md` §3.6. Function signature: `func runCleanupOrphanWorkspacesCmd(args []string) error` (NO Cobra — use stdlib `flag.NewFlagSet`). Include pre-flight `GET /health` check on `:3100` and `:8899` with WARN if server is running. Output reports documents + chunks + transitively-deleted embeddings count.

- [x] **E3** — Register command in `cmd/nano-brain/commands.go` switch dispatcher (match `cleanup-stale-raw` pattern at the same location).

- [x] **E4** — Write `cmd/nano-brain/cmd_cleanup_orphan_workspaces_test.go`:
  - Test: empty DB → "No orphan documents found", exit 0
  - Test: DB with orphans + --dry-run → reports counts (docs, chunks, embeddings), no changes, exit 0
  - Test: DB with orphans + apply → orphans deleted, registered untouched, embeddings cascade-deleted via existing chunks(id)→embeddings FK
  - Test: DB connection failure → exit 1, clear error message
  - Test: pre-flight server detection logs WARN but does NOT abort (deletion still proceeds — warning is advisory)

- [x] **E5** — Update README with cleanup command usage. Add to CLI commands table. Include operator-facing upgrade sequence (STOP → CLEANUP → MIGRATE → START NEW).

- [x] **E6** — Run `go test ./cmd/nano-brain/... -race -short` → all green. Commit: `feat(cli): add cleanup-orphan-workspaces command (#238)`

## Phase F — DB constraint (closes Leak #6)

- [x] **F1** — Create `migrations/00011_add_fk_documents_workspace.sql` per `design.md` §3.5. Include operator-facing comment at top warning that orphans cause migration failure + reference cleanup command. Single `+goose StatementBegin/End` block containing both ALTER TABLE statements (atomic transaction).

- [x] **F2** — Run `nano-brain db:migrate` against local PostgreSQL (clean schema first, no orphans) → migration applies cleanly.

- [x] **F3** — Verify FK enforcement via direct SQL (both INSERT and UPDATE paths):
  ```sql
  -- INSERT path
  INSERT INTO documents (id, workspace_hash, source_path, content, content_hash, collection, title, tags, created_at, updated_at)
  VALUES (gen_random_uuid(), 'not-registered-xyz', '/test', 'x', 'h', 'c', 't', '{}', NOW(), NOW());
  -- Expected: ERROR: violates foreign key constraint "fk_documents_workspace"

  -- UPDATE path (catches code that mutates workspace_hash to an invalid value)
  UPDATE documents SET workspace_hash = 'not-registered-xyz' WHERE id = '<existing-id>';
  -- Expected: ERROR: violates foreign key constraint "fk_documents_workspace"

  -- Chunks (same constraint)
  INSERT INTO chunks (id, document_id, workspace_hash, ...) VALUES (..., 'not-registered-xyz', ...);
  -- Expected: ERROR: violates foreign key constraint "fk_chunks_workspace"
  ```

- [x] **F4** — Test migration on a DB with intentional orphans → migration fails with PostgreSQL error 23503 identifying constraint `fk_documents_workspace` and at least one violating key. Confirms cleanup must run first.

- [x] **F5** — Run `nano-brain cleanup-orphan-workspaces` on that DB → orphans removed → re-run migration → succeeds.

- [x] **F6** — Verify down migration cleanly removes both FK constraints AND does NOT delete any data. Add test case: documents/chunks/embeddings counts before and after `goose down` should be identical.

- [x] **F7** — Verify cascade: insert workspace W with documents + chunks, DELETE workspace row from `workspaces`, verify documents AND chunks AND embeddings (via transitive chunks→embeddings cascade) are all deleted.

- [x] **F8** — Commit: `feat(migration): add FK constraints documents/chunks → workspaces (#238)`

## Phase G — User-flow tests + evidence (non-LLM where possible)

- [x] **G1** — On port 8899 isolated instance:
  - Stop existing server: `kill $(cat /tmp/nano-brain-custom/server.pid)`
  - Build branch binary: `CGO_ENABLED=0 go build -o /tmp/nano-brain-fix238 ./cmd/nano-brain`
  - Run `/tmp/nano-brain-fix238 cleanup-orphan-workspaces --dry-run` → expect "No orphan documents found" (instance is currently clean per RRI-T Tier 0)
  - Run `NANO_BRAIN_CONFIG=/tmp/nano-brain-custom/config.yml /tmp/nano-brain-fix238 db:migrate` → migration 00011 succeeds
  - Start the new binary

- [x] **G2** — TEST: HTTP write to unregistered workspace returns 400 (does NOT require summarization):
  ```bash
  curl -s -w "\nHTTP:%{http_code}\n" -X POST http://localhost:8899/api/v1/write \
    -H "Content-Type: application/json" \
    -d '{"workspace":"fake_unregistered_xyz","source_path":"/test","content":"x","tags":["test"]}'
  ```
  → Expect HTTP 400, error="workspace_not_registered". Capture to evidence.

- [x] **G3** — TEST: HTTP write to registered workspace succeeds:
  ```bash
  WS=$(curl -s http://localhost:8899/api/v1/workspaces | jq -r '.[0].workspace_hash')
  curl -s -w "\nHTTP:%{http_code}\n" -X POST http://localhost:8899/api/v1/write \
    -H "Content-Type: application/json" \
    -d "{\"workspace\":\"$WS\",\"source_path\":\"/rrit/g3\",\"content\":\"g3-test\",\"tags\":[\"test\"]}"
  ```
  → Expect HTTP 200 + doc ID. Capture.

- [x] **G4** — TEST: HTTP write with `workspace: "all"` returns 400:
  ```bash
  curl -s -w "\nHTTP:%{http_code}\n" -X POST http://localhost:8899/api/v1/write \
    -d '{"workspace":"all","source_path":"/test","content":"x","tags":[]}'
  ```
  → Expect HTTP 400, error="workspace_all_not_supported".

- [x] **G5** — TEST: MCP memory_write to unregistered workspace returns error:
  Use MCP client or `curl /mcp` JSON-RPC call invoking `memory_write` tool with `{"workspace":"fake_unregistered_xyz", ...}`.
  → Expect tool result error containing `workspace_not_registered`. Capture stdout+log.

- [x] **G6** — TEST: Claude Code harvester with unregistered session_dir:
  - Edit `/tmp/nano-brain-custom/config.yml`: `harvester.claudecode.enabled: true`, `session_dir: /tmp/rrit-unregistered-cc`
  - `mkdir -p /tmp/rrit-unregistered-cc/projects/x && echo '{"type":"summary"}' > /tmp/rrit-unregistered-cc/projects/x/1.jsonl`
  - Restart binary → expect WARN log: "claude code session_dir is not a registered workspace; harvester disabled"
  - Verify no documents under computed hash via search API.

- [x] **G7** — TEST: Direct SQL orphan INSERT and UPDATE rejected (FK):
  ```sql
  INSERT INTO documents (id, workspace_hash, ...) VALUES (gen_random_uuid(), 'orphan-attempt-xyz', ...);
  -- Expected: PG error 23503
  UPDATE documents SET workspace_hash = 'orphan-attempt-xyz' WHERE id IN (SELECT id FROM documents LIMIT 1);
  -- Expected: PG error 23503
  ```

- [x] **G8** — Capture all 6 evidence outputs (curl outputs, log lines, SQL errors, MCP tool result errors) to `docs/evidence/fix-summary-workspace-registration-leaks/`. Reference in story.md.

> **Note:** Phase H (RRI-T Tier 3 regression with full LLM) is split into a SEPARATE follow-up task per Metis finding 4.1. It runs post-merge as a release-readiness gate, not as a PR gate. This keeps PR scope bounded.

## Phase I — Validate ladder

- [x] **I1** — `validate:quick`: `go build ./... && go test -race -short ./...` → green. Paste output to story Evidence.

- [x] **I2** — `test:integration`: `go test -race -tags=integration ./...` → green. Paste output.

- [x] **I3** — `smoke:e2e`: Build binary → start server on port 8899 → curl write/embed/reindex/summarize endpoints with both registered and unregistered hashes → verify expected outcomes (200 vs 400). Also test MCP memory_write via JSON-RPC.

- [x] **I4** — `self-review:staged-files`: Run `git status` before each commit; confirm no `.opencode/`, `node_modules/`, or unrelated files staged.

- [x] **I5** — `self-review:response-shape`: For each new handler error path (HTTP middleware + MCP tools), verify the JSON error envelope matches existing nano-brain conventions: HTTP=`{"error":"...","message":"..."}`, MCP=`mcp.NewToolResultError("<code>: <message>")`.

## Phase J — Review gate + PR

- [x] **J1** — Trigger `review-work` skill (5 parallel sub-agents). Reviewer ≠ implementer. Paste verdict in story Evidence.

- [x] **J2** — Address any review findings; rerun Phases I + H if implementation changed.

- [x] **J3** — Push branch: `git push -u origin feat/238-fix-summary-workspace-leaks`

- [x] **J4** — Open PR with title `fix(summary): close 6 workspace-registration leak points (#238)` and body containing `Closes #238`. Link RRI-T artifacts + evidence dir.

- [x] **J5** — Bot review loop: address feedback, push again, repeat. Max 3 cycles → escalate to human.

- [x] **J6** — On merge: verify issue #238 auto-closes via `Closes #238` keyword.

- [x] **J7** — `openspec archive fix-summary-workspace-registration-leaks` → updates main specs.

- [x] **J8** — Update `docs/TEST_MATRIX.md` with new test coverage from this change.

## Phase K — Release notes

- [x] **K1** — Add release note entry:
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
| A — Foundations (mostly no-op, query exists) | 0 | 0.1h |
| B — Persister | ~70 | 1h |
| C — Harvesters (+ Claude init extract) | ~120 | 2.5h |
| D — HTTP Middleware | ~125 | 2h |
| D' — MCP tool enforcement (NEW) | ~160 | 2h |
| E — Cleanup command | ~160 | 2.5h |
| F — Migration (+ UPDATE test, cascade test) | ~100 | 1.5h |
| G — User-flow tests (non-LLM) | — | 1.5h |
| I — Validate | — | 0.5h |
| J — Review + PR | — | 2-4h (depends on bot review) |
| K — Release notes | ~30 | 0.5h |
| **Total** | **~765 LoC** | **~16-18h ≈ 2-2.5 days** |

> Phase H (RRI-T Tier 3 regression) moved to separate follow-up task — runs post-merge.
