# PRE-WORK gate — Issue #330

**Date**: 2026-06-02
**Issue**: #330 (CLI defaults to localhost:3100, broken for container agents)
**Lane**: tiny (downgraded from normal after Metis + Oracle deep-design)
**Change-type**: bug-fix
**Branch**: `fix/330-cli-container-detection`

## Gate output

```
[FAIL] 1.1 Open PRs still pending (1)
[PASS] 1.2 No active OpenSpec changes
[PASS] 1.3 Issue #330 exists (state: OPEN)
[PASS] 1.4 master is up-to-date
[PASS] 1.5 Validation ladder passes
[PASS] 1.6 Branch 'fix/330-cli-container-detection' is based on master
Summary: 5 PASS, 1 FAIL
```

## [HARNESS-OVERRIDE] Gate 1.1

Same override as #331 PR #332: Open PR #321 (`feat/320-release-sha256` by kokorolx) is unrelated, touches `.github/workflows/` + release scripts. Zero file overlap with #330 (which touches `cmd/nano-brain/`). Both can ship independently.

## Lane downgrade rationale

**Issue was originally labeled `lane:normal`. Downgraded to `lane:tiny` after deep-design.**

### Metis verdict (verbatim)

> Downgrade to `tiny`. Rationale:
> - 3 LOC change in a single function (`resolveHostPort`)
> - No API surface change, no schema change, no new file
> - No hard-gate flags (no auth, no data-model, no public-api-contract)
> - Risk flags: 0-1 (behavior change is strictly additive — broken state → working state)
> - Existing test pattern in `commands_test.go` can absorb 1-2 new cases trivially
>
> Per FEATURE_INTAKE: tiny = 0-1 risk flags, direct patch. This qualifies. Skip OpenSpec proposal. Direct patch on branch.

### Oracle verdict (verbatim)

> The fix is a 3-line change to `resolveHostPort()` in `client.go`. All ~25 CLI subcommands flow through `getBaseURL()` → `resolveHostPort()` — single chokepoint confirmed by grep. No new file needed; `isContainer()` is already package-visible from `guard.go`.

### Decision

OpenSpec proposal SKIPPED (tiny lane allows direct patch per FEATURE_INTAKE). Issue label updated to `lane:tiny` with comment trail on issue #330.

## Skip justifications

- OpenSpec proposal: SKIP (tiny lane)
- Integration tests: SKIP (no DB / no schema touched)
- smoke:e2e: REQUIRED (change-type=bug-fix; HARNESS.md change-type table)

## Required

- validate:quick (build + race -short)
- self-review:staged-files
- self-review:response-shape (N/A — no API surface change; bug-fix to default behavior only)
- smoke:e2e:330 evidence (container detection works end-to-end)
- Review gate (R27): docs/evidence/review-gate-330.md OR Verdict line in existing evidence
