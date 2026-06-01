## Context

The `/api/v1/workspaces` endpoint is consumed by two clients:

1. **Web UI** (`web/src/hooks/useWorkspaces.ts`) — fetches via `apiGetJSON<WorkspacesResponse>('/api/v1/workspaces')` and renders workspace selector in `CommandPalette` + dashboard.
2. **CLI** (`cmd/nano-brain/workspaces.go` line 49, `cmd/nano-brain/cmd_workspace_remove.go` line 116) — fetches via `doRequest("GET", ...)`, parses into `[]map[string]interface{}`.

Backend `ListWorkspaces` (handler at `internal/server/handlers/workspace.go:182`) currently returns a raw array of `workspaceItem`:

```go
type workspaceItem struct {
    WorkspaceHash       string     `json:"workspace_hash"`
    RootPath            string     `json:"root_path"`
    Name                string     `json:"name"`
    DocumentCount       int64      `json:"document_count"`
    LastDocumentUpdated *time.Time `json:"last_document_updated,omitempty"`
    CreatedAt           time.Time  `json:"created_at"`
    UpdatedAt           time.Time  `json:"updated_at"`
}
// returned via: c.JSON(http.StatusOK, items) where items is []workspaceItem
```

Frontend expects (from `web/src/api/types.ts:1-12`):

```ts
interface Workspace {
  hash: string
  name: string
  root_path: string
  doc_count: number
  chunk_count: number
  created_at: string
}
interface WorkspacesResponse {
  workspaces: Workspace[]
}
```

Three mismatches: wrapping (array vs `{workspaces: [...]}`), field rename (`workspace_hash` → `hash`, `document_count` → `doc_count`), missing field (`chunk_count`).

## Goals / Non-Goals

**Goals:**
- Make web UI workspace selector functional immediately by aligning backend to frontend shape.
- Preserve CLI functionality (update CLI parser to handle new wrapped shape).
- Add regression test that fails CI if response shape diverges from frontend expectation.
- Keep change scope minimal — just `/api/v1/workspaces` endpoint.

**Non-Goals:**
- Schema-driven codegen (OpenAPI, trpcgo) — too broad for this change, deferred to a separate proposal.
- Fixing other endpoints with mismatches (`/api/v1/stats`, `/api/v1/wakeup`, etc.) — each will be its own change.
- Breaking change versioning (`/api/v2/workspaces`) — `/api/v1/` is internal-only at localhost, no external clients to deprecate.

## Decisions

### D1: Adopt frontend's field naming convention (`hash`, `doc_count`)

**Decision:** Rename backend JSON tags to match frontend: `workspace_hash` → `hash`, `document_count` → `doc_count`.

**Rationale:**
- Frontend is the primary consumer (interactive UI), CLI is secondary (admin tooling).
- Backend Go field NAMES stay the same (`WorkspaceHash`, `DocumentCount`) — only JSON tags change. Type safety preserved.
- Frontend matches `Workspace.hash` naming convention used throughout web/src (e.g., `localStorage.setItem('nano-brain.workspace', hash)`).
- "doc_count" is shorter and matches frontend's `doc_count` convention used in `Collection {name, doc_count}`.

**Alternative considered:** Update frontend to use `workspace_hash` and `document_count`. Rejected — would touch ~10 frontend files, broader blast radius.

### D2: Wrap response in `{workspaces: [...]}` object

**Decision:** Return `{"workspaces": []workspaceItem}` instead of raw `[]workspaceItem`.

**Rationale:**
- Standard REST convention: list endpoints return wrapped object for forward extensibility (e.g., adding `total`, `pagination`, `nextCursor` later without breaking clients).
- Frontend already expects wrapper.
- Better discoverability in OpenAPI docs if generated later.

**Alternative considered:** Keep raw array, update frontend to unwrap. Rejected — wrapping is industry standard and easier to extend.

### D3: Add `chunk_count` field via SQL JOIN

**Decision:** Extend the `ListWorkspacesWithStats` query (or add new aggregation) to count chunks per workspace, expose as `ChunkCount int64 json:"chunk_count"`.

**Rationale:**
- Frontend uses chunk_count for "X docs · Y chunks" display in workspace selector.
- One query JOIN is more efficient than N+1 (one query per workspace).

**Alternative considered:** Make chunk_count optional (omitempty), populate lazily on demand. Rejected — frontend always renders it, lazy adds complexity.

### D4: CLI parser update

**Decision:** Update CLI to parse `{workspaces: [...]}` wrapper. Adjust `runWorkspacesListWithIO` (workspaces.go:62) and the listing path in `cmd_workspace_remove.go:116`.

```go
// Before:
var items []map[string]interface{}
if err := json.Unmarshal(body, &items); err != nil { ... }

// After:
var resp struct {
    Workspaces []map[string]interface{} `json:"workspaces"`
}
if err := json.Unmarshal(body, &resp); err != nil { ... }
items := resp.Workspaces
```

**Rationale:** Single source of truth. CLI is in-repo, we can update it atomically with backend.

### D5: Regression test strategy

**Decision:** Add integration test in `internal/server/handlers/workspace_test.go`:

```go
func TestListWorkspaces_ResponseShape(t *testing.T) {
    // Setup test workspace with chunks
    // Call ListWorkspaces handler
    // Assert response body has exactly these keys at root: ["workspaces"]
    // Assert each workspace item has keys: ["hash", "name", "root_path", "doc_count",
    //                                       "chunk_count", "created_at", "updated_at",
    //                                       "last_document_updated" (optional)]
    // Assert types: hash string non-empty, doc_count int64, chunk_count int64
}
```

This test pins the contract. Any future field rename or wrapping change fails CI immediately.

**Alternative considered:** Generate TS types from Go via codegen. Deferred — too broad for this change.

### D6: Live E2E verification before push

**Decision:** Per user request, build local binary, start on port 3199, use chrome-devtools MCP to load `/ui/` and verify workspace selector populates. Push only after browser test passes.

**Rationale:** Catches the same class of bugs the change is fixing. Establishes precedent for E2E UI gate (formalized in separate `harness-e2e-ui-gate` change).

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| Breaking change for external API consumers | `/api/v1/` is localhost-only, no known external consumers. Document in commit + changelog. |
| CLI breaks if parser not updated atomically | Update CLI in same PR. Test `cmd/nano-brain/workspaces_test.go`. |
| Test mocks in frontend may not match new shape | Search `web/src/__tests__` for `workspaces:` mocks, verify. Discovered already at `web/src/__tests__/CommandPalette.test.tsx:12` uses `{workspaces: []}` — already correct. |
| `chunk_count` query adds latency | One JOIN per list; cached for typical 18-workspace counts. Measure: < 5ms additional vs current. |
| Frontend strict mode may reject extra fields | Frontend uses TS interfaces (structural typing), extra fields are tolerated. `last_document_updated` already optional in spec. |

## Migration

No DB migration. No config change. No API version bump (still `/api/v1/`).

Existing external API consumers (if any) will see the response shape change. Document this in commit message and CHANGELOG.

Frontend already expects new shape — once backend is updated, UI starts working immediately, no rebuild needed for frontend.

CLI requires rebuild (Go binary) to handle new shape. Atomic update in same PR.
