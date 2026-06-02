## 1. SQL query

- [ ] 1.1 Inspect `internal/storage/queries/workspaces.sql` for existing `GetWorkspaceByHash` (or equivalent single-row lookup by hash PK)
- [ ] 1.2 If missing, add query: `-- name: GetWorkspaceByHash :one\nSELECT * FROM workspaces WHERE hash = $1;`
- [ ] 1.3 Run `make sqlc-generate` (or `sqlc generate`) to regenerate Go bindings
- [ ] 1.4 Verify regenerated `sqlc.Queries` includes `GetWorkspaceByHash` method

## 2. HTTP handler — POST /api/v1/workspaces/resolve

- [ ] 2.1 Create `internal/server/handlers/workspace_resolve.go`:
  - Define `workspaceResolveRequest{Path string json:"path"}`
  - Define `workspaceResolveResponse{WorkspaceHash, RootPath, Name string; Registered bool}` with snake_case JSON tags
  - Define `WorkspaceResolver` interface with method `GetWorkspaceByHash(ctx, hash) (sqlc.Workspace, error)`
  - Implement `ResolveWorkspace(q WorkspaceResolver, logger zerolog.Logger) echo.HandlerFunc` constructor
  - Handler logic:
    1. Bind request body → 400 if invalid
    2. Validate `path != ""` → 400 `path is required`
    3. `absPath, _ := filepath.Abs(req.Path)` → 400 if error
    4. `hash, _ := storage.WorkspaceHash(absPath)` → 400 if error
    5. Look up `q.GetWorkspaceByHash(ctx, hash)`:
       - If found: response uses DB `Name`, `registered: true`
       - If `sql.ErrNoRows`: response uses `filepath.Base(absPath)`, `registered: false`
       - Other error: 500 + log
    6. Return `c.JSON(200, response)`
- [ ] 2.2 Register route in `internal/server/routes.go` next to `/init` (public group, NOT through `workspaceMiddleware`):
  ```go
  api.POST("/workspaces/resolve", handlers.ResolveWorkspace(s.queries, s.logger))
  ```
- [ ] 2.3 Extend `WorkspaceQuerier` interface in `workspace.go` to include `GetWorkspaceByHash` (or use a separate interface for resolve — prefer keeping minimal surface per handler file)

## 3. HTTP handler tests

- [ ] 3.1 Create `internal/server/handlers/workspace_resolve_test.go` with table-driven tests:
  - `TestResolveWorkspace_Registered` — mock returns row → assert `registered:true`, fields populated from row
  - `TestResolveWorkspace_NotRegistered` — mock returns `sql.ErrNoRows` → assert `registered:false`, `name` from filepath.Base
  - `TestResolveWorkspace_MissingPath` — empty body → 400
  - `TestResolveWorkspace_EmptyPathField` — `{"path":""}` → 400
  - `TestResolveWorkspace_RelativePath` — `{"path":"."}` → normalized to abs
  - `TestResolveWorkspace_DBError` — mock returns other error → 500
- [ ] 3.2 Use inline mock struct implementing `WorkspaceResolver`, no gomock
- [ ] 3.3 Create `internal/server/handlers/workspace_resolve_integration_test.go` with `//go:build integration` and use `testutil.SetupTestDB`:
  - Register a workspace via real `InitWorkspace` handler
  - Call resolve with its path → assert `registered:true`
  - Call resolve with a DIFFERENT path → assert `registered:false`, hash is deterministic

## 4. CLI subcommand — `workspaces current`

- [ ] 4.1 In `cmd/nano-brain/workspaces.go` extend `runWorkspacesCmd` switch:
  ```go
  case "current":
      runWorkspacesCurrent(args[1:])
  ```
- [ ] 4.2 Implement `runWorkspacesCurrent(args []string)` and `runWorkspacesCurrentWithIO(args, stdout, stderr) int`:
  - Parse flags: `--path=<p>`, `--export`, `--json`, `--check`
  - Determine path: `--path` value or `os.Getwd()`
  - Call `POST /api/v1/workspaces/resolve` via existing `doRequest`
  - Parse response into a struct
  - Format output based on flags:
    - `--json`: print body
    - `--export`: print `export NANO_BRAIN_WORKSPACE=<hash>\n`
    - default: print `<hash>\n`
  - Exit code: 0 success; 1 HTTP/server error; 2 `--check` && `!registered`
