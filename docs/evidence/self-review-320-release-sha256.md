# Self-Review — Release-binary SHA-256 integrity verification (#320)

**Date**: 2026-06-02 (UTC)
**Branch**: `feat/320-release-sha256`
**Lane**: normal (infrastructure)
**Implementer**: Sisyphus (autonomous; user delegated full review responsibility for both the feature and the PR-merge decision)

> **Note on the no-self-review rule**: AGENTS.md forbids the implementing
> agent from running its own Review Gate. The user explicitly waived this
> (`"yes, for sure"` followed by `"option 2"` — full deep-design +
> implementation). For `infrastructure` change-type the harness Review Gate
> is already `⚠️ self-verify` per `docs/HARNESS.md`, so the substitution is
> milder than for the prior `user-feature` PR #318. The deep-design pipeline
> (Metis + Oracle + Momus sanity + Momus full review) ran before any code
> was written.

## Pipeline outputs

| Stage | Agent | Verdict |
|---|---|---|
| Phase 1 scope analysis | Metis | HIGH-confidence decisions on all design questions; normal lane confirmed |
| Phase 1 architecture | Oracle | HIGH-confidence on tool choice (`sha256sum`), compute site (release job), stream-tee pattern; identified the existing `download()` function as the model to follow |
| Phase 2 synthesis | Sisyphus | All decisions HIGH-confidence; one minor divergence resolved (stream-tee over read-twice) |
| Phase 2.5 sanity | Momus | PASS |
| Phase 4 full review | Momus | APPROVED — 3 non-blocking recommendations folded in before coding (transient-network spec scenario, stream-tee implementation pattern locked in tasks.md, exit-code change callout in design.md) |

## Diff summary

| File | Type | Δ Lines | Purpose |
|---|---|---|---|
| `.github/workflows/release.yml` | YAML | +3 / −0 | Add `Compute SHA-256 checksums` step between `download-artifact` and `softprops/action-gh-release` |
| `npm/postinstall.js` | Source | +106 / −4 | Add `crypto` require; add `downloadWithHash()` streaming function; add `parseSHA256Line()` pure parser; add `verifySHA256()` orchestrator; add escape-hatch env check in `main()`; route both binary-download call sites through `downloadWithHash` + `verifySHA256`; CommonJS export of helpers for tests |
| `npm/postinstall.test.js` | Test (new) | +97 | Node built-in `--test` runner, 11 test cases covering `parseSHA256Line` happy/missing/malformed/CRLF/multi-entry/exact-match/short-hex/lowercase + 2 end-to-end-style cases using real `crypto.createHash` |
| `README.md` | Docs | +27 / −0 | New "Verifying Downloads" subsection after Quick Start showing `sha256sum -c` flow, automatic verification note, escape hatch, backward-compat note, issue link |
| `openspec/changes/release-sha256-checksums/{proposal,design,tasks}.md` + `specs/release-pipeline/spec.md` | OpenSpec (new) | +432 | Full proposal artifacts, validated `openspec validate --strict --no-interactive` PASS |

Total production diff: ~115 LOC (workflow + postinstall + README). Test diff: ~97 LOC. Doc diff: ~432 LOC OpenSpec + 27 README. No Go code changed.

## Verification ladder (per `docs/HARNESS.md` infrastructure change-type)

| Layer | Command | Result |
|---|---|---|
| `validate:quick` Go build | `go build ./...` | ✅ PASS |
| `validate:quick` Go tests | `go test -race -short ./...` | ✅ PASS — 22 packages green (confirms no Go regression even though no Go was touched) |
| `validate:quick` Node tests | `node --test npm/postinstall.test.js` | ✅ PASS — 11/11 tests pass |
| YAML syntax | `python3 -c 'yaml.safe_load(open("release.yml"))'` | ✅ PASS |
| JS syntax | `node --check npm/postinstall.js` | ✅ PASS |
| OpenSpec strict | `openspec validate release-sha256-checksums --strict --no-interactive` | ✅ PASS |
| `self-review:staged-files` | `git status` — only release.yml + postinstall.js + postinstall.test.js + README.md + openspec change dir + this evidence file | ✅ Clean |
| `test:integration` | N/A — no Go change | ❌ Not required |
| `smoke:e2e` | N/A — `infrastructure` change-type exempt per HARNESS.md change-type table | ❌ Not required |

## Acceptance criteria coverage

All 8 ACs from `proposal.md` are covered:

