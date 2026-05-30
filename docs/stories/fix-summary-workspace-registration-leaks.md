---
github_issue: nano-step/nano-brain#238
openspec_change: openspec/changes/fix-summary-workspace-registration-leaks
lane: high-risk
change_type: bug-fix
risk_flags:
  - data-model
  - audit-security
  - existing-behavior
  - weak-proof
hard_gates:
  - data-model
  - audit-security
branch: feat/238-fix-summary-workspace-leaks
worktree: .opencode/worktrees/fix-summary-workspace-leaks-238/fix-summary-workspace-leaks-238
---

# US-238 Fix Summary Workspace-Registration Leaks

## Status

in-progress

## GitHub Issue

`nano-step/nano-brain#238` — created at Feature Intake step 0.

## Lane

**high-risk** — 4 risk flags + 2 hard gates (data-model + audit-security).

## OpenSpec Change

`openspec/changes/fix-summary-workspace-registration-leaks/`

Artifacts:
- `proposal.md` — intent, constraints, 12 acceptance criteria
- `design.md` — 5 ADRs, 5-layer defense-in-depth, component changes, migration strategy, rollback plan
- `tasks.md` — 11 phases (A–K), ~765 LoC, ~16-18h estimate
- `specs/harvester-summary-only/spec.md` — MODIFIED requirements (5 scenarios)
- `specs/workspace-registration-guard/spec.md` — ADDED 4 requirements (15 scenarios)

`openspec validate fix-summary-workspace-registration-leaks --strict` → PASS

## Product Contract

When `summarization.enabled: true`, it MUST be impossible to create a `documents` row under a `workspace_hash` that does not exist in the `workspaces` table, regardless of the write path used (HTTP, MCP, harvester, internal). Enforcement at 5 layers (defense-in-depth):

1. HTTP middleware (write endpoints)
2. MCP tool handlers (`memory_write`, `memory_update`)
3. Harvester init + per-session check (OpenCode + Claude Code)
4. Persister.Save (defense-in-depth for summary feature)
5. PostgreSQL FK constraint (last line of defense)

## Relevant Product Docs

- `ai/test-case/rri-t/summary/` — full RRI-T test cycle (40 test cases, 5/6 leak points confirmed)
- `README.md` § Session Summarization
- `docs/HARNESS.md` § High-Risk lane requirements
- `openspec/specs/harvester-summary-only/spec.md` — current behavior contract
- `openspec/specs/workspace-config-guard/spec.md` — related workspace-registration patterns

## Acceptance Criteria

Copied from `openspec/changes/.../proposal.md` § Acceptance Criteria:

1. **Persister rejects unregistered workspace_hash** — `Persister.Save()` returns error with `workspace_not_registered`; unit + integration tests assert.
2. **OpenCode harvester skips orphan sessions AND removes auto-registration** — no UpsertWorkspace fallback; per-session registered-check; tests assert.
3. **Claude Code harvester refuses unregistered session_dir** — init logic extracted to testable function; WARN log on unregistered; tests assert.
4. **HTTP middleware rejects unregistered workspace** — new `workspaceRegisteredMiddleware` applied to 5 write endpoints; rejects "all" with `workspace_all_not_supported`; tests assert HTTP 400.
5. **MCP tool handlers reject unregistered workspace** — `memory_write` + `memory_update` validate before UpsertDocument; tests assert tool result error.
6. **FK constraint enforced** — migration 00011 adds FK on documents/chunks; INSERT, UPDATE, cascade tests pass; down migration preserves data.
7. **Pre-migration cleanup** — `cleanup-orphan-workspaces` CLI with --dry-run; reports docs + chunks + transitively-deleted embeddings; pre-flight health check warns if server running.
8. **No regression** — existing test suite passes unchanged.
9. **User-flow test (non-LLM)** — HTTP write, MCP write, OpenCode orphan, Claude Code unregistered session_dir, direct SQL orphan: all 5 paths rejected. Evidence in `docs/evidence/`.
10. **Validate ladder** — validate:quick + test:integration + smoke:e2e green.
11. **Review Gate** — review-work skill 5 parallel sub-agents all PASS.
12. **Release notes** — upgrade sequence, breaking change, HTTP status change.

## Design Notes

