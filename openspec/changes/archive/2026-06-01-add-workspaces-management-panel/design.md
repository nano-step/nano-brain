# Design — Workspaces Management Panel

## Goals

1. Single place to see all workspaces with the stats needed to decide which to remove.
2. Per-row remove with the same safety dialog used by Settings (type workspace name to confirm).
3. Zero backend changes — leverage `GET /api/v1/workspaces` + `DELETE /api/v1/workspaces/:hash` already shipped.

## Non-goals

- Bulk selection.
- Per-workspace fine-grained stats (size on disk, last-activity timestamp, embedding count). Out of scope; would require new backend endpoint.
- Workspace rename or merge.

## Decisions

### D1. Standalone route vs. settings sub-section
**Decision:** Standalone route `/ui/workspaces`.

**Rationale:** Discoverability — the existing Settings panel buries destructive actions at the bottom. A dedicated route surfaced in main nav (next to Memory / Graph / Symbols / Harvest / Settings) makes workspace management a first-class UX. Matches Obsidian / Logseq mental model.

**Alternatives considered:**
- Section inside `/ui/settings` — discoverable only after entering settings, no overview at-a-glance.
- Modal triggered from workspace switcher dropdown — limited table real-estate, breaks the "panel-per-page" pattern.

### D2. Hooks structure
**Decision:** Mirror the `panels/graph/use*.ts` pattern. Two hooks:
- `useWorkspacesList` — React Query `useQuery({queryKey:['workspaces'], queryFn})` against existing `apiFetch('/api/v1/workspaces')`.
- `useRemoveWorkspace` — React Query `useMutation` against `apiFetch('/api/v1/workspaces/'+hash, {method:'DELETE'})` which on success calls `queryClient.invalidateQueries({queryKey:['workspaces']})`.

**Rationale:** Consistent with existing `useGraphOverview` / `useGraphNeighborhood` from #287. Frontend conventions.

### D3. Confirm-by-typing-name dialog
**Decision:** Reuse the existing `ConfirmDialog` component used by Settings → Destructive Actions (path: `web/src/components/ConfirmDialog.tsx` based on session memory; verify location during implementation).

**Rationale:** Same destructive UX everywhere. Single component to maintain.

### D4. What if user removes their currently-active workspace?
**Decision:** After successful DELETE, if the removed hash equals `getCurrentWorkspace()`, set the workspace cookie to the first remaining workspace's hash and reload the page (or use `setCurrentWorkspace + queryClient.invalidateQueries()` if a softer transition works).

**Rationale:** The app keys nearly every API call on the active workspace; leaving the user on a deleted workspace would produce 4xx errors on every page. Page reload is the safest baseline; can be refined later.

**Edge case:** If the user removes the LAST workspace, redirect to `/ui/dashboard` with an empty state explaining how to register a new workspace via `POST /api/v1/init` or CLI.

### D5. Empty state copy
- 0 workspaces total: "No workspaces registered. Use `nano-brain init --root=<path>` or POST to /api/v1/init."
- After removing all but one: standard table with 1 row, no special copy.

### D6. Error handling
- GET fails → show inline error with retry button (React Query refetch).
- DELETE fails → keep dialog open, show error message inside dialog, do NOT invalidate list.

## Data shape

Existing `GET /api/v1/workspaces` returns:
```json
{
  "workspaces": [
    {"hash": "37b3...", "name": "capyhome", "doc_count": 9061, "chunk_count": 12704}
  ]
}
```

Frontend type already exists in `web/src/api/types.ts` as `WorkspaceListItem` (verify name during implementation; if differs, use whatever the existing typed wrapper exports).

## Test plan

1. **Unit / hook:** Mock `apiFetch` returning 2 workspaces; assert table renders both rows.
2. **Hook:** Mock DELETE returning 200; assert list refetched.
3. **Browser E2E (port 3199):**
   - Navigate `/ui/workspaces` → table populated with capyhome + 18 others
   - Click "Remove" on a test workspace (`e2e-test-project` — 0 docs, safe to delete)
   - Type wrong name → confirm button disabled
   - Type correct name → confirm enabled, click → 200, row disappears from table
   - Reload page → row stays removed
4. **smoke:ui.sh** evidence file ending in `=== smoke:ui PASS ===`.

## Rollback

Pure additive change. Rollback = revert the PR; the existing Settings → Destructive Actions path is unaffected.
