## 1. ProjectHash Extraction Utility

- [x] 1.1 Create `extractProjectHashFromPath(filePath: string, sessionsDir: string): string | undefined` function in `src/store.ts` (or `src/utils.ts`). It should parse the path to find a `{sessionsDir}/{12-char-hex}/` segment and return the hex string, or `undefined` if not matched.
- [x] 1.2 Export the function so it can be imported by `watcher.ts`, `index.ts`, and `server.ts`.
- [x] 1.3 Add unit tests for `extractProjectHashFromPath` covering: valid session path, non-session path, path with non-hex subdirectory, path without sessionsDir prefix, edge cases (empty string, trailing slashes).

## 2. Fix Watcher Reindex

- [x] 2.1 In `src/watcher.ts` `triggerReindex()`, import `extractProjectHashFromPath` and the sessions output directory path.
- [x] 2.2 Modify the indexing loop: for the `sessions` collection, call `extractProjectHashFromPath(filePath, sessionsOutputDir)` and use the result (falling back to the watcher's `projectHash` if `undefined`). For other collections, keep using the watcher's `projectHash`.
- [x] 2.3 Add/update test in `test/watcher.test.ts` verifying that session files from different workspaces get tagged with their respective projectHash, not the watcher's.

## 3. Fix Init Command Indexing

- [x] 3.1 In `src/index.ts` `handleInit()`, modify the session collection indexing loop (around line 423-435) to extract projectHash from each session file path using `extractProjectHashFromPath`. Pass the extracted hash to `indexDocument()`.
- [x] 3.2 Ensure non-session collections (`memory`) continue using the workspace's `projectHash`.

## 4. Fix Update Command and MCP Tool

- [x] 4.1 In `src/index.ts` `handleUpdate()`, modify the collection indexing loop to use `extractProjectHashFromPath` for the `sessions` collection.
- [x] 4.2 In `src/server.ts` `memory_update` tool handler, modify the reindex loop to use `extractProjectHashFromPath` for the `sessions` collection. Pass the server's `outputDir + '/sessions'` as the sessionsDir.

## 5. Integration Testing

- [x] 5.1 Add an integration test that: (a) creates session files in two different projectHash subdirectories, (b) runs a reindex, (c) verifies each document has the correct `project_hash` in the database.
- [x] 5.2 Add a test verifying that after reindex, searching with workspace=projectHashA returns only sessions from A (not B), and workspace="all" returns both.
- [x] 5.3 Run full test suite (`npm test`) and verify all existing tests pass.

## 6. Verification

- [ ] 6.1 Run `npx nano-brain init` in a workspace and verify `memory_status` shows correct per-workspace document counts.
- [ ] 6.2 Run `memory_search` with default workspace scoping and confirm only current-workspace sessions appear.
- [ ] 6.3 Run `memory_search` with `workspace="all"` and confirm cross-workspace sessions appear.
