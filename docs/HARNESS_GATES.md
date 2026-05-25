# Harness Gates — nano-brain v2

Gate specification for the nano-brain v2 Go project. Each gate defines the checks required before proceeding to the next phase of development.

**Core rules:**
- 1 feature = 1 PR = 1 GitHub issue
- **All story PRs target `b-main`** — NEVER target `master` directly
- `master` is updated ONLY via a final release PR when v2 is complete
- Agent MUST NOT start the next feature until all gates pass
- PASS = all checks in current phase ✅ → proceed
- FAIL = any check ❌ → BLOCK, must fix before continuing
- SKIP = check not applicable (e.g., first feature → skip 1.1, 1.2)
- **State file:** `docs/harness-state.json` — read BEFORE every story/epic transition. If any debt item has `"status":"open"`, resolve before proceeding.
- **Delegation rule:** Orchestrator (Sisyphus) does NOT write code or tests directly. ALL code/test work MUST be delegated to subagents. Orchestrator only: plans, delegates, verifies results, manages git/PR workflow.

---

## Flow

```
① PRE-WORK → ② IN-PROGRESS → ③ PRE-MERGE → ④ POST-MERGE → ⑤ NEXT-READY
                                                                    │
                                                          (if end of epic)
                                                                    ▼
                                                             ⑥ RETRO-GATE
                                                                    │
                                                                    ▼
                                                          ① PRE-WORK (next epic)
```

---

## ① PRE-WORK

Run before starting any new feature.

| # | Check | How to verify |
|---|-------|---------------|
| 1.1 | Previous feature PR merged & issue closed | `gh pr list --state merged`, `gh issue view` |
| 1.2 | Previous OpenSpec change archived | `openspec list` — no active changes |
| 1.3 | GitHub issue exists for new feature | `gh issue view <N>` |
| 1.4 | Branch `b-main` up-to-date | `git log origin/b-main..HEAD` = empty |
| 1.5 | Validation ladder clean on `b-main` | `go build ./... && go test -race -short ./...` |
| 1.6 | Feature branch created off `b-main` (NOT master) | `git log --oneline b-main..HEAD` — parent is b-main |

**SKIP rules:** First feature of the project → skip 1.1 and 1.2.

---

## ② IN-PROGRESS

Run continuously during development. Check after each story completes.

| # | Check | How to verify |
|---|-------|---------------|
| 2.1 | On feature branch, not `b-main` | `git branch --show-current` ≠ `b-main` |
| 2.2 | OpenSpec change exists & active | `openspec list` has current change |
| 2.3 | Validation ladder pass after each story | `go build ./... && go test -race -short ./...` |
| 2.4 | Self-review after PR creation — Oracle on PR diff, all critical/major findings fixed, evidence saved | Evidence file `docs/evidence/self-review-{story}.md` exists and no unresolved critical/major findings |

### Review process (gates 2.4 + 3.6)

The review flow is designed for parallelism: create the PR first so Gemini bot starts reviewing, then run Oracle self-review concurrently.

**Step-by-step flow:**

1. **Commit and push** code to feature branch
2. **Create PR** targeting `b-main` — this triggers Gemini bot review automatically
3. **Run Oracle self-review** on the diff (`git diff b-main..HEAD` or PR diff) while Gemini is reviewing
4. **Fix ALL critical and major Oracle findings** — push fixes to the same PR branch
5. **Save Oracle review** to `docs/evidence/self-review-{story-id}.md`
6. **Check Gemini PR comments** — read all comments from `gh pr view <N> --comments`
7. **Fix ALL critical and high severity Gemini comments** — push fixes to the same PR branch
8. **Only after both reviews are clean** → merge is allowed

**Parallelism rules:**

- **Review ∥ Next-story prep**: While Oracle + Gemini review story N, orchestrator MAY start story N+1 **prep only** (create issue, create branch, read spec). NO code/test work until story N merges. If story N review fails critically → cancel N+1 prep, fix N first.
- **POST-MERGE gates run in parallel**: After confirming 4.1 (merged), gates 4.2–4.4 run simultaneously. Gate 4.5 (validation) runs last since it depends on merged code.

