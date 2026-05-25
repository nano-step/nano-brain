## Why

The session harvester has two compounding bugs that cause excessive VoyageAI token consumption:

1. **Full-rewrite on every change** — When a session gets 1 new message, the harvester re-reads ALL messages, rewrites the ENTIRE markdown file, producing a new content hash. The indexer sees the hash change, deletes all old embeddings, re-indexes the full content, and queues the entire session for re-embedding. For an active session with 50 messages, adding message #51 means re-embedding all 51 messages worth of tokens.

2. **No collection filter on embedding query** — `getHashesNeedingEmbedding()` in `store.ts` returns documents from ALL collections (codebase, sessions, memory) without filtering. 913 session documents (11.5 MB, ~3.8M estimated tokens) are pending embedding. The function is named `embedPendingCodebase` but embeds everything.

3. **Infinite retry loop for empty sessions** — 25 subagent sessions with no messages enter a retry loop that never terminates. The retry counter increments (line 237) but the re-harvest path overwrites state without preserving retries (line 262), resetting the counter every cycle.

**Current token usage:** 156,494 tokens (6 requests). Pending: ~3.8M tokens from sessions alone.

## What Changes

### Incremental Harvester
- Track `messageCount` in harvest state alongside `mtime`
- When a session has new messages: read ONLY the new messages, append to existing markdown
- When message count unchanged: skip entirely (even if mtime changed from metadata updates)
- First harvest: write full markdown as before

### Embedding Collection Filter
- Add `AND d.collection != 'sessions'` to all 4 embedding SQL queries in `store.ts`
- Sessions are for FTS (full-text search) only — embedding them provides no value and wastes tokens

### Empty Session Handling
- Mark sessions with 0 messages or 0 text content as `skipped: true` immediately on first detection
- Prevents the infinite retry loop caused by retry counter reset

## Impact

- **Immediate**: Prevents ~3.8M token VoyageAI charge from pending session embeddings
- **Ongoing**: Active sessions no longer trigger full re-embedding on every new message
- **I/O**: Harvester reads only new messages instead of all messages per session
- **Logs**: Eliminates "output file missing" noise from 25 subagent sessions
