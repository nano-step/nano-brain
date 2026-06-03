# PRE-MERGE gate — Issue #343 / PR #344

**Date**: 2026-06-03
**Issue**: #343 (chore(openspec): propose nano-brain-cli-ux-overhaul)
**PR**: #344
**Branch**: `chore/openspec-cli-ux-overhaul-proposal`
**Lane**: tiny | **Change-type**: docs (spec-only)

## Gate Run Output

```
─ PRE-MERGE checks
[PASS] 3.1 go build ./...
[PASS] 3.2 go test -race -short ./...
[FAIL] 3.3 go test -race -tags=integration ./... failed
[FAIL] 3.4 golangci-lint found issues
[PASS] 3.5 Review Verdict: PASS in docs/evidence/review-gate-188.md (R27)
[PASS] 3.6 No Gemini comments on PR
[SKIP] 3.7 No open PR or CI not yet complete
[PASS] 3.8 PR closes exactly 1 issue (R1)
[PASS] 3.9 PR targets master
[FAIL] 3.10 No self-review evidence for story openspec
[PASS] 3.11 PR commit count: 1 (R29: ≤ 3)
[SKIP] 3.12 No smoke-e2e-openspec*.{md,txt} (R19)
[SKIP] 3.13 no web change in PR diff (smoke:ui not required)
Summary: 7 PASS, 3 FAIL, 3 SKIP (13 total)
```

## [HARNESS-OVERRIDE] Gate 3.3 — integration tests

**Reason**: Pre-existing failures on master, NOT introduced by PR #344. The PR diff is 8 spec markdown files (729 insertions, 0 deletions). Zero `.go` files touched. Cannot have introduced these failures.

**Evidence (run from worktree, branched off latest master `23b5afa`)**:

```
$ git diff --name-only master HEAD
openspec/changes/nano-brain-cli-ux-overhaul/design.md
openspec/changes/nano-brain-cli-ux-overhaul/proposal.md
openspec/changes/nano-brain-cli-ux-overhaul/specs/cli-binary-resolution/spec.md
openspec/changes/nano-brain-cli-ux-overhaul/specs/cli-doctor-runtime/spec.md
openspec/changes/nano-brain-cli-ux-overhaul/specs/cli-install-path-optimization/spec.md
openspec/changes/nano-brain-cli-ux-overhaul/specs/cli-mcp-url-resolution/spec.md
openspec/changes/nano-brain-cli-ux-overhaul/specs/skill-distribution-docs/spec.md
openspec/changes/nano-brain-cli-ux-overhaul/tasks.md
```

```
$ go test -race -tags=integration -count=1 -timeout=60s ./internal/harvest/ ./internal/server/handlers/
FAIL  github.com/nano-brain/nano-brain/internal/server/handlers [build failed]
--- FAIL: TestOpenCodeSQLite_OrphanSession_NoWorktree_Skipped (0.15s)
--- FAIL: TestOpenCodeSQLite_UnregisteredWorktree_Skipped (0.18s)
FAIL  github.com/nano-brain/nano-brain/internal/harvest 2.238s
```

Reproduced identically on master HEAD (`git checkout master -- internal/`). All other packages PASS.

These pre-existing failures should be tracked by a separate issue — recommended follow-up: file an issue covering both the `handlers` build failure and the `harvest` test failures, then reference it in `docs/HARNESS_BACKLOG.md`.

## [HARNESS-OVERRIDE] Gate 3.4 — golangci-lint

**Reason**: Pre-existing lint violations on master, NOT introduced by PR #344. Same zero-Go-files-in-diff argument as 3.3.

## Gate 3.10 — self-review evidence

Created at `docs/evidence/self-review-openspec-cli-ux-overhaul-proposal.md`. This file references it for completeness.

## R19 — smoke:e2e not required

Change-type is `docs` (spec-only). Per `docs/HARNESS.md` change-type table, smoke:e2e is `❌` for docs. Gate 3.12 correctly SKIP'd.

## R56 — Review Gate not required

Tiny lane + docs change-type: Review Gate is `❌` per harness change-type table.

## Validation summary

- validate:quick: ✅ PASS — full output in self-review file
- self-review:staged-files: ✅ PASS — only intended files
- self-review:response-shape: N/A — no API surface change
- 1 commit, atomic, conventional message, Closes #343
- openspec validate --strict --no-interactive: ✅ PASS
- Pre-existing failures (3.3 + 3.4) overridden with evidence; awaiting user `[HARNESS-OVERRIDE]: <reason>` comment on PR #344 per R7