- [ ] 4.3 Update `workspacesUsage()` to include `current` in usage line

## 5. CLI tests

- [ ] 5.1 In `cmd/nano-brain/workspaces_test.go` add tests using `httptest.NewServer` (same pattern as `runWorkspacesListWithIO` tests):
  - `TestRunWorkspacesCurrent_Default_PrintsHash`
  - `TestRunWorkspacesCurrent_Export_PrintsExportLine`
  - `TestRunWorkspacesCurrent_JSON_PrintsFullBody`
  - `TestRunWorkspacesCurrent_Check_RegisteredExitsZero`
  - `TestRunWorkspacesCurrent_Check_UnregisteredExitsTwo`
  - `TestRunWorkspacesCurrent_PathOverridesCWD`
  - `TestRunWorkspacesCurrent_ServerUnreachable_ExitsOne`
- [ ] 5.2 Use `os.Setenv("NANO_BRAIN_HOST", ...)` + `NANO_BRAIN_PORT` to point `getBaseURL()` at the test server

## 6. MCP tool — memory_workspaces_resolve

- [ ] 6.1 In `internal/mcp/tools.go` register new tool in the same pattern as existing memory_* tools
- [ ] 6.2 Extract the handler core logic into an unexported function `resolveWorkspaceCore(ctx, q, path) (response, error)` reusable by HTTP and MCP
- [ ] 6.3 MCP tool wraps core: validate args["path"] is non-empty string → call core → marshal response
- [ ] 6.4 In `internal/mcp/tools_test.go` add `TestMemoryWorkspacesResolve_*` tests for registered, not_registered, empty_path

## 7. Documentation — SKILL.md (phase-based rewrite)

- [ ] 7.1 Rewrite `skills/nano-brain/SKILL.md` with frontmatter + 4 phases:
  - Phase 1 DISCOVER — server health check, bootstrap one-liner, registration check
  - Phase 2 SELECT — user intent → operation table
  - Phase 3 EXECUTE — sub-sections for query/search/vsearch/write/wake-up/graph/symbols/tags, each with request, response, error table
  - Phase 4 RECOVER — error catalog + retry policy
  - References section pointing to `references/*.md`
- [ ] 7.2 Keep total length ≤500 lines (Anthropic guideline)
- [ ] 7.3 Verify all curl examples use `/api/v1/*` paths and include `workspace` field or `$NANO_BRAIN_WORKSPACE`
- [ ] 7.4 Verify all CLI examples use `nano-brain` command names that actually exist (cross-check against `cmd/nano-brain/main.go` switch)

## 8. Documentation — AGENTS_SNIPPET.md (rewrite)

- [ ] 8.1 Rewrite `skills/nano-brain/AGENTS_SNIPPET.md` to ~40 lines:
  - Bootstrap one-liner: `eval "$(npx nano-brain workspaces current --export)"`
  - Quick reference table (CLI commands)
  - HTTP API section showing 1 example with workspace field + correct path
  - Pointer to `skills/nano-brain/SKILL.md` for full reference
- [ ] 8.2 Preserve `<!-- OPENCODE-MEMORY:START -->` and `<!-- OPENCODE-MEMORY:END -->` markers

## 9. Documentation — project AGENTS.md sync

- [ ] 9.1 Replace `<!-- OPENCODE-MEMORY:START -->` block in `/Users/tamlh/workspaces/self/AI/Tools/nano-brain/AGENTS.md` with content from new `AGENTS_SNIPPET.md`
- [ ] 9.2 Verify no other places in `AGENTS.md` reference the wrong `/api/query` path

## 10. Documentation — skill copies sync

- [ ] 10.1 Sync `.opencode/skills/nano-brain/SKILL.md` from canonical
- [ ] 10.2 Sync `.opencode/skills/nano-brain/references/*.md` from canonical
- [ ] 10.3 Sync `~/.config/opencode/skills/nano-brain/SKILL.md` from canonical
- [ ] 10.4 Sync `~/.config/opencode/skills/nano-brain/references/*.md` from canonical
- [ ] 10.5 Verify all 3 copies (`skills/`, `.opencode/skills/`, `~/.config/`) are byte-identical for SKILL.md + references

