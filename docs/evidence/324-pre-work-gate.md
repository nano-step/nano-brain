# PRE-WORK gate — Issue #324

**Date**: 2026-06-02
**Issue**: #324 (embed_permanently_failed CHECK constraint mismatch + dead MarkChunkEmbedPermanentlyFailed)
**Lane**: tiny (reclassified from no-lane / change-type:infrastructure)
**Change-type**: refactor (reclassified — this is pure dead-code removal, no infrastructure)
**Branch**: `refactor/324-remove-dead-permanently-failed`

## Gate output

```
[FAIL] 1.1 Open PRs still pending (1)
[PASS] 1.2 No active OpenSpec changes
[PASS] 1.3 Issue #324 exists (state: OPEN)
[PASS] 1.4 master is up-to-date (at d55b566)
[PASS] 1.5 Validation ladder passes
[PASS] 1.6 Branch 'refactor/324-remove-dead-permanently-failed' is based on master
Summary: 5 PASS, 1 FAIL
```

## [HARNESS-OVERRIDE] Gate 1.1

Same override as #322/#331/#330: Open PR #321 (`feat/320-release-sha256` by kokorolx) is unrelated, touches `.github/workflows/` + release scripts. Zero file overlap with #324 (which touches `internal/embed/` + `internal/storage/`).

## Decision: REMOVE (not WIRE) — based on forensic git-history evidence

**HIGH confidence (95%)** decision from explore investigation:

- **b4373ef (May 30, 2026, PR #208/#209)**: Added `MarkChunkEmbedPermanentlyFailed` + `isDeterministicEmbedError` + tests asserting it would be called
- **9a53f80 (May 31, 2026, PR #260/#267, NEXT DAY)**: Refactored to use `MarkChunkEmbedFailed` instead. Renamed error classifier to `isHardFailureEmbedError`. Replaced the old test with hard-failure tests.
- **3+ weeks of zero production callers**: design settled, dead code never removed

The `permanent failure` semantic was abandoned by design within 24 hours of being introduced. The current architecture treats all hard failures as `embed_failed`. There is no plan to wire this code; keeping it imposes:
- Cognitive load (developers wonder why two failure states exist)
- Latent crash risk (if anyone wires it, PG error 23514 from CHECK constraint)
- Schema sprawl (would require migration 00015 + handler updates)

## Skip justifications

- OpenSpec proposal: SKIP (tiny lane, pure code-deletion, no semantic change)
- smoke:e2e: SKIP (change-type=refactor per HARNESS.md table)
- Review gate: ⚠️ self-verify only (change-type=refactor per HARNESS.md table)
- Integration tests: SKIP (no schema change, no migration)

## Required

- validate:quick (build + race -short) — already PASS
- self-review:staged-files — 4 files, deletions only

## Lane justification (tiny)

- 4 files modified, 30 lines deleted, 0 added
- 0 schema changes, 0 API surface changes, 0 migrations
- Risk flags: 0 (pure dead-code removal; tests still cover the active path via `MarkChunkEmbedFailed`)
- Forensic evidence is overwhelming and well-documented
