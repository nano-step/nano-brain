## Why

Web UI workspace selector cannot populate workspaces. Live test on production v2026.6.3 confirms:

- `GET /api/v1/workspaces` returns raw array `[{workspace_hash, root_path, name, document_count, ...}]`
- Frontend hook `useWorkspaces` expects `{workspaces: [{hash, name, doc_count, chunk_count, ...}]}`
- Mismatches: response wrapping (array vs object), field name (`workspace_hash` → `hash`, `document_count` → `doc_count`), missing field (`chunk_count`)

This is the latest manifestation of a **recurring API contract drift pattern**:
- `fc85571` today: "add json tags so /api/v1/config returns snake_case keys for frontend"
- `f4c8678`: "oracle+gemini review fixes for story 9.6"
- `b839aef`: "wire /settings to SettingsPanel"

Root cause: NO schema-as-source-of-truth. Go structs and TS interfaces evolve independently. Fixes are point patches that don't prevent the next drift.

This change does TWO things:
1. **Fix #277 immediately**: align backend `/api/v1/workspaces` to frontend contract.
2. **Add regression guard**: integration test that parses real response against frontend-expected shape, so future backend changes that drift from FE fail CI.

## What Changes

- **Backend** (`internal/server/handlers/workspace.go`):
  - `workspaceItem` JSON tags: `workspace_hash` → `hash`, `document_count` → `doc_count`
  - Add `ChunkCount int64 json:"chunk_count"` field, populated from new query `WorkspaceChunkCount`
  - Wrap response: `{"workspaces": [...]}` instead of raw array
  - Keep `last_document_updated`, `created_at`, `updated_at`, `root_path`, `name` as-is

- **CLI** (`cmd/nano-brain/workspaces.go`):
  - Update `runWorkspacesListWithIO` to parse `{workspaces: [...]}` wrapper
  - Update `cmd_workspace_remove.go` listing path to use same wrapper

- **SQL** (`internal/storage/queries/workspaces.sql`):
  - Extend `ListWorkspacesWithStats` to JOIN chunks count, OR add separate `WorkspaceChunkCount`

- **Frontend** (`web/src/api/types.ts`, `web/src/hooks/useWorkspaces.ts`):
  - No changes needed — frontend already expects target shape

- **Tests**:
  - New `TestListWorkspaces_ResponseShape` in `internal/server/handlers/workspace_test.go`: asserts response is wrapped object with `workspaces` key, each item has all expected fields with correct JSON tags
  - New `TestRunWorkspacesList_ParsesWrappedShape` in `cmd/nano-brain/workspaces_test.go`: CLI handles new shape

- **Documentation**:
  - Update `README.md` if it documents `/api/v1/workspaces` shape

## Capabilities

### New Capabilities
- `workspaces-api-contract`: Defines the canonical shape of `GET /api/v1/workspaces` response. Both backend implementations and frontend consumers MUST match this shape. Adds regression test that catches future drift.

### Modified Capabilities
None — this is a defect fix; no new user-facing features.

## Impact

- **Code**: ~6 files touched (handler + CLI + SQL + test + frontend already aligned).
- **Behavior**: Frontend UI workspace selector now populates correctly. CLI `workspaces list` continues to work.
- **Risk**: Medium — breaking change for any external API consumers using raw array shape. Mitigated by CLI being only known consumer + this is internal-facing localhost server.
- **Database**: No schema migration. New query joins existing tables.
- **Performance**: One extra query per workspace listed (chunk count). Negligible for typical workspace counts (< 100).

## Out of Scope

Phase 2 long-term solution (schema-driven contract via OpenAPI codegen or trpcgo) is tracked in a separate follow-up change. This change only fixes the immediate drift on `/api/v1/workspaces`.

E2E UI test infrastructure (browser-driven testing with port 3199) is added in a separate change `harness-e2e-ui-gate` per user request.
