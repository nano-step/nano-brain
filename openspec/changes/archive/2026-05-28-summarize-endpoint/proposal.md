## Why

Summarization currently only runs when a session is **newly harvested**. Sessions already in the DB (`skipped` on harvest) are never summarized — including all sessions ingested before summarization was enabled. There is no way to trigger summarization independently of harvest.

This means users who enable `summarization.enabled: true` on an existing install get zero summaries until new sessions appear, and have no recovery path.

## What Changes

- Add `POST /api/summarize` endpoint that queries already-harvested sessions from the DB and runs the summarization pipeline on them
- Support filtering by `source` (opencode/claude), `limit`, and `force` re-summarize flag
- Reuse existing `HarvestSummarizer` → `Pipeline` → `Persister` chain — no new summarization logic
- Return counts of summarized / skipped / errors

## Capabilities

### New Capabilities

- `summarize-endpoint`: `POST /api/summarize` triggers summarization of already-harvested sessions. Accepts optional `source` filter, `limit`, and `force` flag. Idempotent by default (content-hash skip in Persister). Requires summarization to be enabled in config.

### Modified Capabilities

None — existing harvest pipeline unchanged.

## Impact

- `internal/server/handlers/summarize.go` (new) — Echo handler, request/response structs
- `internal/server/server.go` — register new route
- `internal/server/server_deps.go` (or equivalent) — expose `SessionSummarizer` to handlers
- No schema changes
- No breaking API changes
- Closes #182
