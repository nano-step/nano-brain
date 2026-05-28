# Design: Harvest Summary-Only

## Research Findings (from investigation agents)

### OpenCode SQLite Schema (6,880 sessions)
- `session.time_updated` (unix ms) — last activity timestamp → usable for age gate
- `session.time_archived` — nullable, currently 0 sessions archived → not reliable for "completed" detection
- `project.worktree` — 100% populated, NOT NULL → correct source for wsHash derivation
- `session.workspace_id` — **NEVER populated (0/6880)** → must use `project.worktree`, not `session.workspace_id`
- Part types: `tool` (266K), `step-start/finish` (454K), `text` (147K) — tool parts dominate → stripping critical
- Avg 37 messages/session

### Resume Mechanism
- **Implicit**: `GetDocumentBySourcePath(sourcePath, wsHash)` + content_hash check — no checkpoint table
- **Full re-scan** every cycle but O(1) skip per session (DB lookup) → acceptable at 6,880 sessions
- **Embed queue**: DB-backed (`chunks.embed_status='pending'`), survives restart via `scanPending()` on startup
- **Backpressure**: rejects enqueue at 50,000 pending chunks — harvest continues but embed queuing stops

### Strip Effectiveness
- `StripOpenCode()` correctly handles OpenCode format (tested, covers system prompts, tool outputs, code blocks, base64, error dedup)
- 100K chars raw → ~40-60K chars stripped → ~12-16K tokens input
- `max_tokens: 4800` sufficient for summary output
- Pipeline has map-reduce built-in: `SingleShotThreshold=4000` chars, `ReduceContextLimit=50000` chars
- Model context window massively underutilized (using ~20K of 128-200K available)

### Active Session Detection
- No "session closed" signal in schema (no `status` column, no `time_completed`)
- Best signal: `time_updated < NOW() - 10min` → session likely not actively running
- `time_archived IS NOT NULL` exists but unused in practice

# Design

## Scope Decisions (from user)

| Question | Decision |
|----------|----------|
| Which sessions to summarize? | **All sessions** (no cutoff) |
| First-time backfill (slow)? | **Acceptable** — first time can be slow, must not crash |
| Active session handling? | **Age gate**: skip sessions with `time_updated > NOW() - 10min` |
| LLM call failure? | **Fallback to raw** — never lose a session |
| File write to disk? | **No** — DB-only persistence |
| Backfill concurrency? | Existing `concurrency: 2` + `requests_per_second: 2` in config is sufficient |

## Current Flow (Broken)

```
OpenCode SQLite DB
  └─ OpenCodeSQLiteHarvester.HarvestAll()
       ├─ render raw markdown (full transcript, all tool outputs)
       ├─ UpsertDocument(raw, collection="sessions", wsHash=per-session)  ← raw stored
       ├─ chunk + enqueue raw chunks
       └─ [if summarizer != nil] SummarizeAndPersist(raw)
            └─ Persister.Save(summary, workspace="")  ← BUG: empty workspace
                 ├─ writeFile → ~/.nano-brain/summaries/*.md  ← unnecessary
                 └─ UpsertDocument(summary, collection="session-summary", wsHash="")  ← wrong workspace
```

**Problems:**
- `sessions` and `session-summary` both indexed → double embed queue pressure
- `session-summary` indexed under empty workspace → invisible to per-project queries
- `writeFile` writes `.md` to disk — not needed, watcher doesn't watch summaries dir
- Summarizer wired via `hr.WithSummarizer(s)` after `NewRunner()` — race if first harvest fires immediately
- `Persister` workspace always `""` (main.go line 378: `summarize.NewPersister(db, ..., "", eq, logger)`)

## Age Gate Design

```go
const activeSessionGrace = 10 * time.Minute

// isSessionActive returns true if session was updated recently (likely still running)
func isSessionActive(sess SqSession) bool {
    return time.Since(sess.UpdatedAt) < activeSessionGrace
}
```

- `SqSession` needs `UpdatedAt time.Time` field (from `session.time_updated`)
- `listSessions` query must include `time_updated` column (currently fetches `time_created` only → **query change needed**)
- Active sessions: harvested = 0, skipped++ (not counted as error)

## Target Flow

