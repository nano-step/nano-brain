## Context

The session harvester converts OpenCode session data into markdown files for nano-brain's FTS index. The current implementation has a full-rewrite architecture that causes cascading token waste.

**Current pipeline (per session update):**
```
Session gets 1 new message
  → harvester.ts: parseMessages() reads ALL messages
  → harvester.ts: sessionToMarkdown() generates FULL markdown
  → harvester.ts: writeFileSync() OVERWRITES entire file
  → watcher.ts: triggerReindex() reads file, computes new hash
  → store.ts: indexDocument() sees hash changed → deletes old embeddings
  → store.ts: insertContent() inserts full new content
  → codebase.ts: embedPendingCodebase() queues full content for VoyageAI
  → VoyageAI charges for ALL tokens (not just the new message)
```

**Compounding factors:**
- 913 session documents pending embedding (never filtered by collection)
- 25 subagent sessions stuck in infinite retry loop (retry counter reset bug)
- Active sessions trigger this full pipeline on every new message

## Goals / Non-Goals

**Goals:**
- Harvester only reads and writes NEW messages (incremental append)
- Sessions are never sent to VoyageAI for embedding (FTS only)
- Empty/subagent sessions are skipped immediately (no retry loop)
- Backward compatible with existing harvest state format

**Non-Goals:**
- Changing the markdown output format or frontmatter structure
- Modifying the chunker or FTS indexing strategy
- Adding session embedding as a configurable feature
- Changing how codebase or memory collections are embedded

## Decisions

### D1: Track messageCount in harvest state

Add `messageCount` field to `HarvestStateEntry`:
```typescript
interface HarvestStateEntry {
  mtime: number;
  retries?: number;
  skipped?: boolean;
  messageCount?: number;  // NEW
}
```

Backward compatible — existing entries without `messageCount` treated as 0 (triggers full harvest on first run after upgrade).

### D2: Incremental message reading

When `state[sessionFile].messageCount` exists and is less than current message count:
1. Parse ALL message metadata (needed for sorting by created time)
2. Sort messages by created time
3. Only call `parseParts()` for messages at index >= `messageCount`
4. Append formatted new messages to existing markdown file

**Why parse all metadata but only parts for new messages:** Message metadata (JSON parse) is cheap. `parseParts()` reads part files from disk — that's the expensive I/O. By parsing metadata for all messages, we maintain correct sort order. By only parsing parts for new messages, we skip the heavy I/O.

### D3: Append vs rewrite

**Append to existing file** using `appendFileSync()` instead of `writeFileSync()`.

The markdown format is append-friendly — each message is a `## User` or `## Assistant (agent)` section. New messages append naturally.

**Hash change is expected and acceptable.** The indexer will see the hash changed and re-index FTS. FTS re-indexing is a local SQLite operation (microseconds). The key win is that sessions are NOT embedded (D4), so hash changes don't trigger VoyageAI API calls.

### D4: Exclude sessions from embedding

Add `AND d.collection != 'sessions'` to all 4 embedding SQL queries in `store.ts`:
- `getHashesNeedingEmbeddingStmt` (line 382)
- `getHashesNeedingEmbeddingByWorkspaceStmt` (line 391)
- `getNextHashNeedingEmbeddingStmt` (line 399)
- `getNextHashNeedingEmbeddingByWorkspaceStmt` (line 408)

Sessions are searched via FTS (keyword search), which works without embeddings. Vector search for sessions provides minimal value — users search sessions by keywords ("what did we do about X"), not by semantic similarity.

### D5: Skip empty sessions immediately

When `messages.length === 0` or `hasContent === false`, set `skipped: true` in state immediately:
```typescript
state[sessionFile] = { mtime: lastMtime, skipped: true };
```

This fixes the infinite retry loop where:
1. Line 237 sets `retries = 1`
2. Fall-through re-harvest at line 262 creates `{ mtime: lastMtime }` (no retries)
3. Next cycle: retries = 1 again (never reaches 3)

### D6: Skip when messageCount unchanged

When `state[sessionFile].messageCount === messages.length`, skip the session even if mtime changed. Session metadata changes (title updates, etc.) don't add searchable content.

## Files Changed

| File | Changes |
|------|---------|
| `src/harvester.ts` | Add messageCount tracking, incremental append logic, skip empty sessions |
| `src/store.ts` | Add `d.collection != 'sessions'` to 4 embedding SQL queries |

## Risks

1. **First run after upgrade**: All sessions will be re-harvested once (messageCount = undefined → treated as 0). This is a one-time cost and produces the same output as before.
2. **Message deletion**: If OpenCode deletes messages from a session, messageCount would be higher than actual count. The harvester would skip the session (messageCount >= messages.length). This is acceptable — deleted messages don't need re-indexing.
3. **FTS re-index on append**: Appending changes the hash, triggering FTS re-index. This is cheap (local SQLite) and acceptable.
