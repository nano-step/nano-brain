# Self-Review — workspace-local `.nano-brainignore` (#317)

**Date**: 2026-06-02 (UTC)
**Branch**: `feat/317-workspace-nano-brainignore`
**Lane**: normal (user-feature)
**Implementer**: Sisyphus (autonomous; user delegated full review responsibility)

> **Note on the no-self-review rule**: AGENTS.md forbids the implementing
> agent from running its own Review Gate. The user explicitly waived this
> ("tôi không review, bạn tự review và chịu trách nhiệm") and accepted
> responsibility for the result. This document fulfills the harness
> requirement of an explicit verdict + evidence trail and substitutes the
> external review with documented reasoning from the deep-design pipeline
> (Metis + Oracle + cross-critique + Momus full review) that ran before
> code was written.

## Pipeline outputs (independent agents)

| Stage | Agent | Verdict |
|---|---|---|
| Phase 1 scope analysis | Metis | Settled — 0–1 risk flags, normal lane recommended |
| Phase 1 architecture | Oracle | Settled — separate `localIgnore` field, inline load in `newFileFilter` |
| Phase 1.5 cross-critique | Metis (re-reviewing Oracle) | WARN + DEBUG log, normal lane (HIGH confidence) |
| Phase 1.5 cross-critique | Oracle (re-reviewing Metis) | Confirmed WARN, DEBUG, normal lane (HIGH confidence) |
| Phase 2.5 sanity | Momus | PASS |
| Phase 4 full review | Momus | APPROVED WITH CONDITIONS — `go-gitignore` library never errors on content; rename test to `Unreadable`, switch test strategy to directory-at-path |

The Momus condition was addressed before any code was written (see commit
b97b947 — proposal/design/tasks/spec all updated, openspec validate passed
strict).

## Diff summary

| File | Type | Δ Lines | Purpose |
|---|---|---|---|
| `internal/watcher/filter.go` | Source | +23 / −2 | Add `localIgnore` field; load `<rootDir>/.nano-brainignore` in `newFileFilter`; change signature to `(*fileFilter, error)`; add nil-checked check in `shouldSkip` |
| `internal/watcher/watcher.go` | Source | +7 / −1 | Handle new error from `newFileFilter` at the one production call site; emit DEBUG on success, WARN on IO failure |
| `internal/watcher/filter_test.go` | Test | +120 / −9 | 5 new tests covering applies / missing / compose-global / compose-gitignore / unreadable. Updated 9 existing call sites for new signature |
| `README.md` | Docs | +55 / −12 | Restructure "Global ignore patterns" → "Ignore patterns" with both subsections; updated precedence table; reload-semantics paragraph |
| `openspec/changes/workspace-nano-brainignore/{proposal,design,tasks}.md` + `specs/watcher-file-filtering/spec.md` | OpenSpec | +404 | Full proposal artifacts, validated `openspec validate --strict` |
| `docs/evidence/smoke-e2e-317-workspace-nano-brainignore.md` | Evidence | +173 | Smoke E2E test session against isolated PG `nanobrain_smoke317` on port 4199 |

Total production diff: ~30 LOC. Test diff: ~120 LOC. Doc diff: ~230 LOC.

## Verification ladder

| Layer | Command | Result |
|---|---|---|
| `validate:quick` build | `go build ./...` | ✅ PASS |
| `validate:quick` tests | `go test -race -short ./...` | ✅ PASS — 25 packages, all green |
| `lint` (watcher pkg only) | `golangci-lint run ./internal/watcher/...` | ✅ PASS — no findings |
| `test:integration` (watcher pkg) | `go test -race -tags=integration ./internal/watcher/...` | ✅ PASS |
| `test:integration` (all pkgs) | `go test -race -tags=integration ./...` | ⚠️ 2 pre-existing build failures and 2 pre-existing test failures unrelated to this change — see "Pre-existing failures" below |
| `smoke:e2e` | manual server + curl | ✅ PASS — see `smoke-e2e-317-workspace-nano-brainignore.md` |

## Acceptance criteria coverage

All 8 ACs from proposal.md are covered:

| AC | Test/evidence | Status |
|---|---|---|
| 1. File honored | smoke TC-1 + `TestFileFilter_LocalNanoBrainIgnoreApplies` | ✅ |
| 2. File missing = no-op | `TestFileFilter_LocalNanoBrainIgnoreMissing` | ✅ |
| 3. Compose with global | `TestFileFilter_LocalNanoBrainIgnoreCombinesWithGlobal` | ✅ |
| 4. Compose with `.gitignore` | `TestFileFilter_LocalNanoBrainIgnoreCombinesWithGitignore` | ✅ |
| 5. Unreadable file → WARN, continue | smoke TC-4 + `TestFileFilter_LocalNanoBrainIgnoreUnreadable` | ✅ |
| 6. Successful load → DEBUG log | smoke TC-1 (log line captured) | ✅ |
| 7. Precedence documented | README.md updated section "Ignore patterns" | ✅ |
| 8. Re-init picks up changes | smoke TC-3 (2nd DEBUG log + immediate effect) | ✅ |

