# Phase 8 Research & Design: Session Harvest Unification & Ticket Linking

**Status:** Design approved by user (brainstorming, 2026-06-29). This doc is the authoritative spec; the planner should turn it into plans without re-researching.
**Requirement:** REQ-CI-05

---

## Problem

1. Each session source (OpenCode, Claude Code) is a self-contained harvester duplicating discovery/parse/dedup/persist. Adding a new agent (Codex, Cursor, Gemini) means rewriting most of it.
2. Sessions land in TWO collections: OpenCode raw → `sessions`, summarized → `session-summary`. `memory_wake_up` reports only `sessions`, so claude/summarized sessions look missing (cosmetic but confusing).
3. No way to see all work on a ticket across sources/repos. Subagent sessions often lack the ticket id in their content.

## Current Architecture (verified via code map, file:line)

- **Harvester interface** — `internal/harvest/runner.go:15-17`: only `HarvestAll(ctx, enqueuer) (harvested, skipped, errCount int)`. Optional `summarizerSettable.setSummarizer` (runner.go:20-22).
- **Runner** — `runner.go:36-68`: `NewRunner`, `AddHarvester` (propagates summarizer), `WithSummarizer`, `WithPublisher`, `Run` (immediate tick + interval), `RunOnce` (mutex-guarded). Iterates harvesters sequentially.
- **Claude harvester** — `internal/harvest/claudecode.go`: struct `ClaudeCodeHarvester{db, logger, sessionDir, workspace, summarizer}` (34-40). Parse `parseJSONLFile` (268-298) → private `claudeCodeMessage`. Render `renderClaudeCodeMarkdown` (302-376). Discovery in `cmd/nano-brain/claudecode_init.go:51-98` (`initClaudeCodeHarvesters`, fixed this branch).
- **OpenCode SQLite harvester** — `internal/harvest/opencode_sqlite.go`: struct (24-30). Discovery `ScanOpenCodeDBRoot` (577-638) matches `project.worktree` to registered workspaces. Parse `listSessions`/`listMessages` (288-419) → `SqSession{ID,Title,CreatedAt,UpdatedAt,Worktree}` (266-272), `sqMessage{role,content,createdAt}`. Skips active sessions <10min (274).
- **OpenCode JSON harvester** (legacy) — `internal/harvest/opencode.go`: always writes `Collection:"sessions"` (143), no summarizer.
- **Shared boundary type** — `SummaryMeta` `internal/harvest/harvest.go:10-20`: `Source, SessionID, Title, Agent, ProjectPath, CreatedAt, Duration, ParentID, WorkspaceHash`. **Gaps: `ParentID`, `Agent`, `ProjectPath`, `Duration` NEVER populated; NO `git_branch` field anywhere.**
- **NO normalized session/message model** — each harvester has private parse types.
- **Collections:** `sessions` = raw/fallback (writers: claudecode.go:196, opencode.go:143, opencode_sqlite.go:484). `session-summary` = summarized (sole writer `internal/summarize/persist.go:118` + link block :164). The plural `session-summaries` does NOT exist in code (stale DB rows only).
- **source_path switch** — `persist.go:211-218` `buildSourcePath`: `claude`→`summary://claude/`, else→`summary://opencode/`. New source falls through wrongly — needs a case.
- **LATENT relationship support (reuse this!):** `internal/summarize/pipeline.go:34-45` `SessionMetadata` already has `ParentID, ParentTitle, Children []RelatedSession, Siblings []RelatedSession`, populated by a `RelationshipLookup` — but wired `nil` at `main.go:796`. Parent/child linking is half-built.
- **Tag extraction today:** `InferSemanticTags` (tags.go, legacy-only, keyword tags bug-fix/feature/...); `AutoMemoryExtractor` (automemory.go, decisions/lessons → `memory` collection). Neither extracts tickets/branch.
- **Data available for linking (confirmed in raw sources):** OpenCode `session` table has `parent_id` (+ index). Claude JSONL has per-record `gitBranch`, `cwd`, `sessionId`, `isSidechain` (no cross-session parent).

## Target Design (approved)

### Components
1. **`SessionSource` adapter interface** (new pluggable seam): `Name()`, `Discover(registeredWorkspaces) []Location`, `Read(Location) []NormalizedSession`. Each agent = one adapter. OpenCode + Claude refactored into adapters.
2. **Normalized model** (new shared types): `Session{Source, SessionID, ParentID, WorkspaceHash, Branch, Cwd, Title, CreatedAt, Messages}` and `Message{Role, Content, Timestamp, ToolName, IsSidechain}`. Every adapter produces these.
3. **Generic harvest engine** (shared, replaces per-source duplication): discover → read → normalize → content-hash dedup → skip-active → render → summarize/persist → raw fallback. One implementation reused by all sources.
4. **Unified collection:** canonical = `sessions`. Make the summarizer Persister write to `sessions` (not `session-summary`); migrate existing `session-summary` docs → `sessions`; `buildSourcePath` keeps per-source prefix (`summary://<source>/`). `memory_wake_up` then reports correct counts.
5. **Ticket extraction + linking** (new shared step over normalized Session): ticket set = regex over content (`[A-Z]+-\d+`, `#\d+`) ∪ branch-derived ∪ inherited from `ParentID`. Store as tags (`ticket:DEV-4706`). Wire the latent `RelationshipLookup` so parent/children populate. Add `Branch`+`Cwd`+`ParentID` to `SummaryMeta`/`SessionMetadata`/front-matter (use `internal/harvest/git.go` for branch where source lacks it).
6. **Cross-workspace ticket query** (new): a tool/endpoint returning all sessions tagged with a ticket across ALL workspaces (e.g. `memory_ticket(ticket)` or a `ticket` filter + cross-workspace mode on memory_search). Ticket regex patterns configurable (DEV-/#/JIRA ABC-123...).

### Plan decomposition (3 plans)
- **08-01 — Pluggable refactor + unify collection:** introduce `SessionSource` interface + normalized model; refactor OpenCode + Claude into adapters behind the generic engine; unify to `sessions` collection + migrate `session-summary`; add `case` per source in buildSourcePath. No behavior loss; wake_up counts correct. Tests green.
- **08-02 — Ticket extraction + linking:** add Branch/Cwd/ParentID through SummaryMeta→pipeline→front-matter; implement ticket extraction (content+branch+parent inheritance) → tags; wire RelationshipLookup (parent/children). Configurable ticket patterns.
- **08-03 — Cross-workspace ticket query:** tool/endpoint to fetch all sessions for a ticket across workspaces/sources; MCP tool exposure.
- *(Follow-up, not in this phase):* a Codex adapter to prove extensibility.

### Constraints
- Smallest-diff, follow repo patterns (constructor injection, no over-engineering). No new deps.
- No regression: existing OpenCode/Claude harvest keeps working; `go test -race -short ./...` passes.
- Migration of `session-summary`→`sessions` must be idempotent and safe (read-modify-write or SQL update by collection).

### Verification
- Unit tests per new unit (interface, normalized model, ticket extractor with subagent/parent cases).
- Live: enable harvest, confirm `memory_wake_up(zengamingx).sessions > 0` (after unify) and `memory_ticket("DEV-XXXX")` returns sessions from >1 source/repo.
