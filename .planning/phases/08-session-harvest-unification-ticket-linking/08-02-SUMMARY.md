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

## Post-Review Fixes (commit 53c0e88)

Independent review APPROVED 08-02 (subagent inheritance + branch extraction work and are tested). Three findings raised; two fixed, one is a no-op against current code state:

**1. [Rule 1 - Bug] Ticket regex matched technical strings (UTF-8, SHA-256, etc.)**
- **Issue:** Default JIRA pattern `[A-Z][A-Z0-9]+-\d+` had no word boundaries, so `UTF-8`, `SHA-1`, `SHA-256`, `ISO-8601`, `RFC-2616`, `CVE-2024-12345` all matched as tickets — polluting tags on nearly every technical session and degrading the 08-03 ticket query.
- **Fix:** Pattern is now `\b[A-Z][A-Z0-9]+-\d+\b` (Go RE2 ASCII word boundaries). Added `nonTicketPrefixes` denylist (UTF, UTF8, UTF16, SHA, MD5, ISO, RFC, TLS, SSL, HTTP, HTTPS, CVE, BASE64, IPV4, IPV6, X86, ARM64) with `isNonTicket()` filter in `Extract` as a second line of defense. The `#NN` GitHub form has no prefix and is unaffected.
- **Tests added:** `TestExtract_NonTicketTechnicalStrings` (asserts none of UTF-8/SHA-1/SHA-256/ISO-8601/RFC-2616/TLS-1.3/CVE-2024-12345 produce tickets), `TestExtract_RealTicketAmongTechnicalStrings` (DEV-4706/PROJ-42 still extracted when mixed with UTF-8/SHA-256), `TestExtract_WordBoundaryNoSubstringMatch` (embedded `XDEV-100Z` rejected).
- **Files modified:** `internal/harvest/tickets.go`, `internal/harvest/tickets_test.go`
- **Commit:** 53c0e88

**2. [Rule 1 - Bug] Dead `stripHeadings bool` param in `scanText` closure**
- **Issue:** `scanText` had a `stripHeadings bool` parameter that was always called with `false`, making the `if stripHeadings` branch unreachable and the param misleading.
- **Fix:** Removed the dead closure; replaced with a plain `addMatches(src string)` helper that scans patterns and applies the denylist. Content is passed pre-stripped (`stripMarkdownHeadings(content)`); branch is passed raw. No behavior change for valid tickets.
- **Files modified:** `internal/harvest/tickets.go`
- **Commit:** 53c0e88

**3. [Not applicable in current code state] Wire `TicketPatterns` via `NewEngineWithTicketPatterns`**
- **Finding:** Reviewer asked to wire `main.go` builders from `NewEngine` (defaults) to `NewEngineWithTicketPatterns` so `ClaudeCodeHarvesterConfig.TicketPatterns` takes effect.
- **Investigation:** Verified `harvest.NewEngine` has **zero** production call sites (`grep` across `cmd/` and `internal/`, excluding tests/engine.go, returns nothing). The Claude harvester is built via `initClaudeCodeHarvesters` → `harvest.NewClaudeCodeHarvester` (legacy path); OpenCode via `buildOpenCodeHarvesters`. The `Engine` (which owns `ticketExtractor`) is the deferred "engine flag path" from 08-01 (08-01 SUMMARY Known Stubs: "Engine is built and tested but not yet wired to Runner"). There is no `NewEngine` call to convert.
- **Decision:** Not implemented. Wiring the Engine into the Runner is the explicitly-deferred Phase 8 follow-up, out of 08-02 scope and not a smallest-diff fix. The constructor side is already correct: `NewEngineWithTicketPatterns(source, db, summarizer, cfg.Harvester.ClaudeCode.TicketPatterns, logger)` is ready for whoever wires the Engine. Until then, `TicketPatterns` config is inert by design (consistent with the whole Engine path being inert). No code change.

**Children deferral:** Left empty as the review confirmed (documented deferral — fine).

**Verification:** `CGO_ENABLED=0 go build ./...` clean; `go test -race -short ./...` all packages PASS, 0 FAIL. The false-positive denylist/boundary tests pass.

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

---

## Addendum: Idempotent Startup Backfill for Session Ticket Tags (commit dec9dfe)

**Why:** Session docs harvested before phase 8 have no `ticket:` tags. The harvester
dedup-skips them (content hash unchanged) so they are never re-processed by the
normal harvest cycle. Fresh installs are unaffected — their first harvest already
applies the extractor. This backfill reconciles only upgrade scenarios.

### What was added

**`internal/harvest/migration.go` — `BackfillSessionTicketTags`**

- Selects `collection = 'sessions'` docs in batches of 500 (LIMIT/OFFSET keyset
  on `id`); filters in Go for docs without any `ticket:` tag — so re-runs are a
  true no-op (tagged=0).
- Calls `extractBranchFromContent` to parse `Branch:` from the rendered
  front-matter (best-effort; falls back to `""`).
- Runs `extractor.Extract(content, branch, nil)` and appends `ticket:ID` tags via
  `appendTicketTags` (deduplicates, preserves existing tag order).
- Updates each affected row with `UPDATE documents SET tags = $1 WHERE id = $2`.
- Logs `{"tagged": N}` on completion.

**`internal/harvest/migration_test.go`** — four unit tests (in-process SQLite):

| Test | Scenario |
|------|----------|
| `TagsUntaggedDoc` | content mentions DEV-1234 → tag added |
| `Idempotent` | doc already has `ticket:DEV-1234` → tagged=0, tags unchanged |
| `NonSessionSkipped` | `collection='memory'` doc untouched |
| `ExistingTagsPreserved` | pre-existing `bug-fix,feature` tags kept; `ticket:PROJ-55` appended |

**`cmd/nano-brain/main.go`** — wired immediately after `MigrateSessionSummaryToSessions`:

```go
if ticketExtractor, teErr := harvest.NewTicketExtractor(nil); teErr != nil {
    logger.Warn().Err(teErr).Msg("ticket extractor init failed — skipping session ticket tag backfill")
} else if backfillN, backfillErr := harvest.BackfillSessionTicketTags(ctx, db, ticketExtractor, logger); backfillErr != nil {
    logger.Warn().Err(backfillErr).Msg("session ticket tag backfill failed — continuing without backfill")
} else if backfillN > 0 {
    logger.Info().Int("tagged", backfillN).Msg("session ticket tag backfill complete")
}
```

Non-fatal in both error cases. Zero-tagged runs produce no log output.

### Verification

```
CGO_ENABLED=0 go build ./...     → PASS
go test -race -short ./...       → 28 packages PASS (4 new backfill tests in harvest)
harness pre-commit gate          → 4/4 PASS
```