## Risk audit

### Risk: signature change of `newFileFilter` breaks callers

**Mitigation evidence**: `git grep -n 'newFileFilter('` reveals exactly 1 production call site (`watcher.go:154`) and 9 test call sites — all updated in this PR. `go build ./...` and `go test -race -short ./...` both pass. LOW risk; SETTLED.

### Risk: locked watcher state during IO

`(*Watcher).WatchWithFilter` holds `w.mu.Lock` while calling `newFileFilter`, which performs `os.Stat` + `os.ReadFile`. This is identical to the existing `.gitignore` load pattern at `filter.go:60-65`. Tiny local-filesystem reads, not network IO. No regression. LOW risk; SETTLED.

### Risk: race on `fileFilter.localIgnore`

`fileFilter` is constructed once per collection registration under `w.mu.Lock` and is read-only thereafter. `shouldSkip` performs nil-checked reads only. No mutex required on `localIgnore`. Verified by `go test -race`. LOW risk; SETTLED.

### Risk: documentation drift

The README "Ignore patterns" section now documents both global and workspace-local files with the updated 6-layer precedence. The existing global spec at `openspec/specs/watcher-global-ignore-file/spec.md` is unaffected (this PR adds a new delta in `openspec/changes/workspace-nano-brainignore/specs/watcher-file-filtering/`). LOW risk; SETTLED.

## Surgical-change audit

Per AGENTS.md "Surgical Changes" guideline: every changed line traces directly to issue #317.

- `internal/watcher/filter.go`: only fields/functions related to local ignore. The pre-existing `.gitignore` block at lines 60-65 was kept verbatim (didn't "improve" it).
- `internal/watcher/watcher.go`: only the 6-line block around the `newFileFilter` call. No other lines touched.
- `internal/watcher/filter_test.go`: signature update via single Edit `replaceAll=true` (mechanical). 5 new tests appended at end. No existing test logic modified.
- `README.md`: only the "Global ignore patterns" section was restructured; everything before/after is byte-identical.

## Pre-existing failures (NOT caused by this change)

Confirmed by re-running the same commands against `origin/master` HEAD (`7d2b12f`):

1. `internal/harvest/opencode_sqlite_integration_test.go:167` — `undefined: q` (build failure with `-tags=integration`). Present on master before this PR.
2. `internal/search/isolation_test.go:315` — `HybridSearch` argument count mismatch (4 args provided, 5 required). Present on master before this PR.
3. `internal/server/handlers` integration tests `TestEventsIntegration_ReindexPublishesSequence` and `TestListWorkspacesE2E` fail. Present on master before this PR.
4. `golangci-lint run ./...` produces ~6 warnings (errcheck on `json.Unmarshal` in tests, unused funcs in `documents_test.go`, `events_test.go`, and `cmd_detect_changes.go`). None in `internal/watcher/` — confirmed clean.

These are **out of scope** for this PR per the "don't fix pre-existing issues unless asked" rule. Will note them in the PR body for triage as separate issues.

## Gemini Verification Triage

Per R31 in `docs/HARNESS.md`: every Gemini PR comment must be triaged using the closed verdict vocabulary.

| Comment ref                | Agent verdict   | Reasoning                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | Action                       |
| -------------------------- | --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------- |
| PR#318 filter.go L73-80    | VALID:high      | Gemini correctly identified that the original `os.Stat` + `CompileIgnoreFile` pattern silently swallowed non-`IsNotExist` errors. This violated AC #5 in proposal.md (which explicitly named "permission denied" as a case that MUST log WARN). My `TestFileFilter_LocalNanoBrainIgnoreUnreadable` test passed only because `os.Stat` on a directory succeeds — `chmod 0000` on a regular file would have failed `Stat` and been silently skipped, never reaching the WARN path. Momus full review did not catch this. Replaced with direct `CompileIgnoreFile` + `os.IsNotExist` check. Added `TestFileFilter_LocalNanoBrainIgnorePermissionDenied` regression test (chmod 0000 the file, runtime+euid guards). | fixed in commit `e27b06c` |

**Loop count**: 1 of 3 (max per R31 before human escalation).

## Verdict

**APPROVED for merge** (subject to next PR bot review on the fix commit).

- Implementation matches the design brief exactly (no scope creep).
- All 8 acceptance criteria verified with reproducible evidence (unit tests + smoke logs + curl outputs).
- Validation ladder passes on all layers under my control (watcher package). Pre-existing failures elsewhere are documented and noted as out of scope.
- Production memory was not touched (smoke ran on isolated DB `nanobrain_smoke317`, cleaned up after).
- Gemini review feedback triaged + fixed per R31.
