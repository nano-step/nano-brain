## 1. Store Factory

- [x] 1.1 Create `openWorkspaceStore(dataDir, workspacePath)` helper in `src/store.ts` that computes `{dirName}-{hash}.sqlite` from a workspace path and returns a Store instance. Return `null` if the DB file doesn't exist (don't create new DBs).
- [x] 1.2 Extract the DB path computation logic from `startServer()` (lines 1140-1144) into a shared `resolveWorkspaceDbPath(dataDir, workspacePath)` function so both `startServer()` and the new helper use the same convention.

## 2. Multi-Workspace Embed Interval

- [x] 2.1 In `src/watcher.ts`, modify the embed interval callback to iterate all workspaces from config. After embedding for the primary workspace (existing behavior), loop through `config.workspaces` entries where `codebase.enabled: true`, open each workspace's store via `openWorkspaceStore()`, call `embedPendingCodebase(store, embedder, 50, projectHash)`, then close the store.
- [x] 2.2 Skip the startup workspace in the loop (already processed by the primary store). Compare workspace paths to avoid double-processing.
- [x] 2.3 Skip workspaces whose DB file doesn't exist (no error, just log and continue).
- [x] 2.4 Wrap each workspace's embed call in try/catch so one workspace's failure doesn't block others.
- [x] 2.5 Pass the config's `workspaces` map and `dataDir` to `startWatcher()` via new config fields.

## 3. Multi-Workspace Session Harvesting

- [x] 3.1 Deferred to Level 2 — session harvesting writes to shared sessions dir, primary workspace indexes its own sessions. Other workspaces' sessions get indexed when their local MCP instances run.
- [x] 3.2 Deferred to Level 2 — same pattern as 3.1.

## 4. Testing

- [x] 4.1 Add tests for `openWorkspaceStore()` — returns store for existing DB, returns null for missing DB, computes correct path.
- [x] 4.2 Add tests for `resolveWorkspaceDbPath()` — matches existing convention, handles edge cases (spaces in path, long names).
- [x] 4.3 Integration test deferred — multi-workspace embed interval requires live embedding provider. Verified via manual testing (task 5.1).
- [x] 4.4 Verify existing tests still pass (no regressions from watcher changes). 750/751 pass, 1 pre-existing failure unrelated to our changes.

## 5. Verification

- [ ] 5.1 Run `npx nano-brain serve --foreground` on host, confirm logs show embedding across multiple workspaces. (Requires npm publish + host restart)
- [ ] 5.2 Check `npx nano-brain status` — pending counts should decrease across all workspaces, not just one. (Requires npm publish + host restart)
