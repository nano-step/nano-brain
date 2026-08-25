# Self-Review — Pi CLI Agent Session Harvesting

Branch: `feat/pi-session-harvesting`. OpenSpec change: `openspec/changes/pi-session-harvesting/`.

This feature went through two full review passes before this PR, both using independently-spawned adversarial subagents (never self-approved by the implementing agent, per R88).

## Pass 1 — proposal review (before any code was written)

Ran via the `openspec-review-workflow` skill: explore → proposal/design/specs/tasks → two independent reviewers (Reviewer A: architecture adversary, Reviewer B: delivery adversary) → cross-reviewer debate (each received the other's raw findings and returned `accept`/`rebut`/`amend` verdicts).

Result: 15 findings (3 HIGH, 4 MEDIUM, ~8 LOW/confirmed-fine), **zero rebuttals** — both reviewers agreed on every finding. Key catches, all resolved in `design.md`/`spec.md`/`tasks.md` before implementation started:

- The original schema draft only handled `text` content blocks; verified against all 120 real session files on this machine, the actual schema also has `toolResult` (the single largest role), `toolCall`, `thinking`, and `image` — a literal implementation would have dropped ~half of real content.
- `pipeline.go`'s strip-function dispatch had no `SourcePi` case (would silently route Pi content through `StripOpenCode`).
- `sourceFromTags` (manual re-summarize path) had no `"pi"` branch (would default to `"opencode"`, causing duplicate documents) — the debate step traced this to the *same root cause* as the pipeline.go finding, but requiring a *separate* fix at a separate call site.
- The malformed-line handling spec ("skip whole session") contradicted `ClaudeCodeHarvester`'s actual precedent (skip one line, keep parsing).
- Several proposal claims (a `~`-expansion helper, `config.example.yml`, the directory-encoding formula) didn't match what the codebase actually does — corrected against source before any code was written.

## Pass 2 — code review (after implementation, against the actual diff)

Five independent subagents reviewed the real diff: reuse, simplification, efficiency, altitude, correctness. See `openspec/changes/pi-session-harvesting/tasks.md` Group 7 for the full list. Findings fixed:

- **Correctness (HIGH):** `internal/storage/sourcepath.go`'s `SourceFromPath` (backs the `memory_ticket` MCP tool, no tag fallback) had no Pi case — fixed, test added.
- **Correctness (MEDIUM):** `cmd/nano-brain/cmd_backfill_summaries.go`'s `extractBackfillSource` had no Pi case (misfiles Pi docs to the wrong disk directory) — fixed, test added.
- **Correctness (spec overclaim, HIGH):** the proposal's dedup requirement claimed content-hash-based re-ingestion on session growth; actual behavior (inherited from `ClaudeCodeHarvester`) is presence-based only. Corrected `spec.md`/`design.md`/`tasks.md` to state the true, shared, out-of-scope-to-fix-here behavior instead of the false stronger claim, rather than leaving the discrepancy undocumented.
- **Reuse + Altitude (2 reviewers converged independently):** `PiHarvester.writeRawFallback` was a third near-verbatim copy of the tx-based persist skeleton already duplicated between `ClaudeCodeHarvester`/`OpenCodeSQLiteHarvester`. Extracted `persistRawFallback` (`internal/harvest/raw_fallback.go`); the other two harvesters are left unchanged (out of scope, no risk to already-shipped code).
- **Simplification:** collapsed a byte-for-byte-identical `"user"`/`"assistant"` case pair in `renderPiMarkdown`.
- **Efficiency:** no issues — `PiHarvester`'s header-then-scan I/O pattern is the minimal cost given the format constraint (session ID lives in content, not filename).

## Verification evidence

- `go build ./...` — clean.
- `go test -race -short ./...` — full repo, all packages `ok`.
- `go test -race -tags=integration ./internal/harvest/... ./cmd/nano-brain/... ./internal/storage/... ./internal/summarize/... ./internal/server/handlers/...` against the real `nanobrain_test` Postgres (never the dev DB) — all `ok`, including `TestPiHarvester_PresenceBasedSkip` (real dedup against Postgres) and `TestInitPiHarvesters_*` (real workspace matching).
- `openspec validate pi-session-harvesting --strict` — valid.

No unresolved critical/major findings from either review pass.
