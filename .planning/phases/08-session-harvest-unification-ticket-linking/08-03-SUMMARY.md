---
phase: "08"
plan: "03"
subsystem: server
tags: [cross-workspace, ticket-query, mcp-tool, sqlc, http-endpoint]
dependency_graph:
  requires: [08-01-sessions-collection-unification, 08-02-ticket-tags-on-documents]
  provides: [ListDocumentsByTag-query, TicketHandler-endpoint, memory_ticket-mcp-tool]
  affects: [internal/storage/queries, internal/storage/sqlc, internal/server/handlers, internal/server/routes, internal/mcp]
tech_stack:
  added: []
  patterns: [cross-workspace-query, sqlc-positional-param, echo-handler-interface, mcp-tool-registration]
key_files:
  created:
    - internal/storage/queries/documents.sql (ListDocumentsByTag query added)
    - internal/storage/sqlc/documents.sql.go (ListDocumentsByTag generated)
    - internal/server/handlers/ticket.go
    - internal/server/handlers/ticket_test.go
  modified:
    - internal/server/routes.go
    - internal/mcp/tools.go
    - internal/mcp/tools_test.go
    - internal/mcp/concurrent_test.go
decisions:
  - "SQL query uses positional $1 (not named param) so sqlc generates Column1 field — callers use Column1 for the tag value"
  - "sourceFromPath takes priority over sourceFromTags in TicketHandler; source_path scheme is more reliable than tag presence"
  - "Route placed on api group (no workspace middleware) to allow cross-workspace queries — consistent with design intent"
  - "MCP tool derives source inline (no import of handlers package) to avoid circular dependency"
  - "Tool count tests updated from 17 to 18 — normal maintenance when adding a new MCP tool"
metrics:
  duration: "~13 minutes"
  completed: "2026-06-29"
  tasks_completed: 3
  tasks_total: 3
  files_created: 4
  files_modified: 4
status: complete
---

# Phase 8 Plan 03: Cross-Workspace Ticket Query Summary

**One-liner:** Added `ListDocumentsByTag` cross-workspace SQL query, `TicketHandler` HTTP endpoint (`GET /api/v1/sessions/by-ticket?ticket=<ID>`), and `memory_ticket` MCP tool returning all sessions tagged `ticket:<ID>` across all workspaces.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | ListDocumentsByTag sqlc query + TicketHandler + tests | 0adb587 | documents.sql, documents.sql.go, ticket.go, ticket_test.go |
| 2 | Register HTTP route + MCP tool memory_ticket | 66765db | routes.go, tools.go, tools_test.go, concurrent_test.go |
| 3 | Unit tests (covered in Task 1) | 0adb587 | ticket_test.go |

## What Was Built

### Task 1 — SQL query + handler + tests

**SQL query `ListDocumentsByTag`** (`internal/storage/queries/documents.sql`):
```sql
SELECT id, workspace_hash, title, content, source_path, collection, tags, created_at, updated_at
FROM documents
WHERE $1::text = ANY(tags)
  AND collection = $2
ORDER BY updated_at DESC
LIMIT $3;
```
No `workspace_hash` filter — intentionally cross-workspace. `sqlc generate` produces `ListDocumentsByTagParams{Column1, Collection, Limit}` and `ListDocumentsByTagRow`.

**`TicketHandler`** (`internal/server/handlers/ticket.go`):
- `TicketQuerier` interface wrapping `ListDocumentsByTag`
- `TicketSessionResult` struct: `session_id`, `title`, `source`, `workspace_hash`, `source_path`, `tags`, `snippet` (first 300 runes)
- Source derived from `source_path` scheme first (`summary://claude/` → "claude", `summary://opencode/` → "opencode", etc.), falls back to `sourceFromTags`
- Returns 400 for missing ticket param, 500 on DB error, 200 JSON array (empty array for unknown ticket)