## 11. Documentation — README.md update

- [ ] 11.1 Add row to "Public Endpoints" table: `POST /api/v1/workspaces/resolve | Resolve path → workspace hash + registration status`
- [ ] 11.2 Add row to "CLI Commands" table: `nano-brain workspaces current [--path=<p>] [--export] [--json] [--check] | Resolve current/path workspace hash`
- [ ] 11.3 Add row to "MCP Tools" table: `memory_workspaces_resolve | Resolve path → workspace hash + registered`

## 12. Verification — validate:quick

- [ ] 12.1 `cd .opencode/worktrees/feat-316-workspace-resolve && go build ./...` → exit 0
- [ ] 12.2 `go vet ./...` → no warnings
- [ ] 12.3 `go test -race -short ./internal/server/handlers/... ./cmd/nano-brain/... ./internal/mcp/...` → all PASS
- [ ] 12.4 `lsp_diagnostics` on all changed Go files → no errors/warnings

## 13. Verification — test:integration

- [ ] 13.1 `go test -race -tags=integration ./internal/server/handlers/...` → PASS
- [ ] 13.2 Verify the integration test creates + queries a real workspace via real PG

## 14. Verification — smoke:e2e

- [ ] 14.1 Build local binary: `CGO_ENABLED=0 go build -o /tmp/nb-316 ./cmd/nano-brain`
- [ ] 14.2 Start server on port 3199 (avoid clash with running 3100): `NANO_BRAIN_SERVER_PORT=3199 /tmp/nb-316 &`
- [ ] 14.3 Curl resolve with current path → assert response shape + `registered:true`
- [ ] 14.4 Curl resolve with a non-registered path → assert `registered:false`
- [ ] 14.5 Run `/tmp/nb-316 workspaces current --export` → eval the output → assert `$NANO_BRAIN_WORKSPACE` is set
- [ ] 14.6 Use the exported env to call `query` via curl with manual `workspace` field — assert search works
- [ ] 14.7 Run `/tmp/nb-316 workspaces current --check` in a non-registered dir → assert exit code 2
- [ ] 14.8 Stop server cleanly

## 15. Verification — self-review

- [ ] 15.1 `self-review:response-shape` — read `workspaceResolveResponse` struct + handler mapping → confirm all 4 fields explicitly assigned in both branches (registered/unregistered)
- [ ] 15.2 `self-review:staged-files` — `git status` → no `.opencode/` files staged, no `package-lock.json`, no worktree metadata
- [ ] 15.3 Verify no AI-slop: no fabricated paths, no comments like "// implement X here", no unused error variables ignored with `_`

## 16. PR + Review Gate

- [ ] 16.1 Stage only intended files, write conventional commit:
  `feat(workspace-resolve): add POST /resolve + CLI 'workspaces current' + MCP tool + phase-based skill rewrite (#316)`
- [ ] 16.2 Push branch `feat/316-workspace-resolve` to origin
- [ ] 16.3 Open PR via `gh pr create` with body referencing #316 and the OpenSpec change folder
- [ ] 16.4 STOP — request human/external review per harness rule "no self-review"
- [ ] 16.5 Address bot review (Gemini) findings, ≤3 push cycles
- [ ] 16.6 After Review Verdict = PASS, squash merge + delete branch

## 17. Archive + Release

- [ ] 17.1 Pull merged master
- [ ] 17.2 `openspec archive add-workspace-resolve-endpoint --yes`
- [ ] 17.3 Commit archive + push (master push triggers auto-tag → release → npm publish)
- [ ] 17.4 Remove worktree: `git worktree remove .opencode/worktrees/feat-316-workspace-resolve`
- [ ] 17.5 Delete local branch: `git branch -D feat/316-workspace-resolve`
- [ ] 17.6 Comment release note on issue #316 + close

## 18. Memory persistence

- [ ] 18.1 Write session summary to nano-brain memory with tags=[decision, skill-rewrite, workspace-resolve] documenting the design choices (D1-D10) for future sessions