```
OpenCode SQLite DB
  └─ OpenCodeSQLiteHarvester.HarvestAll()
       └─ listSessions() → includes time_updated
            └─ for each sess:
                 ├─ [NEW] if isSessionActive(sess) → skip (session likely running)
                 ├─ derive wsHash from sess.Worktree (via project JOIN — already exists)
                 ├─ sourcePath = "summary://opencode/<id>"  (UNIFIED — always, regardless of summarizer state)
                 ├─ check existing: GetDocumentBySourcePath(sourcePath, wsHash)
                 │    └─ if exists + content_hash unchanged → skip
                 ├─ listMessages → render raw markdown (in-memory only, NOT stored)
                 ├─ [if summarizer != nil]
                 │    ├─ SummarizeAndPersist(raw, SummaryMeta{WorkspaceHash: wsHash, ...})
                 │    │    → persists doc at summary://opencode/<id>, collection="session-summary"
                 │    ├─ on success → summary_success++
                 │    └─ on error  → fallback: UpsertDocument(raw, sourcePath, collection="sessions", metadata={"fallback":true}) → summary_fallback++
                 │                   (DB upsert error during fallback also caught → log + skip session)
                 └─ [if summarizer nil]
                      └─ UpsertDocument(raw, sourcePath, collection="sessions", metadata={"fallback":true}) → summary_fallback++
```

## Key Design Decisions

### 1. Skip-check uses summary source path (CRITICAL FIX — UNIFIED PATH)

**Final decision (post deep-design B1)**: Always use `summary://<source>/<id>` as the source_path, regardless of whether summarizer is active, nil, or failing. The `collection` field (`session-summary` vs `sessions`) and `metadata.fallback` (true/false) distinguish summary docs from fallback raw docs. The source_path itself stays unified.

**Why unified?** If fallback writes to a different path (`opencode://session/<id>`) than the skip-check looks at (`summary://opencode/<id>`), then during sustained LLM outage:
- Cycle N: skip-check finds nothing → re-renders → LLM fails → raw fallback written to `opencode://session/<id>`
- Cycle N+1: skip-check looks for `summary://opencode/<id>` → not found → re-renders again → infinite loop

