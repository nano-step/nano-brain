# Analysis: nano-brain VoyageAI Token Bloat & Harvester "Output File Missing"

**Date:** 2026-03-08
**Status:** Research complete, no code changes made

---

## Issue 1: VoyageAI Token Bloat — Ticking Time Bomb

**Summary:** 913 harvested session documents are pending embedding. The embedding pipeline has NO collection filtering — it will embed ALL pending documents. Next embed cycle will send ~3.8M tokens to VoyageAI.

**Data:**

| Collection | Docs | Content Size | Embedded? | Pending Tokens (est.) |
|---|---|---|---|---|
| codebase | 8,725 | 49 MB | ✅ 19,441 chunks | ~10K pending |
| sessions | 913 | 11.5 MB | ❌ ZERO | ~3,820K pending |
| memory | 34 | 0.2 MB | ✅ 119 chunks | ~13K pending |

**Current tracked usage:** 156,494 tokens (voyage-code-3, 6 requests). Sessions would **25x** that.

**Root cause code:** `store.ts` `getHashesNeedingEmbedding()` SQL query selects ALL pending documents without WHERE clause on collection:

```sql
SELECT c.hash, c.body, d.path FROM content c
JOIN documents d ON d.hash = c.hash AND d.active = 1
LEFT JOIN content_vectors cv ON cv.hash = c.hash
WHERE cv.hash IS NULL LIMIT ?
```

**Embedding pipeline:** `codebase.ts` `embedPendingCodebase()` processes 50 docs/batch, 200 chunks/batch, 200K chars/API call (~66.7K tokens), 40 rpm limit.

**Fix needed:** Add collection filtering to embedding query, or add config option like `embedding.collections: [codebase, memory]`.

---

## Issue 2: "Output File Missing" — Subagent Sessions Without Messages

**Summary:** 25 sessions repeatedly trigger "output file missing, retry N" in harvester logs. ALL 25 are subagent sessions (@explore, @librarian, @sisyphus-junior, @prometheus) or empty "New session" entries.

**Root cause:** These sessions have session JSON files in OpenCode storage but NO message directories. oh-my-opencode creates session metadata for subagents but messages live in the parent session or are cleaned up after completion.

**Verification:**

- All 25 failing sessions: message dir = DOES NOT EXIST
- 24/25 are in the `global` project directory
- Titles contain `@explore subagent`, `@librarian subagent`, etc.
- Normal sessions (checked 50 random): ALL have message directories

**Harvester bug (harvester.ts lines 227-251):**

1. Session mtime unchanged AND output file missing → enters retry path
2. Increments retry counter, logs "output file missing"
3. Hits `continue` at line 251 → SKIPS actual harvest code (lines 253-325)
4. Even if it re-harvested, `parseMessages()` returns empty → no output written
5. After 3 retries → permanently skipped

**Additional bug:** On first harvest, when `messages.length === 0`, state is saved with mtime (line 262) but no output file is written. This creates the inconsistency — state says "processed" but no output exists.

**Fix needed:** Detect "no messages" on first attempt and mark as skipped immediately. Or filter out sessions from `global/` that lack message directories.

---

## Issue 3: Storage Eviction (NOT the cause here, but documented)

- No `storage:` section in config.yml — eviction is NOT configured
- `storage.ts` has `evictExpiredSessions()` and `evictBySize()` that DELETE markdown output files
- `watcher.ts` calls eviction IMMEDIATELY after harvest (potential race condition if eviction were enabled)
- If eviction deletes an output file, the harvester retry path never re-generates it (same `continue` bug as above)

---

## Issue 4: oh-my-opencode Session Management

- oh-my-opencode v3.10.0 HAS session deletion capabilities: `session_delete` tool, `session.deleted` events, `stripThinkingParts()`
- OpenCode CLI has native `opencode session delete <id>`
- BUT: failing sessions are NOT deleted — their JSON still exists. They simply never had message directories.
- `stripThinkingParts()` deletes thinking-type part files but doesn't remove entire message directories

---

## Harvest State Statistics

- Total tracked in `.harvest-state.json`: 956 entries
- MD files on disk: 913
- Gap: 43 sessions (4.5%) — includes the 25 subagent sessions + ~18 others that may have had empty content
- All entries have only `{mtime: number}` — no retries, no skipped flags (retries reset to 0 after state rewrite)
- All 913 MD files contain valid session IDs in YAML frontmatter matching harvest state entries

---

## Two Issues Are Related But Distinct

| Issue | Cause | Impact |
|---|---|---|
| **Token bloat** | 913 sessions pending embedding, no collection filter in embed query | ~3.8M VoyageAI tokens will be consumed on next embed cycle |
| **Output file missing** | Subagent sessions have session JSON but no message dirs | CPU/IO waste on retry loop, noisy logs, no direct token cost |

The harvester successfully processes ~913 sessions that DO have messages → those become documents in the DB → those are pending embedding → **that's the token bloat**.

The ~25 subagent sessions that LACK messages → never produce output files → stuck in retry loop → **that's the log noise**.

---

## Recommendations (prioritized)

1. **URGENT — Prevent token bomb:** Add collection filter to `getHashesNeedingEmbedding()` before next embed cycle runs. Sessions should NOT be auto-embedded without explicit opt-in.
2. **HIGH — Fix harvester for empty sessions:** Mark sessions with no messages as `skipped` on first attempt instead of saving mtime and creating retry loop.
3. **MEDIUM — Fix harvester retry logic:** When output file is missing and session hasn't changed, re-run full harvest instead of just incrementing retry counter (remove the `continue` at line 251, or restructure the conditional).
4. **LOW — Add embedding config:** Support `embedding.collections` in config.yml to control which collections get embedded.
5. **LOW — Add harvest validation:** After bulk harvest, compare state entries vs output files and log discrepancies.

---

## Files Involved

| File | What |
|---|---|
| `src/store.ts` | `getHashesNeedingEmbedding()` — no collection filter |
| `src/codebase.ts` | `embedPendingCodebase()` — embedding pipeline |
| `src/harvester.ts` | Lines 227-251 (retry bug), Line 262 (empty session state save) |
| `src/storage.ts` | `evictExpiredSessions()`, `evictBySize()` — not active but risky |
| `src/watcher.ts` | Calls harvest then eviction (race condition if eviction enabled) |
| `src/embeddings.ts` | VoyageAI API call, 200K chars/batch, 40 rpm |
| `src/server.ts` | `memory_index_codebase` handler triggers embedding for all pending |
| `~/.nano-brain/config.yml` | No storage section, no embedding collection filter |
| `~/.nano-brain/sessions/.harvest-state.json` | 956 entries, 913 with output files |