- **Commands:** new `nano-brain cleanup-orphan-workspaces [--dry-run]`
- **Queries:** `GetWorkspaceByHash` (already exists in sqlc); new SQL queries `CountOrphanDocumentsByWorkspace`, `DeleteOrphanDocuments`, `DeleteOrphanChunks`
- **API:** HTTP 5 write endpoints add 400 response (`error: workspace_not_registered` / `workspace_all_not_supported` / `workspace_lookup_failed`). MCP `memory_write` + `memory_update` add tool result errors.
- **Tables:** `documents` + `chunks` get FK constraints to `workspaces(hash) ON DELETE CASCADE`. No schema additions, only constraint additions.
- **Domain rules:** Workspaces must be pre-registered via `POST /api/v1/init` or `nano-brain init`. Harvester no longer silently auto-registers worktrees.
- **UI surfaces:** N/A (no UI in nano-brain)

## Validation

| Layer | Expected proof |
| --- | --- |
| Unit | `go test ./internal/summarize ./internal/server ./internal/mcp ./internal/harvest ./cmd/nano-brain -race -short` |
| Integration | `go test -race -tags=integration ./...` including new `persist_integration_test.go` + FK constraint test |
| E2E | Build binary on port 8899 → curl write/embed/reindex/summarize with registered + unregistered hashes; MCP memory_write via JSON-RPC; capture all outputs |
| Platform | `nano-brain status` after migration applied → healthy |
| Release | Migration 00011 applies cleanly on production DB after cleanup; existing harvest flow unaffected for registered workspaces |

## Change Type

`bug-fix` — fixes 7 leak points (6 confirmed by RRI-T + 1 MCP path found in deep-design). Per `docs/HARNESS.md`:
- E2E gate: **required** for bug-fix
- Review gate: **required** for bug-fix

Also touches `index-schema` (migration 00011) — secondary label.

## Testing Checklist

- [ ] User-flow test covers primary changed behavior (file: `docs/evidence/fix-summary-workspace-registration-leaks/g2-http-unregistered-rejected.txt` etc.)
- [ ] Error/edge path tested — high-risk required:
  - [ ] HTTP write with `workspace: "all"` rejected
  - [ ] HTTP write with unregistered hash rejected
  - [ ] MCP memory_write with unregistered hash rejected
  - [ ] OpenCode harvester with orphan session skipped
  - [ ] Claude Code harvester with unregistered session_dir skipped
  - [ ] Direct SQL INSERT with orphan workspace rejected (FK)
  - [ ] Direct SQL UPDATE to orphan workspace rejected (FK)
  - [ ] Workspace DELETE cascades to documents + chunks + embeddings
- [ ] E2E applies (bug-fix → required) — non-LLM path used to avoid LLM cost (Phase G)
- [ ] All listed tests pass (output pasted in Evidence)

## Review

- Reviewer agent: `review-work` skill (5 parallel sub-agents)
- Reviewer ≠ implementer: yes (review-work spawns fresh agents)
- Verdict: `PENDING`
- Date: TBD
- Commit: TBD

| Acceptance Criterion | Evidence | Status |
| --- | --- | --- |
| AC1 — Persister rejects unregistered | TBD (Phase B) | ☐ |
| AC2 — OpenCode skip + remove UpsertWorkspace | TBD (Phase C) | ☐ |
| AC3 — Claude Code refuses unregistered | TBD (Phase C) | ☐ |
| AC4 — HTTP middleware rejects | TBD (Phase D) | ☐ |
| AC5 — MCP tool handlers reject | TBD (Phase D') | ☐ |
| AC6 — FK constraint enforced | TBD (Phase F) | ☐ |
| AC7 — Cleanup command | TBD (Phase E) | ☐ |
| AC8 — No regression | TBD (Phase I) | ☐ |
| AC9 — User-flow test | TBD (Phase G) | ☐ |
| AC10 — Validate ladder | TBD (Phase I) | ☐ |
| AC11 — Review Gate | TBD (post-impl) | ☐ |
| AC12 — Release notes | TBD (Phase K) | ☐ |

## PR Bot Review

- PR URL: TBD
- Bot rounds: 0 (max 3 before human escalation)
- Outstanding comments: TBD
- Bot approved: TBD

## Harness Delta

None — this story follows the existing HIGH-RISK lane process without harness rule changes.

## Evidence

Will be populated in `docs/evidence/fix-summary-workspace-registration-leaks/` during Phase G + Phase I.

Pre-implementation evidence:
- RRI-T test cycle at `ai/test-case/rri-t/summary/` (NO-GO verdict, 5/6 leaks confirmed)
- Deep-design synthesis: Metis + Oracle + explore on MCP path (commit `e349a38`)
- Human approval logged on `nano-step/nano-brain#238` on 2026-05-30