**Fix:** Fallback raw MUST upsert under the same path the skip-check uses (`summary://<source>/<id>`), with `collection="sessions"` and `metadata.fallback=true`. The unified-path approach:
- survives restarts (no in-memory state)
- makes rollback safe (toggling `Enabled=false` doesn't change source_path)
- lets operators query fallback docs via `WHERE metadata->>'fallback' = 'true'` to identify backfill candidates

**Trade-off acknowledged (post deep-design B2):** Fallback docs at `summary://` path are **permanent by design**. When LLM recovers, the existing fallback doc's content_hash matches the rendered raw → skip-check returns "already processed" → no auto re-summarization. To re-summarize a fallback session, operators must either (a) add new messages to the session (changes content_hash) OR (b) manually `DELETE FROM documents WHERE source_path = 'summary://opencode/<id>' AND collection='sessions'` then re-harvest. Future work: `nano-brain harvest --resummarize-fallbacks` CLI flag (separate ticket).

### 2. Workspace threading into Persister
`Persister.Save()` currently uses `p.workspace` (set at init time, always `""`). Two options:

- **Option A**: Add `workspace` param to `Save(ctx, markdown, meta, wsHash)` — breaks interface
- **Option B**: `HarvestSummarizer.SummarizeAndPersist` receives wsHash via `SummaryMeta` → Persister uses it

**Decision: Option B** — add `WorkspaceHash` to `harvest.SummaryMeta`, thread into `Persister.Save` via `SessionMetadata`. No interface break on `SessionSummarizer`.

**⚠️ Oracle Finding 2 — 3 hardcoded `p.workspace` references (MEDIUM):**
`persist.go` uses `p.workspace` at lines **62, 94, and 112** — the idempotency lookup, document upsert, and chunk upsert. All three MUST switch to `meta.WorkspaceHash`. Missing even one causes summaries to land in wrong workspace. After fix, remove `p.workspace` field from `Persister` struct entirely to prevent regression.

### 3. Remove file write
`Persister.writeFile()` writes to `~/.nano-brain/summaries/`. The watcher does NOT watch this dir. File write adds latency and disk I/O with no benefit. **Remove the `writeFile` call from `Persister.Save()`**. Keep the function but don't call it (or delete it).

### 4. Init order fix in main.go
Current:
```go
oh := harvest.NewOpenCodeSQLiteHarvester(db, logger, cfg.Harvester.OpenCode.DBPath)
hr = harvest.NewRunner(oh, eq, interval, logger)
// ... later ...
harvestSummarizer = summarize.NewHarvestSummarizer(pipeline, persister, logger)
hr.WithSummarizer(harvestSummarizer)  // ← post-init
```

Target:
```go
var harvestSummarizer *summarize.HarvestSummarizer
if cfg.Summarization.Enabled {
    // init summarizer first
    harvestSummarizer = summarize.NewHarvestSummarizer(pipeline, persister, logger)
}
oh := harvest.NewOpenCodeSQLiteHarvester(db, logger, cfg.Harvester.OpenCode.DBPath).
    WithSummarizer(harvestSummarizer)  // nil-safe
hr = harvest.NewRunner(oh, eq, interval, logger)
```

### 5. Backwards compatibility
If `cfg.Summarization.Enabled = false` or summarizer init fails → `harvestSummarizer` is nil → harvester falls back to raw storage. No behavior change for users without LLM config.

## LLM Pipeline for 100K Sessions

```
Raw session (100K chars)
  │
  ▼ StripOpenCode()
  ~40-60K chars (strips tool outputs, system prompts, code blocks, base64)
  │
  ▼ pipeline.Summarize()
  if stripped < 4000 chars → single LLM call
  else → map-reduce:
    ├─ Chunk into ~4K pieces → N chunks
    ├─ Map: N parallel LLM calls (bounded by concurrency:2)  ← each call: ~4K in, ~800 out
    └─ Reduce: 1 LLM call (all chunk summaries → final)      ← single call: ~8K in, ~4800 out
  │
  ▼ Persister.Save()
  UpsertDocument(summary, collection="session-summary", wsHash=correct)
  + chunks + embed queue
```

**Timing estimate per session (100K chars):**
- Strip: <1ms
- Map phase: ~6 chunks × 2s/call ÷ concurrency:2 = ~6s
- Reduce phase: ~2s
- DB persist: ~50ms
- Total: ~8-10s per session

**First-time backfill (6,880 sessions, assume 50% already harvested):**
- 3,440 new sessions × 8s ÷ concurrency:2 = ~13,760s ≈ **4 hours**
- Acceptable per user decision ("first time thì lâu cũng không sao")
- Rate limited by `requests_per_second: 2` config → won't crash LLM provider

## File Changes Summary

| File | Change |
|------|--------|
| `internal/harvest/opencode_sqlite.go` | Add age gate (skip active sessions); conditional skip-check path; summarize-first with raw fallback; add `UpdatedAt` to `SqSession` |
| `internal/harvest/opencode_sqlite.go` | `listSessions()` query: add `time_updated` column |
| `internal/harvest/claudecode.go` | Same summarize-first pattern; confirm wsHash derivation |
| `internal/harvest/harvest.go` | Add `WorkspaceHash string` to `SummaryMeta` |
| `internal/summarize/pipeline.go` | Add `WorkspaceHash string` to `SessionMetadata` |
| `internal/summarize/persist.go` | Use `meta.WorkspaceHash` instead of `p.workspace`; remove `writeFile` call |
| `internal/summarize/harvest_adapter.go` | Thread `WorkspaceHash` from `SummaryMeta` → `SessionMetadata` |
| `cmd/nano-brain/main.go` | Init summarizer before harvester; graceful degradation if LLM init fails |

## Risks

| Risk | Mitigation |
|------|-----------|
| LLM call fails during harvest → session never indexed | `SummarizeAndPersist` failure → log warn + fallback upserts raw under **same** `summary://opencode/<id>` path (Oracle Fix 1) |
| sourcePath flip-flop causes infinite retry storm during outage | Fallback uses same path as skip-check (Oracle Finding 1 fix) |
| LLM latency slows harvest loop | Already bounded by `concurrency` + `requests_per_second` in config |
| Existing `sessions` collection has stale raw docs after migration | One-time cleanup: after backfill, delete `opencode://session/<id>` docs where matching `summary://opencode/<id>` exists (Oracle Finding 3 — tracked as follow-up) |
| 3 hardcoded `p.workspace` in persist.go | All 3 lines (62, 94, 112) must use `meta.WorkspaceHash`; remove `p.workspace` field after (Oracle Finding 2) |
| `claudecode.go` may not have wsHash readily available | Inspect current implementation — may need same wsHash derivation pattern as opencode_sqlite |

## Post-Implementation Follow-Up (Out of Scope for This Change)

- **Stale raw doc cleanup** (Oracle Finding 3): One-time migration to delete `opencode://session/<id>` raw docs from `sessions` collection where a matching `summary://opencode/<id>` exists. Prevents duplicate results in search. Track as separate issue.
- **Increase `max_tokens`** (ai-proxy finding): Current `max_tokens: 4800` is safe but conservative. Can increase to `8000–10000` for richer summaries — Sonnet 4.6 has 200K output capacity. Track as config tuning follow-up.