**PR comment review rules:**
- Critical/High severity Gemini comments → MUST fix before merge (BLOCKING)
- Medium severity Gemini comments → FIX if effort < 15 min, otherwise note in PR and defer
- Low/Informational comments → ACKNOWLEDGE in PR, fix is optional
- If Gemini finds nothing → proceed (no evidence needed beyond PR review thread)

**Self-review evidence file format:**

```markdown
## Self-Review: {story-id}
Date: {date}
Reviewer: Oracle / review-work

## Findings
| # | Severity | File | Description | Status |
|---|----------|------|-------------|--------|
| 1 | critical | src/foo.go | Missing error handling | FIXED |
| 2 | major | src/bar.go | Potential nil pointer | FIXED |
| 3 | minor | src/baz.go | Inconsistent naming | DEFERRED |

## Summary
- Critical: 1 found, 1 fixed
- Major: 1 found, 1 fixed  
- Minor: 1 found, 0 fixed (deferred to next sprint)
```

---

## ③ PRE-MERGE

Run before creating or merging a PR. All checks must be green.

| # | Check | How to verify |
|---|-------|---------------|
| 3.1 | `go build ./...` pass | exit code 0 |
| 3.2 | `go test -race -short ./...` pass | exit code 0 |
| 3.3 | `go test -race -tags=integration ./...` pass | exit code 0 |
| 3.4 | `golangci-lint run` clean (if available) | exit code 0 |
| 3.5 | Review Gate pass (Oracle or review-work) | evidence in PR/issue |
| 3.6 | PR review comments addressed — all critical/high Gemini comments fixed, medium acknowledged | `gh pr view --comments` — no unresolved critical/high comments |
| 3.7 | CI workflow pass | `gh pr checks` all green |
| 3.8 | PR linked to GitHub issue | PR body contains `Closes #N` |
| 3.9 | PR targets `b-main` (NOT master) | `gh pr view --json baseRefName` = `b-main` |
| 3.10 | Self-review evidence exists for this story | `ls docs/evidence/self-review-*.md` for current story |

---

## ④ POST-MERGE

Run immediately after the PR merges.

| # | Check | How to verify |
|---|-------|---------------|
| 4.1 | PR merged successfully | `gh pr view --json state` = MERGED |
| 4.2 | GitHub issue auto-closed | `gh issue view <N> --json state` = CLOSED |
| 4.3 | OpenSpec archived | `openspec archive "<name>"` done |
| 4.4 | Feature branch deleted | `git branch -r` no longer has branch |
| 4.5 | `b-main` validation pass after merge | `go build ./... && go test -race -short ./...` |

---

## ⑤ NEXT-READY

Permission gate to start the next feature. All prior phases must be complete.

| # | Check | How to verify |
|---|-------|---------------|
| 5.1 | All ① – ④ of previous feature = ✅ | aggregate |
| 5.2 | No stale open PRs/issues | `gh pr list --state open` = empty |
| 5.3 | No uncommitted changes | `git status --porcelain` = empty |

Once all three checks pass, proceed to ① PRE-WORK for the next feature.

---

## ⑥ RETRO-GATE

Retrospective gate. Mandatory after every epic completes, and triggered mid-epic on signal.

### Trigger conditions

- After every epic completion — mandatory
- Mid-epic if 3+ consecutive stories fail review gate — emergency retro
- Any PR with review cycle count > 2 — flag for retro

### Checks

| # | Check | How to verify |
|---|-------|---------------|
| 6.1 | Count PR review cycles of completed epic | `gh pr view` — how many push cycles? |
| 6.2 | Count CI failures in epic | scan workflow runs |
| 6.3 | Count review-gate FAIL before PASS | evidence files |
| 6.4 | Classify recurring errors (pattern analysis) | agent analysis: type error? test miss? logic bug? |
| 6.5 | Compare with previous epic (trend) | metrics improving or declining? |

### Output

Save retro output to `docs/evidence/retro-epic-{N}.md` with these sections:

```
## Metrics
## Pattern Analysis
## Root Cause
## Harness Rule Updates
## Applied Changes
```

---

## Enforcement Summary

| Result | Meaning | Action |
|--------|---------|--------|
| PASS | All checks in phase ✅ | Proceed to next phase |
| FAIL | Any check ❌ | BLOCK — fix before continuing |
| SKIP | Check not applicable | Document reason, proceed |

See `docs/HARNESS.md` for the Review Gate and PR Bot Review process referenced in gate 3.5 and 3.6.
