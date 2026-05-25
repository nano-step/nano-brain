# Tasks: Workspaces List CLI + Status Count Fix

## Phase 1 — Server-side count fix (foundation)

- [x] **1.1** Add SQL query in `internal/storage/queries/workspaces.sql`:
  ```sql
  -- name: CountWorkspaces :one
  SELECT COUNT(*) FROM workspaces;
  ```
- [x] **1.2** Regenerate sqlc: `sqlc generate` (or commit hand-written change matching sqlc output if sqlc binary not installed in the dev env — confirm by inspecting existing `workspaces.sql.go` style).
- [x] **1.3** Add `WorkspaceCounter` interface to `internal/server/handlers/health.go` with single method `CountWorkspaces(ctx context.Context) (int64, error)`.
- [x] **1.4** Add `counter WorkspaceCounter` field to `Health` struct; update `NewHealth` signature; update call site in `internal/server/routes.go` (pass `s.queries` or equivalent).
- [x] **1.5** Replace hardcoded `0` in both `Health.Health()` (line 85) and `Health.Status()` (line 110) with `h.counter.CountWorkspaces(ctx)`. Soft-fail on error: log warning, use `0`.
- [x] **1.6** Update `internal/server/handlers/health_test.go`:
  - Add a mock implementing `WorkspaceCounter`.
  - Add tests: `TestStatusReturnsRealWorkspaceCount`, `TestHealthReturnsRealWorkspaceCount`, `TestStatusSoftFailsOnCountError`, `TestHealthSoftFailsOnCountError`.
  - Ensure existing tests still pass (update constructors to pass the new mock).

## Phase 2 — CLI workspaces command

- [x] **2.1** Create `cmd/nano-brain/workspaces.go`:
  - `runWorkspacesCmd(args []string)` — dispatcher mirroring `runCollectionCmd`. Treats empty args or arg starting with `--` as `list`.
  - `runWorkspacesList(args []string)` — parses `--json`, calls `doRequest("GET", getBaseURL()+"/api/v1/workspaces", nil)`, renders table or passes through JSON.
  - Use `text/tabwriter` for table render.
  - Path truncation helper: `truncateLeft(s string, max int) string` — if `len(s) <= max`, return `s`; else return `".." + s[len(s)-(max-2):]`.
- [x] **2.2** Wire into `cmd/nano-brain/main.go`: add `case "workspaces": runWorkspacesCmd(args[1:])`. Add help entry alphabetized with siblings.
- [x] **2.3** Add `cmd/nano-brain/workspaces_test.go` with 10 tests from design.md test plan. Use `httptest.NewServer` + `t.Setenv("NANO_BRAIN_HOST", ...)` and capture stdout/stderr via os.Pipe or by accepting `io.Writer` params (refactor `runWorkspacesList` to take writer params if needed for testability).

## Phase 3 — Validation ladder

- [x] **3.1** `CGO_ENABLED=0 go build ./...` → success
- [x] **3.2** `go vet ./...` → clean
- [x] **3.3** `go test -race -short ./cmd/nano-brain/... ./internal/server/handlers/... ./internal/storage/...` → all pass
- [x] **3.4** `go test -race -short ./...` → all 14+ packages pass

## Phase 4 — Smoke evidence

- [x] **4.1** Write `docs/evidence/workspaces-list-cli.md` with manual smoke transcript (if a live server is available in the dev environment). If not, document the limitation and note that unit tests cover the behavior.

## Phase 5 — Mark tasks complete in OpenSpec

- [x] **5.1** Mark all `[ ]` checkboxes complete in this file using `- [x]`.

## Phase 6 — PR

- [ ] **6.1** (Orchestrator only — not implementation agent) Push branch, open PR linking issue #142.
