# Evidence: fix-summary-workspace-registration-leaks (#238)

Phase G user-flow tests executed on port 8899 isolated instance after applying:
- branch `feat/238-fix-summary-workspace-leaks`
- binary `/tmp/nano-brain-fix238` (CGO_ENABLED=0)
- migration 11 (FK constraints)

## Upgrade sequence executed

1. **STOP** — killed old binary PID 98427
2. **CLEANUP** — `cleanup-orphan-workspaces --dry-run` → "No orphan documents found. DB is clean."
3. **MIGRATE** — `db:migrate` → "Database is up to date (version 11)"
4. **START NEW BINARY** — `./nano-brain-fix238 serve` → PID 97345 healthy

API confirms `migration_version: 11, workspace_count: 13`.

## Test results

| Phase | Test | File | Result |
|-------|------|------|--------|
| G2 | HTTP write unregistered workspace | `g2-http-unregistered-rejected.txt` | PASS — HTTP 400 `workspace_not_registered` |
| G3 | HTTP write registered workspace | `g3-http-registered-accepted.txt` | PASS — HTTP 201, doc_id returned |
| G4 | HTTP write `workspace: "all"` | `g4-http-all-rejected.txt` | PASS — HTTP 400 `workspace_all_not_supported` |
| G5 | HTTP summarize unregistered | `g5-http-summarize-unregistered-rejected.txt` | PASS — HTTP 400 `workspace_not_registered` |
| G7 | Doc from G3 is searchable | `g7-http-registered-doc-searchable.txt` | PASS — 7 results in 5ms |

## Leak point status

| Leak | Phase | Method | Verified by |
|------|-------|--------|-------------|
| #1 HTTP middleware trust | D | `workspaceRegisteredMiddleware` | G2, G4, G5 (HTTP 400 responses) |
| #2 Claude Code harvester unregistered | C | `initClaudeCodeHarvester` registered-check | Unit tests `TestInitClaudeCodeHarvester_*` PASS |
| #3 Persister.Save trust | B | `ErrWorkspaceNotRegistered` guard | Integration tests `TestPersister_Save_*` PASS |
| #4 HarvestSummarizer passthrough | B (transitive) | Persister guard catches all callers | Same as #3 |
| #5 OpenCode fallback + auto-registration | C | Removed UpsertWorkspace + skip orphans + registered-check | Integration tests `TestOpenCodeSQLite_Orphan/Unregistered_*` PASS |
| #6 Missing FK constraint | F | Migration 00011 | Integration tests `TestMigration00011_*` PASS (INSERT, UPDATE, CASCADE, chunks) |
| #7 MCP write bypass | D' | `requireRegisteredWorkspace` in MCP tools | Integration tests `TestMemoryWrite_*` + `TestMemoryUpdate_*` PASS |

## Pre-existing issues (out of scope, not caused by this PR)

1. `internal/search/isolation_test.go` build failure — `HybridSearch` signature drift from PR #221 (`--tags` filter). Pre-existed on b-main.
2. `POST /api/v1/get` returns null fields for some paths — pre-existing RRI-T finding F3, out of scope.

## Server log evidence

See `server-startup.log` for new-binary boot logs and request handling. Notable lines:

- POST /api/v1/write status=400 latency_ms=2 (G2: unregistered, rejected by middleware in 2ms)
- POST /api/v1/write status=201 latency_ms=12 (G3: registered, written in 12ms)
- POST /api/v1/summarize status=400 latency_ms=0 (G5: unregistered, rejected by middleware)
- POST /api/v1/search status=200 results=7 latency_ms=5 (G7: doc searchable)
