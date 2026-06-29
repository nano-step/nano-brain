---
phase: "08"
plan: "02"
subsystem: harvest
tags: [harvest, tickets, relationship-lookup, branch, cwd, parent-id, tags]
dependency_graph:
  requires: [08-01-SessionSource-interface, 08-01-NormalizedSession-model, 08-01-Engine-harvester]
  provides: [Branch-Cwd-ParentID-in-front-matter, TicketExtractor, DBRelationshipLookup, ticket-tags-on-documents]
  affects: [internal/harvest, internal/summarize, internal/config, cmd/nano-brain]
tech_stack:
  added: []
  patterns: [regex-extraction, tag-inheritance, relationship-lookup, best-effort-enrichment]
key_files:
  created:
    - internal/harvest/tickets.go
    - internal/harvest/tickets_test.go
    - internal/summarize/relationship.go
  modified:
    - internal/harvest/harvest.go
    - internal/harvest/engine.go
    - internal/harvest/opencode_sqlite.go
    - internal/harvest/opencode_source.go
    - internal/harvest/opencode_sqlite_test.go
    - internal/harvest/opencode_sqlite_scan_test.go
    - internal/summarize/pipeline.go
    - internal/summarize/harvest_adapter.go
    - internal/summarize/persist.go
    - internal/config/config.go
    - cmd/nano-brain/main.go
decisions:
  - "formatHeader early-return guard for Claude removed so Claude sessions get Branch/Cwd/SessionID in front-matter header"
  - "stripMarkdownHeadings applied to content before #\\d+ scan to prevent ATX headings (# Intro) from matching as ticket references"
  - "Children lookup deferred: metadata JSON does not store parent_id so a targeted query is not possible; Children stays empty gracefully (pipeline handles nil)"
  - "NewEngineWithTicketPatterns added for config-driven patterns; NewEngine uses defaults (nil) for backward compatibility"
  - "parent tag lookup in engine is best-effort: any error silently skips inheritance rather than failing the session"
  - "SqSession.ParentID added; listSessions uses COALESCE(s.parent_id,'') for graceful handling if older OpenCode builds omit the column"
metrics:
  duration: "~30 minutes"
  completed: "2026-06-29"
  tasks_completed: 3
  tasks_total: 3
  files_created: 3
  files_modified: 11
status: complete
---

# Phase 8 Plan 02: Ticket Extraction + Linking Summary

**One-liner:** Threaded Branch/Cwd/ParentID from NormalizedSession through SummaryMeta → SessionMetadata → rendered front-matter; implemented TicketExtractor (content+branch+parent inheritance, configurable patterns); wired DBRelationshipLookup so parent titles populate summary headers.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Thread Branch/Cwd/ParentID through SummaryMeta → SessionMetadata → front-matter | f369a6c | harvest.go, engine.go, opencode_sqlite.go, opencode_source.go, pipeline.go, harvest_adapter.go, test fixtures |
| 2 | TicketExtractor with configurable patterns + parent inheritance | 4cddea3 | tickets.go, tickets_test.go, config.go |
| 3 | Wire ticket tags into persist paths + DBRelationshipLookup | 3df45c9 | engine.go, persist.go, relationship.go, main.go |

## What Was Built

### Task 1 — Branch/Cwd/ParentID threading

- `SummaryMeta` gains `Branch string`, `Cwd string`, `Tags []string` fields
- `SessionMetadata` gains `Branch string`, `Cwd string`, `Tags []string` fields
- `formatHeader`: Claude early-return guard removed; all sources now emit `Session ID`, `Branch`, `Cwd` when non-empty (Claude sessions were previously truncated after `Source:`)
- `harvest_adapter.go`: copies `Branch`, `Cwd`, `Tags` from `SummaryMeta` → `SessionMetadata`
- `engine.go`: copies `Branch`, `Cwd` from `NormalizedSession` into `SummaryMeta`
- `SqSession` gains `ParentID string`; `listSessions` selects `COALESCE(s.parent_id, '')` in both query variants (filtered + unfiltered)
- `opencode_source.go`: populates `NormalizedSession.ParentID` from `SqSession.ParentID`
- Test fixtures: all SQLite test session tables updated to include `parent_id TEXT` column

### Task 2 — TicketExtractor

- `internal/harvest/tickets.go`:
  - `TicketExtractor` struct with compiled `[]*regexp.Regexp`
  - `NewTicketExtractor(nil)` → defaults `[A-Z][A-Z0-9]+-\d+` and `#\d+`
  - `Extract(content, branch, parentTags)`: scans content (headings stripped via `stripMarkdownHeadings` to prevent ATX `# Heading` matching `#\d+`), scans branch directly, inherits any `ticket:` prefixed parent tags; returns sorted+deduplicated bare IDs
  - `AsTags(tickets)` → `["ticket:DEV-4706", ...]`