| AC | Coverage | Status |
|---|---|---|
| 1. SHA256SUMS published | release.yml step added at line 55: `cd artifacts && sha256sum nano-brain-* > SHA256SUMS`. Picked up by existing `files: artifacts/*` glob. | ✅ |
| 2. Manual verification works | README "Verifying Downloads" section documents `sha256sum -c SHA256SUMS --ignore-missing` flow with full curl example. | ✅ |
| 3. Automatic verification on install | `main()` in postinstall.js calls `downloadWithHash` + `verifySHA256` at both candidate-tag loop AND API-fallback paths. Hard fail (`process.exit(1)`) on `SECURITY:`-prefixed errors. | ✅ |
| 4. Backward compat — old release w/o SHA256SUMS | `verifySHA256` catches the SHA256SUMS download failure (any HTTP error, including 404) and emits a single WARN before returning. Caller proceeds to `chmod 0755`. | ✅ |
| 5. Backward compat — SHA256SUMS network failure | Same `verifySHA256` catch path covers transient 5xx / DNS / connection-reset. Spec scenario added explicitly (`Transient SHA256SUMS network failure` in spec.md). | ✅ |
| 6. Escape hatch | `main()` checks `process.env.NANO_BRAIN_SKIP_SHA_VERIFY` at the top; if truthy, emits one-line WARN and routes both call sites through the legacy `download()` path. | ✅ |
| 7. README updated | "Verifying Downloads" subsection inserted between Quick Start and Configuration. | ✅ |
| 8. parseSHA256Line tests | 11 test cases in `npm/postinstall.test.js`, runnable via `node --test`, zero new deps. | ✅ |

## Risk audit (per `docs/FEATURE_INTAKE.md`)

Risk flags this change carries:

- ✅ **Existing behavior**: modifies `npm/postinstall.js` install flow — additive (skip-friendly via env var) and backward compatible (soft fail on old releases)
- ✅ **External systems**: GitHub Releases API interaction — but only adds one download to an existing list
- ⚠️ **Audit-security**: additive — adds a security verification path, does NOT modify existing audit/access surfaces (per Metis HIGH confidence). Hard gates table in HARNESS.md treats this as `additive ≠ hard gate`.

Flag count: 2–3 (additive audit-security is half-flag at most). **Lane: normal.** No hard gates triggered.

The mismatch hard-fail (`process.exit(1)`) is the FIRST hard-fail path in postinstall.js — all prior failure paths use `process.exit(0)` (best effort). This is intentional and called out in `design.md` decision log. It's not a behavior regression because: (a) it only fires on actual hash mismatch, which today either silently corrupts the install OR coincides with a complete download failure (current code already prints an error); (b) AC #5 (network failure ≠ mismatch) preserves best-effort semantics for the common transient-failure case.

## Surgical-change audit

Per AGENTS.md "Surgical Changes" guideline: every changed line traces directly to issue #320.

- `release.yml`: only the 3-line `Compute SHA-256 checksums` step. No other changes to the matrix, the build job, the npm-publish job, or the existing release step.
- `postinstall.js`: only new functions (downloadWithHash, parseSHA256Line, verifySHA256), one new `crypto` require, escape-hatch env check at top of `main()`, and updates to the two existing binary-download call sites. Version-normalization logic at lines 58–108 was NOT touched (explicit MUST NOT in tasks.md).
- `postinstall.test.js`: new file, no equivalent test infrastructure to extend.
- `README.md`: only the new "Verifying Downloads" subsection inserted between two existing sections; no other lines modified.

## Pre-existing failures (NOT introduced by this change)

The current `origin/master` (commit a4745ae from the #317 archive) has integration-test build failures and golangci-lint warnings unrelated to this work:

- `internal/harvest/opencode_sqlite_integration_test.go:167` — `undefined: q` (under `-tags=integration`)
- `internal/search/isolation_test.go:315` — `HybridSearch` arg-count mismatch (under `-tags=integration`)
- `internal/server/handlers` integration tests for events + workspaces-list (under `-tags=integration`)
- `golangci-lint run ./...` reports ~6 warnings in handlers + cmd packages

None are in files I changed. None are regressions from this PR. Documented in PR body for separate triage.

## Lessons applied from PR #318

1. **Gemini bot review WILL be triaged per R31** with closed-vocab verdicts in the next review cycle. If Gemini files inline comments after push, I'll triage them in `## Gemini Verification Triage` section appended here.
2. **Archive of the OpenSpec change WILL go through a PR**, not direct-to-master. The #317 archive direct-push violation is documented in tasks.md as a warning.
3. **The integration test build failures** noted above are not regressions — re-confirmed against pristine `origin/master`.

## Verdict

**APPROVED for merge** (subject to PR bot review).

- Implementation matches the design brief exactly (no scope creep — no cosign, no SLSA, no backfill, no Content-Length check, no self-verify CI step).
- All 8 acceptance criteria verified with reproducible evidence (Node test output + Go build + YAML lint + manual reasoning trail in the diff).
- Validation ladder passes on every layer in scope for `infrastructure` lane (Go build/tests confirm no regression; Node tests confirm new code works; YAML/JS syntax confirms artifacts are valid).
- The 3 Momus recommendations were folded into spec/design/tasks before coding.
- Test environment was not needed (no Go change, no server needed) — `node --test` runs entirely in-process.
