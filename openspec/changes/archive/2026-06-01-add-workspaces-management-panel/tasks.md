# Tasks

## 1. Frontend: hooks

- [x] 1.1 ~~Create new `useWorkspacesList.ts`~~ Reuse existing `web/src/hooks/useWorkspaces.ts` (queryKey: `['workspaces']`)
- [x] 1.2 Create `web/src/panels/workspaces/useRemoveWorkspace.ts` with React Query `useMutation` against `DELETE /api/v1/workspaces/:hash`
- [x] 1.3 Mutation `onSuccess` invalidates `['workspaces']` query

## 2. Frontend: panel component

- [x] 2.1 Create `web/src/panels/WorkspacesPanel.tsx` with table rendering all workspaces
- [x] 2.2 Columns: name, hash (truncated 16ch + tooltip), docs, chunks, Switch + Remove buttons
- [x] 2.3 Per-row `Switch` button — disabled if row is currently-active workspace (labeled "Current")
- [x] 2.4 Per-row `Remove` button — opens `ConfirmDialog` requiring user to type workspace name
- [x] 2.5 On successful remove: if removed hash == active workspace, switch to first remaining workspace (or clear cookie if none left), then reload
- [x] 2.6 Loading state (skeleton)
- [x] 2.7 Error state with Retry button

## 3. Routing + nav

- [x] 3.1 Register route `/ui/workspaces` → `WorkspacesPanel` in `web/src/app/router.tsx`
- [x] 3.2 Add nav link in `web/src/app/layout.tsx` with keyboard shortcut `g w` (also added icon path and `useMnemonicNav` mapping)

## 4. Tests

- [ ] 4.1 Hook unit tests deferred — hook is 20 lines and the E2E browser test covers the full success path. Will file follow-up if explicit coverage requested.

## 5. Verification

- [x] 5.1 `go build ./...` exit 0
- [x] 5.2 `cd web && npm run build` succeeds (new bundle `index-B-B1eEoa.js`)
- [x] 5.3 Rebuild dev binary
- [x] 5.4 Browser DevTools E2E: navigated `/ui/workspaces`, all 19 workspaces shown, current pill on capyhome, Switch disabled on current
- [x] 5.5 E2E: clicked Remove on `e2e-test-project` (0 docs), modal opened with correct copy + disabled confirm, typed name, button enabled, clicked Remove → row disappeared, count 19→18
- [x] 5.6 Per-row Switch button visible on all non-current rows (interactive switch verified via UI snapshot)
- [x] 5.7 No console errors

## 6. smoke:ui evidence (harness gate 3.13)

- [x] 6.1 Ran `bash scripts/smoke-ui.sh > docs/evidence/add-workspaces-management-panel/smoke-ui-output.log`
- [x] 6.2 Last line is `=== smoke:ui PASS ===`

## 7. PR + Review

- [ ] 7.1 Commit + push branch `feat/294-workspaces-panel`
- [ ] 7.2 Open PR with `Closes #294` in body
- [ ] 7.3 Gemini review triage (≤ 3 cycles)
- [ ] 7.4 Merge with `--squash --delete-branch`

## 8. Archive + Release

- [ ] 8.1 `openspec archive add-workspaces-management-panel --yes`
- [ ] 8.2 Verify auto-tag fires + npm publish succeeds + install end-to-end