- `internal/harvest/tickets_test.go`: 9 test cases
  - `TestExtract_ContentMatch`, `TestExtract_BranchMatch`, `TestExtract_ParentInheritance`, `TestExtract_SubagentNoContent`, `TestExtract_Deduplicate`, `TestExtract_CustomPattern`, `TestExtract_MarkdownHeadingsNotMatchedByHash`, `TestExtract_Empty`, `TestAsTags`, `TestNewTicketExtractor_InvalidPattern`
- `config.go`: `ClaudeCodeHarvesterConfig.TicketPatterns []string` (`yaml:"ticket_patterns"`) — zero value uses defaults

### Task 3 — Ticket wiring + RelationshipLookup

- `engine.go`:
  - `Engine` gains `ticketExtractor *TicketExtractor` field
  - `NewEngine`: initializes with `NewTicketExtractor(nil)` (defaults)
  - `NewEngineWithTicketPatterns`: accepts `[]string` patterns for config-driven use
  - `HarvestAll`: before summarize/raw-fallback, looks up parent doc tags via `GetDocumentBySourcePath` (best-effort, non-fatal); extracts tickets via `ticketExtractor.Extract(md, sess.Branch, parentTags)`; passes `ticketTags` into `SummaryMeta.Tags` and `writeRawFallback`
  - `writeRawFallback`: signature gains `ticketTags []string`; merges them into document `Tags` alongside base tags
- `persist.go`: `Persister.Save` merges `meta.Tags` into document tags so `ticket:ID` appears on summarized documents
- `relationship.go`: `DBRelationshipLookup` implements `RelationshipLookup`; resolves parent title by querying `GetDocumentBySourcePath` for `summary://<source>/<parentID>`; strips `"Summary: "` prefix; children deferred (non-fatal empty)
- `main.go`: `NewDBRelationshipLookup(db, logger)` wired into `NewPipeline` replacing the previous `nil`

## Ticket Extraction Sources

| Source | Example | Mechanism |
|--------|---------|-----------|
| Content | `"Working on DEV-1234"` | regex over stripped content |
| Branch | `feat/DEV-4706-my-feature` | regex over branch name |
| Parent tags | `ticket:PROJ-42` on parent doc | tag prefix inheritance |

## Verification

```
CGO_ENABLED=0 go build ./...  → PASS
go test -race -short ./...    → 28 packages PASS, 0 FAIL
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Test fixture SQLite schemas missing `parent_id` column**
- **Found during:** Task 1 (harness gate triggered)
- **Issue:** `listSessions` query now selects `COALESCE(s.parent_id, '')` but test fixture `CREATE TABLE session` statements did not include the column, causing `no such column: s.parent_id` errors in 6 tests
- **Fix:** Added `parent_id TEXT` to all 4 session table definitions across `opencode_sqlite_test.go` and `opencode_sqlite_scan_test.go` (3 distinct schemas in the scan test)
- **Files modified:** `internal/harvest/opencode_sqlite_test.go`, `internal/harvest/opencode_sqlite_scan_test.go`
- **Commit:** f369a6c

**2. [Rule 1 - Bug] Claude front-matter truncated by early-return guard**
- **Found during:** Task 1 review of `formatHeader`
- **Issue:** `if meta.Source == SourceClaude { return b.String() }` at line 230 caused Claude sessions to emit only `Date:` and `Source:` in the header — no `Session ID`, `Branch`, `Cwd`, or relationship fields
- **Fix:** Removed the early return; all sources now emit the full header. This is the correct behavior since Branch/Cwd are the new value-add for Claude sessions specifically
- **Files modified:** `internal/summarize/pipeline.go`
- **Commit:** f369a6c

## Known Stubs

- **Children lookup deferred**: `DBRelationshipLookup` always returns empty `Children`. The metadata JSON stored by `Persister.Save` does not include `parent_id`, making a targeted child query impossible without a full-table scan. First iteration accepts this; `pipeline.go:252` handles nil Children gracefully. Tracked for future plan.
- `NewEngineWithTicketPatterns` is not yet called from `main.go` — the Engine constructed in `buildClaudeHarvester`/`buildOpenCodeHarvester` still uses `NewEngine` (default patterns). Config-driven patterns will be wired when those builders are refactored to use the Engine path (Phase 8 follow-up per 08-01 deferral note).

## Threat Flags

None — no new network endpoints, auth paths, file access patterns, or schema changes. Regex patterns are compiled at startup from config; invalid patterns return an error (not a panic). Parent doc lookup is read-only against the existing documents table.

## Self-Check: PASSED

Files created:
- `internal/harvest/tickets.go` — FOUND
- `internal/harvest/tickets_test.go` — FOUND
- `internal/summarize/relationship.go` — FOUND

Commits verified in git log:
- f369a6c — FOUND (Task 1)
- 4cddea3 — FOUND (Task 2)
- 3df45c9 — FOUND (Task 3)

Build: `CGO_ENABLED=0 go build ./...` — clean.
Tests: `go test -race -short ./...` — all 28 packages PASS, 0 FAIL.