**Tests** (`internal/server/handlers/ticket_test.go`) — 7 cases:
- `TestTicketHandler_CrossWorkspace`: 2 docs from different workspace hashes → both returned; asserts tag and collection sent to querier
- `TestTicketHandler_EmptyTicket`: missing param → 400
- `TestTicketHandler_QuerierError`: DB error → 500
- `TestTicketHandler_HashStyleTicket`: `ticket=#42` (URL-encoded) → tag `ticket:#42`
- `TestTicketHandler_LimitEnforced`: querier called with `Limit=50`
- `TestTicketHandler_SourceFromPath`: no source tag → source derived from source_path scheme
- `TestTicketHandler_UnknownTicket`: empty result set → 200 empty array

### Task 2 — Route + MCP tool

**HTTP route** (`internal/server/routes.go`):
```
GET /api/v1/sessions/by-ticket?ticket=<ID>
```
Placed on the `api` group (no workspace middleware), making it accessible without workspace context.

**MCP tool `memory_ticket`** (`internal/mcp/tools.go`):
- Input: `{ "ticket": "DEV-4706" }` (required)
- Calls `a.queries.ListDocumentsByTag` directly (no HTTP round-trip)
- Returns formatted markdown:
  ```
  ## Sessions for ticket DEV-4706

  - **<title>** (`<source>`, workspace `<ws[:8]>`)
    <snippet>
  ```
- Returns `"No sessions found for ticket DEV-4706."` for empty results (not an error)
- Tool count tests updated 17→18; `memory_ticket` added to expected names list

## Verification

```
CGO_ENABLED=0 go build ./...  → PASS
go test -race -short ./...    → all packages PASS (29 packages, 0 FAIL)
```

Key packages:
- `internal/server/handlers` — 7 new ticket tests PASS, 170+ total tests PASS
- `internal/mcp` — tool count 18 PASS, concurrent race detector PASS
- `internal/server` — routes compile and pass integration tests

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `sourceFromTags` redeclared in handlers package**
- **Found during:** Task 1 build
- **Issue:** `sourceFromTags` already exists in `internal/server/handlers/summarize.go`. Adding a second definition caused a compile error.
- **Fix:** Removed duplicate; used the existing shared `sourceFromTags` (same package). Adjusted source detection to use `sourceFromPath` as primary (more precise for ticket handler context) with `sourceFromTags` as fallback.
- **Files modified:** `internal/server/handlers/ticket.go`
- **Commit:** 0adb587

**2. [Rule 1 - Bug] Tool count tests expected 17 tools, not 18**
- **Found during:** Task 2 test run
- **Issue:** `TestRegisterTools_CountAndNames` and `TestToolRegistration_ListToolsUnderRaceDetector` assert exact tool count = 17. Adding `memory_ticket` bumped it to 18 and caused test failures.
- **Fix:** Updated both test files to expect 18 tools; added `memory_ticket` to the expected names list.
- **Files modified:** `internal/mcp/tools_test.go`, `internal/mcp/concurrent_test.go`
- **Commit:** 66765db

**3. [Simplify] Removed `normalizeTicketID` dead wrapper**
- **Found during:** /simplify review pass
- **Issue:** `normalizeTicketID` only called `strings.TrimSpace` on an already-trimmed input and returned it unchanged — a one-liner that added no value.
- **Fix:** Removed the function; used `ticketParam` (already trimmed) directly.
- **Files modified:** `internal/server/handlers/ticket.go`
- **Commit:** 0adb587

## Known Stubs

None — all data paths are wired. Results are empty if no harvest cycle has run since 08-02 landed (ticket tags are applied during harvest, not retroactively).

## Threat Flags

None — no new auth surface introduced. The `GET /api/v1/sessions/by-ticket` endpoint is on the `api` group which already has global auth middleware applied at the server level. `memory_ticket` is an MCP tool accessible only to processes with a valid MCP connection (server-local).

## Self-Check: PASSED

Files created:
- `internal/server/handlers/ticket.go` — FOUND
- `internal/server/handlers/ticket_test.go` — FOUND

Commits verified in git log:
- `0adb587` — FOUND (Task 1)
- `66765db` — FOUND (Task 2)

Build: `CGO_ENABLED=0 go build ./...` — clean.
Tests: `go test -race -short ./...` — all packages PASS, 0 FAIL.
