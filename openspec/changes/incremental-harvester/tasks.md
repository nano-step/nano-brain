## 1. Embedding Collection Filter (URGENT — prevents token bomb)

- [x] 1.1 In `store.ts` line 387, change `WHERE cv.hash IS NULL` to `WHERE cv.hash IS NULL AND d.collection != 'sessions'` in `getHashesNeedingEmbeddingStmt`.
- [x] 1.2 In `store.ts` line 396, add `AND d.collection != 'sessions'` to `getHashesNeedingEmbeddingByWorkspaceStmt` (before `LIMIT ?`).
- [x] 1.3 In `store.ts` line 404, add `AND d.collection != 'sessions'` to `getNextHashNeedingEmbeddingStmt`.
- [x] 1.4 In `store.ts` line 413, add `AND d.collection != 'sessions'` to `getNextHashNeedingEmbeddingByWorkspaceStmt`.

## 2. Empty Session Handling (fixes infinite retry loop)

- [x] 2.1 In `harvester.ts` line 262, change `state[sessionFile] = { mtime: lastMtime }` to `state[sessionFile] = { mtime: lastMtime, skipped: true }`.
- [x] 2.2 In `harvester.ts` line 275, change `state[sessionFile] = { mtime: lastMtime }` to `state[sessionFile] = { mtime: lastMtime, skipped: true }`.

## 3. Incremental Harvester

- [x] 3.1 In `harvester.ts`, add `messageCount?: number` to `HarvestStateEntry` interface (line 29-33).
- [x] 3.2 In `harvester.ts` `harvestSessions()`, after parsing messages (line 259) and before the `messages.length === 0` check: if `state[sessionFile]?.messageCount` exists and equals `messages.length`, skip the session (`continue`). Also skip if `messageCount > messages.length` (messages were deleted).
- [x] 3.3 In `harvester.ts`, after the `hasContent` check (line 278): determine `previousMessageCount = state[sessionFile]?.messageCount ?? 0`. If `previousMessageCount > 0` AND the output file exists, this is an incremental update — only process messages from index `previousMessageCount` onwards.
- [x] 3.4 For incremental updates (task 3.3): call `parseParts()` only for messages at index >= `previousMessageCount`. Format only the new messages using a new helper `messagesToMarkdown(parsedMessages)` (extract the message formatting loop from `sessionToMarkdown`).
- [x] 3.5 For incremental updates: use `appendFileSync(outputPath, newMarkdown)` instead of `writeFileSync(outputPath, fullMarkdown)`.
- [x] 3.6 For first-time harvests (no previous messageCount or no existing output file): keep current behavior — write full markdown with `writeFileSync`.
- [x] 3.7 Update state save (line 318) to include messageCount: `state[sessionFile] = { mtime: lastMtime, messageCount: messages.length }`.

## 4. Verification

- [x] 4.1 Run `lsp_diagnostics` on `store.ts` and `harvester.ts` — zero errors.
- [x] 4.2 Verify the 4 embedding SQL queries all contain `d.collection != 'sessions'`.
- [x] 4.3 Verify `HarvestStateEntry` has `messageCount` field.
- [x] 4.4 Verify empty sessions (messages.length === 0) get `skipped: true` in state.
- [x] 4.5 Verify incremental path uses `appendFileSync` and only processes new messages.
