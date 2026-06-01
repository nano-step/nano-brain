Tracking: #294

## Why

Today the only way to remove a workspace via the UI is to switch to it (top-left dropdown), navigate to `/ui/settings`, scroll to "Destructive Actions", and use "Remove workspace". For users with many workspaces (the dev container has 19), there is no overview of doc counts / staleness to help decide which ones to clean up. Removing N stale workspaces requires N switch-then-remove round trips.

Backend endpoints `GET /api/v1/workspaces` (returns hash + name + doc_count + chunk_count) and `DELETE /api/v1/workspaces/:hash` (cascade delete) already shipped in #277 / v2026.6.0102. Only the UI layer is missing.

## What Changes

- Add new React panel `web/src/panels/WorkspacesPanel.tsx` rendering a table of all workspaces from `GET /api/v1/workspaces` with columns: name, hash (truncated), docs, chunks, actions.
- Add new hook `web/src/panels/workspaces/useWorkspacesList.ts` (React Query `useQuery` against `/api/v1/workspaces`).
- Add new hook `web/src/panels/workspaces/useRemoveWorkspace.ts` (React Query `useMutation` against `DELETE /api/v1/workspaces/:hash`) which invalidates the list query on success.
- Per-row "Remove" button opens the existing `ConfirmDialog` (already used by Settings → Destructive Actions) requiring the user to type the workspace name. On confirm, mutation fires.
- Special case: if the user removes the currently-active workspace, switch to the first remaining workspace by updating the workspace cookie and reloading.
- Register route in `web/src/App.tsx`: new lazy route `/ui/workspaces` → `WorkspacesPanel`.
- Add nav link in `web/src/components/Nav.tsx` (or wherever main nav lives) labeled "Workspaces" with keyboard shortcut `g w`.

## Capabilities

### New Capabilities
- `workspaces-management-panel`: Defines the UI route and behavior for the workspaces overview + per-row remove flow.

### Modified Capabilities
None — the existing Settings → Destructive Actions "Remove workspace" path is preserved (acts on the current workspace only).

## Impact

- **Code:** ~250 lines across 4 new files + 2 small edits to App.tsx and Nav.tsx. No Go code changes.
- **Behavior:** New `/ui/workspaces` route with table + per-row remove. Existing settings path unchanged.
- **Risk:** Low — additive UI on top of existing API. DELETE is already battle-tested (used by Settings panel + CLI).
- **Performance:** GET /workspaces is O(W) with W ≤ ~50 in practice; one DB scan. Negligible.

## Out of Scope

- Bulk select / bulk delete — follow-up issue if requested
- Per-workspace stats beyond what GET returns (last_activity, size on disk, embedding count) — separate enhancement; backend would need a richer query
- Workspace rename — separate enhancement (no backend support yet)
- Backend changes — already shipped in #277, #278
