## 1. SQL query for chunk count

- [x] 1.1 Inspect existing `ListWorkspacesWithStats` query in `internal/storage/queries/workspaces.sql` and `internal/storage/sqlc/workspaces.sql.go`
- [x] 1.2 Extend query to include `chunk_count` via JOIN on chunks table grouped by workspace_hash, OR add separate `WorkspaceChunkCount(workspaceHash)` query
- [x] 1.3 Run `make sqlc-generate` (or equivalent) to regenerate Go bindings
- [x] 1.4 Verify regenerated `sqlc` types include the new `ChunkCount` field

## 2. Backend handler changes

- [x] 2.1 In `internal/server/handlers/workspace.go`, change `workspaceItem` JSON tags:
  - `WorkspaceHash string json:"workspace_hash"` → `json:"hash"`
  - `DocumentCount int64 json:"document_count"` → `json:"doc_count"`
- [x] 2.2 Add new field: `ChunkCount int64 json:"chunk_count"` to `workspaceItem` struct
- [x] 2.3 Populate `ChunkCount` from the SQL query result in `ListWorkspaces` handler
- [x] 2.4 Change return statement at workspace.go:206 from `c.JSON(http.StatusOK, items)` to `c.JSON(http.StatusOK, map[string]any{"workspaces": items})` OR add typed wrapper struct
- [x] 2.5 Update `WorkspaceQuerier` interface if signature of underlying query changed

## 3. CLI updates

- [x] 3.1 In `cmd/nano-brain/workspaces.go` `runWorkspacesListWithIO` (line 62), change parse target from `[]map[string]interface{}` to wrapper struct `{Workspaces []map[string]interface{}}`
- [x] 3.2 Update `renderWorkspacesTable` to read `hash` field instead of `workspace_hash`, `doc_count` instead of `document_count`
- [x] 3.3 In `cmd/nano-brain/cmd_workspace_remove.go` (line 116), apply same wrapper parsing for the listing path used by `--workspace-path` lookup

## 4. Tests

- [x] - [ ] 4.1 Add `TestListWorkspaces_ResponseShape` to `internal/server/handlers/workspace_test.go` — asserts wrapped object, exact field names per spec
- [x] - [ ] 4.2 Add `TestListWorkspaces_EmptyArray` — asserts `{workspaces: []}` for empty server
- [x] - [ ] 4.3 Add `TestListWorkspaces_ChunkCountPopulated` — creates workspace with chunks, asserts chunk_count matches
- [x] - [ ] 4.4 Update `TestRunWorkspacesList` (if exists) in `cmd/nano-brain/workspaces_test.go` to mock new wrapped response
- [ ] 4.5 Add `TestRunWorkspacesList_ParsesWrappedShape` — explicit test for new parser
- [ ] 4.6 Add `TestRunWorkspacesList_RejectsRawArray` — test that CLI returns clear error if server returns old raw-array shape (helps debug forward)

## 5. Verification

- [x] 5.1 Run `go build ./...` → exit 0
- [x] 5.2 Run `go vet ./...` → no warnings
- [x] 5.3 Run `go test -race -short ./internal/server/handlers/... ./cmd/nano-brain/...` → all PASS
- [x] 5.4 Run `curl http://localhost:3199/api/v1/workspaces | jq` and verify shape: `{"workspaces":[{"hash":"...","doc_count":N,"chunk_count":M,...}]}`
- [x] 5.5 Run `nano-brain workspaces list` against local dev server → table renders correctly
- [x] 5.6 Browser devtools test: load `http://localhost:3199/ui/`, open workspace selector, verify list populates with names and counts

## 6. PR + Review

- [ ] 6.1 Commit with message: `fix(api): wrap workspaces response and align field names with frontend (#277)`
- [ ] 6.2 Push branch `fix/277-workspaces-api-contract` to origin
- [ ] 6.3 Open PR with label `change-type:bug-fix`, `lane:normal`, `status:in-review`
- [ ] 6.4 Gemini review triage in `docs/evidence/fix-workspaces-api-contract/gemini-triage.md` per R31
- [ ] 6.5 Address findings (≤3 push cycles)
- [ ] 6.6 Squash merge with `--delete-branch` after CI green
- [ ] 6.7 Close issue #277

## 7. Archive + Release

- [ ] 7.1 Pull merged b-main
- [ ] 7.2 `openspec archive fix-workspaces-api-contract --yes`
- [ ] 7.3 Commit archive + push
- [ ] 7.4 Tag next `v2026.6.X` and push
- [ ] 7.5 Verify Release workflow + npm publish for `@nano-step/nano-brain`
- [ ] 7.6 Remove worktree + delete local branch
- [ ] 7.7 Comment release note on issue #277
