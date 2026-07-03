---
phase: 13-interactive-init-wizard-one-command-interactive-setup-detect
plan: 02
subsystem: health
tags: [doctor, embedding, config, go]

# Dependency graph
requires:
  - phase: 13-01
    provides: D-11 BM25-only config write (embedding.provider "")
provides:
  - "doctor CheckEmbeddingProvider/CheckEmbeddingModel skip guards for disabled embeddings"
affects: [13-06 wizard serve step]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "doctor Check{Status: 'skip'} early-return guard pattern for intentional non-configuration"

key-files:
  created: []
  modified:
    - internal/health/doctor/doctor.go
    - internal/health/doctor/doctor_test.go

key-decisions:
  - "RED and GREEN committed together in a single feat commit — repo's pre-commit hook (scripts/harness-check.sh in-progress) runs the full go test -short suite, so a pure-RED-only commit cannot land standalone (same pattern as Phase 10-01, logged in STATE.md)"
  - "TestCheckEmbeddingModel_Disabled asserts Detail (not just Status) to distinguish the new Provider==\"\" guard from the pre-existing ollamaBody==nil skip path, which also returns skip but with a different Detail (model name) — without the Detail assertion the RED test would pass immediately (fail-fast violation)"

requirements-completed: [D-13]

coverage:
  - id: D1
    description: "CheckEmbeddingProvider returns skip/'disabled — BM25-only' when Provider == \"\", performing zero HTTP calls, before the existing ollama fallback"
    requirement: "D-13"
    verification:
      - kind: unit
        ref: "internal/health/doctor/doctor_test.go#TestCheckEmbeddingProvider_Disabled"
        status: pass
    human_judgment: false
  - id: D2
    description: "CheckEmbeddingModel returns skip/'disabled — BM25-only' when Provider == \"\""
    requirement: "D-13"
    verification:
      - kind: unit
        ref: "internal/health/doctor/doctor_test.go#TestCheckEmbeddingModel_Disabled"
        status: pass
    human_judgment: false
  - id: D3
    description: "Configured-provider path (voyage) is unchanged — regression guard"
    verification:
      - kind: unit
        ref: "internal/health/doctor/doctor_test.go#TestCheckEmbeddingProvider_VoyageConfigured"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-07-02
status: complete
---

# Phase 13 Plan 02: Doctor Embedding-Disabled Skip Guards Summary

**Added leading `Provider == ""` skip guards to `CheckEmbeddingProvider`/`CheckEmbeddingModel` so a BM25-only config (D-11) reports doctor "skip" instead of a false FAIL against a nonexistent Ollama.**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-07-02T14:44:00Z
- **Completed:** 2026-07-02T14:56:30Z
- **Tasks:** 2 (RED test + GREEN implementation, committed together)
- **Files modified:** 2

## Accomplishments
- `CheckEmbeddingProvider(config.EmbeddingConfig{Provider: ""})` now returns `Check{Status: "skip", Detail: "disabled — BM25-only"}` and a nil `ollamaBody`, with zero network egress
- `CheckEmbeddingModel(config.EmbeddingConfig{Provider: ""}, nil)` now returns `Check{Status: "skip", Detail: "disabled — BM25-only"}`
- Configured-provider path (`voyage` + API key) proven byte-for-byte unchanged via a new regression test
- Full `internal/health/doctor` suite green, `go build ./...` succeeds

## Task Commits

Both tasks landed in a single commit because the repo's pre-commit hook runs the full test suite (see Deviations below):

1. **Task 1 (RED) + Task 2 (GREEN): doctor skip embedding checks when provider disabled** - `504139e` (feat)

**Plan metadata:** (this commit)

_Note: RED test commit alone would fail the repo's `go test -short` pre-commit gate, so RED+GREEN are combined per the Phase 10-01 precedent documented in STATE.md._

## Files Created/Modified
- `internal/health/doctor/doctor.go` - Added `Provider == ""` skip guard as the first statement in `CheckEmbeddingProvider` (before the voyage branch and before the `cfg.Provider = "ollama"` fallback) and in `CheckEmbeddingModel`. No other lines changed.
- `internal/health/doctor/doctor_test.go` - Added `TestCheckEmbeddingProvider_Disabled`, `TestCheckEmbeddingModel_Disabled`, `TestCheckEmbeddingProvider_VoyageConfigured`.

## Decisions Made
- RED test verified as genuinely failing pre-implementation (captured in terminal output: `status = "ok", want skip` and a live HTTP round-trip to a real local Ollama instance) before writing the GREEN guards — satisfies the TDD fail-fast rule.
- `TestCheckEmbeddingModel_Disabled` was strengthened to assert `Detail` in addition to `Status`, because the pre-existing `ollamaBody == nil` code path already returns `Status: "skip"` for an unrelated reason (missing Ollama response body) — without the Detail check, the RED test would have passed immediately against unmodified code, silently skipping the RED gate for that one test. This was caught during the RED run and fixed before proceeding to GREEN.
- RED and GREEN committed together in one `feat` commit rather than as separate `test`/`feat` commits — the repo's `.git/hooks/pre-commit` invokes `scripts/harness-check.sh in-progress`, which runs `go test -race -short ./...` as part of its validation ladder. A commit containing only the RED test would fail that gate. This mirrors the documented Phase 10-01/09-01 precedent for this exact repo constraint.

## Deviations from Plan

None of Rules 1-4 triggered. One process deviation from the plan's literal two-task/two-commit structure, documented above: RED+GREEN combined into a single commit due to the repository's pre-commit test-gate hook (not a plan defect — the plan itself anticipated this: "commit RED per project hook rule — stage with a minimal change or stash-round-trip as needed").

## Issues Encountered
- Initial `TestCheckEmbeddingModel_Disabled` draft asserted only `Status == "skip"` and passed against unmodified code (false RED-phase pass), because the pre-existing `ollamaBody == nil` skip path shares the same Status value for a different reason. Fixed by adding a `Detail` assertion before proceeding to GREEN, per the TDD fail-fast rule in the executor's TDD guidance.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Plan 06 (wizard serve step) can now rely on `doctor.RunAll` emitting `skip` (not `fail`) for embedding checks when `embedding.provider` is empty, per D-14.
- No blockers for downstream plans in this phase.

## TDD Gate Compliance

RED gate: verified via terminal output showing both new tests failing pre-implementation (`TestCheckEmbeddingProvider_Disabled`: `status = "ok", want skip`; `TestCheckEmbeddingModel_Disabled`: `detail = "nomic-embed-text", want "disabled — BM25-only"`). GREEN gate: verified via full `go test -race -short ./internal/health/doctor/...` passing after the guards were added. Per-commit separation of `test(...)` then `feat(...)` was not possible due to the repo's pre-commit test gate (see Deviations) — both phases are captured in a single `feat` commit, with the RED terminal evidence recorded here in lieu of a separate commit artifact.

## Self-Check: PASSED

- FOUND: internal/health/doctor/doctor.go
- FOUND: internal/health/doctor/doctor_test.go
- FOUND: commit 504139e (git log --oneline --all | grep 504139e)

---
*Phase: 13-interactive-init-wizard-one-command-interactive-setup-detect*
*Completed: 2026-07-02*
