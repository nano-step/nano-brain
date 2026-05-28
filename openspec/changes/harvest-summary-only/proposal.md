## Why

Currently `OpenCodeSQLiteHarvester` stores the **raw session transcript** (full tool outputs, system prompts, XML blobs) as a document in collection `sessions`, then optionally calls the summarizer as a side-effect. This means:

1. **Both raw + summary are indexed** — doubles storage and embed queue pressure
2. **Raw transcripts pollute search** — tool call JSON, system prompts, base64 blobs surface as search results
3. **Workspace bug** — `Persister` is initialized with `workspace: ""`, so summary documents land in the wrong workspace and are never returned by per-project queries
4. **Summarizer initialized after runner** — `WithSummarizer()` called after `NewRunner()`, creating a race condition

The summarization infrastructure is already built (Pipeline, Persister, HarvestSummarizer). This change rewires the harvest flow to use it correctly: **summarize first, store only the summary**.

## What Changes

- `opencode_sqlite.go`: If summarizer is set, summarize session content first → store summary document only. Skip raw `UpsertDocument`. Fallback to raw if summarizer nil (backwards-compatible).
- `claudecode.go`: Same pattern — summarize first, store summary only if summarizer set.
- `internal/summarize/persist.go`: Fix workspace — use per-session `wsHash` instead of empty string.
- `cmd/nano-brain/main.go`: Init summarizer **before** `NewRunner`, pass via constructor not post-init `WithSummarizer`.
- Collection `sessions` becomes unused when summarizer is active (summary goes to `session-summary`).
- Remove file write from `Persister.writeFile` — DB-only persistence (no disk `.md` files needed).

## Capabilities

### Modified
- `harvest-opencode`: No longer stores raw transcripts when summarizer is configured
- `harvest-claudecode`: Same
- `summary-persistence`: Fixed workspace, DB-only (no file write)
- `main-wiring`: Summarizer init order fixed — before runner creation

### Removed Capability
- Raw session transcripts in collection `sessions` (when summarizer enabled)

## Impact

- `internal/harvest/opencode_sqlite.go` — ~40 lines changed
- `internal/harvest/claudecode.go` — ~20 lines changed
- `internal/summarize/persist.go` — workspace param threading, remove writeFile call
- `cmd/nano-brain/main.go` — init order fix ~15 lines
- No new dependencies
- No DB migrations needed
- Backwards compatible: summarizer nil → raw harvest unchanged
