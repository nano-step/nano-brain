## Self-Review: feat/155-workspace-remove
Date: 2026-05-30
Reviewer: Sisyphus orchestrator

## Findings
| # | Severity | File | Description | Status |
|---|----------|------|-------------|--------|
| - | none | - | Mirrors existing ResetWorkspace destructive op pattern: explicit DeleteDocumentsByWorkspace before DeleteWorkspace (no FK cascade from docs → workspaces). | n/a |

## E2E smoke (PG @ host.docker.internal:5432, env var configured)
1. Created disposable workspace: `fe8b6d471fa2325f4d8f3efd90a6702497981af50f3404a836247f7b8e199bea`
2. `workspaces remove --workspace=$WS --dry-run` → "would be removed (0 document(s) deleted). No changes written."
3. `workspaces remove --workspace=$WS` (no --force) → "Refusing to remove ... Use --force to confirm deletion." (exit 2)
4. `workspaces remove --workspace=$WS --force` → "Workspace ... removed. 0 document(s) deleted." (200 OK)
5. `workspaces remove --workspace=$WS --force` again → "Error: workspace ... not found" (404)

All 4 user paths verified working with real PG. INFO logs include workspace + doc count + dry-run flag.

## Unit tests
- 4 handler tests in workspace_remove_test.go: Success, NotFound, CascadeStats, MissingHash
- 10 CLI tests in cmd_workspace_remove_test.go: parseWorkspaceRemoveFlags + runWorkspacesRemoveWithIO mock-server tests
- Full suite: `go test -race -short ./...` → all 20 packages OK

## Build
- `CGO_ENABLED=0 go build ./...` → exit 0

## Design notes
- REST: `DELETE /api/v1/workspaces/:hash` (consistent with `DELETE /api/v1/collections/:name`)
- CLI: `workspaces remove` subcommand (consistent with `workspaces list`); also accepts `rm` alias
- Safety: refuses without --force OR --dry-run; --dry-run fetches doc count via existing list endpoint
- Cascade: docs→chunks→embeddings via FK ON DELETE CASCADE; workspace→docs uses explicit DeleteDocumentsByWorkspace (no FK)

## Summary
- Critical: 0, Major: 0, Minor: 0
- E2E end-to-end destructive flow verified

## Gemini PR #222 Review — Findings Addressed (2026-05-30)

| # | Finding | Severity | Verdict | Fix |
|---|---------|----------|---------|-----|
| 1 | Sequential DeleteDocs + DeleteWorkspace not transactional → orphaned state on partial failure | Major | VALID | Wrapped in `db.BeginTx` + `tx.Commit`; rollback on either error. Falls back to non-tx path if `db == nil` (test injection). Server log includes `transactional: true/false` field. |
| 2 | CLI silently proceeds on JSON unmarshal failure (stats lost) | Major | VALID | Both `fetchDocCount` and `workspaceRemoveExecute` now print a warning + return exit code 1 |
| 3 | No validation when both `--workspace` and `--workspace-path` provided | Major | VALID | Added explicit `mutually exclusive` check after presence check; returns exit 2 |
| 4 | CHANGELOG entry duplicated across historical release sections | Medium | VALID | Restored CHANGELOG from clean `origin/b-main` baseline and re-applied a single entry under `[Unreleased] ### Features` |

## Note on ResetWorkspace
`internal/server/handlers/reset_workspace.go` has the same non-transactional pattern. Out of scope for #155 (separate handler, separate endpoint). Should be filed as follow-up issue.

## Re-verified E2E
- Conflict flags `--workspace=abc --workspace-path=/tmp/x` → "mutually exclusive" error, exit 2
- Full delete cycle with disposable workspace → server log shows `"transactional":true`
- All 4 handler tests + 10 CLI tests still pass
