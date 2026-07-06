# Harness Gates — nano-brain v2

Gate specification for the nano-brain v2 Go project. Each gate defines the checks required before proceeding to the next phase of development.

**Core rules:**
- 1 feature = 1 PR = 1 GitHub issue
- **All PRs target `master`** (the v2 `b-main` staging branch is retired; gate 3.9 enforces this)
- Agent MUST NOT start the next feature until all gates pass
- PASS = all checks in current phase ✅ → proceed
- FAIL = any check ❌ → BLOCK, must fix before continuing
- SKIP = check not applicable (e.g., first feature → skip 1.1, 1.2)
- **State:** git history + `.planning/STATE.md` are the source of truth (the old `docs/harness-state.json` is retired). Process debt is tracked as GitHub issues labeled `harness-debt` — resolve open ones before starting a new epic.
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

| # | Check | How to verify | Evidence Required |
|---|-------|---------------|-------------------|
| 1.1 | No open PR **overlaps** this work (R90) — an open PR blocks only if its changed files intersect this branch's changed files; orthogonal PRs proceed in parallel | `gh pr view <N> --json files` ∩ `git diff --name-only origin/master...HEAD` = empty | Overlap check output |
| 1.2 | No active GSD phase | `.planning/STATE.md` — current phase is "None" or all phases Pending/Completed | STATE.md snapshot |
| 1.3 | GitHub issue exists for new feature | `gh issue view <N>` | Issue URL |
| 1.4 | Branch `master` up-to-date | `git log origin/master..master` = empty | git log output |
| 1.5 | Validation ladder clean on `master` | `go build ./... && go test -race -short ./...` | Build + test output |
| 1.6 | Feature branch created off `master` | `git merge-base --is-ancestor master <branch>` | Branch name + parent commit |
| 1.7 | Deep-design completed (normal+ risk) | `docs/evidence/deep-design-{phase}.md` exists with verdict | Deep-design evidence file |

**SKIP rules:** 
- First feature of the project → skip 1.1 and 1.2
- Tiny lane changes → skip 1.7 (deep-design not required)

---

## ② IN-PROGRESS

Run continuously during development. Check after each story completes.

| # | Check | How to verify |
|---|-------|---------------|
| 2.1 | On feature branch, not `master` | `git branch --show-current` ≠ `master` |
| 2.2 | Active GSD phase exists | `.planning/STATE.md` — current phase shows "in progress" |
| 2.3 | Validation ladder pass after each story | `go build ./... && go test -race -short ./...` |
| 2.4 | Self-review after PR creation — Oracle on PR diff, all critical/major findings fixed, evidence saved | Evidence file `docs/evidence/self-review-{story}.md` exists and no unresolved critical/major findings |

### Review process (gates 2.4 + 3.6)

The review flow is designed for parallelism: create the PR first so Gemini bot starts reviewing, then run Oracle self-review concurrently.

**Step-by-step flow:**

1. **Commit and push** code to feature branch
2. **Create PR** targeting `master` — this triggers Gemini bot review automatically
3. **Run Oracle self-review** on the diff (`git diff master..HEAD` or PR diff) while Gemini is reviewing
4. **Fix ALL critical and major Oracle findings** — push fixes to the same PR branch
5. **Save Oracle review** to `docs/evidence/self-review-{story-id}.md`
6. **Check Gemini PR comments** — read all comments from `gh pr view <N> --comments`
7. **Verify each Gemini finding against actual codebase context** — Gemini lacks full context; fire explore subagents to confirm validity before fixing. Mark each finding as VALID / FALSE POSITIVE / DEFER with reasoning.
8. **Fix VALID critical and high severity Gemini comments** — push fixes to the same PR branch. FALSE POSITIVEs must be documented with explanation in PR thread.
9. **Only after both reviews are clean** → merge is allowed

**Parallelism rules:**

- **Review ∥ Next-story prep**: While Oracle + Gemini review story N, orchestrator MAY start story N+1 **prep only** (create issue, create branch, read spec). NO code/test work until story N merges. If story N review fails critically → cancel N+1 prep, fix N first.
- **POST-MERGE gates run in parallel**: After confirming 4.1 (merged), gates 4.2–4.4 run simultaneously. Gate 4.5 (validation) runs last since it depends on merged code.

**PR comment review rules (R31 — verdict-based, NOT effort-based):**

Every Gemini comment receives an agent verdict from the closed set defined in
HARNESS.md § PR + Bot Review Loop. The verdict — not Gemini's own severity label
and not the agent's effort estimate — determines whether merge is blocked.

- `VALID:critical` / `VALID:high` → BLOCKING. Must include `fixed in commit <sha>`
  in the triage Action column, OR PR must have `[HARNESS-OVERRIDE]` (R7).
- `VALID:medium` / `VALID:low` → NON-BLOCKING. Agent MUST acknowledge in a PR
  reply. Fix is optional. No effort threshold; no "fix if cheap" judgment.
- `FALSE_POSITIVE` → NON-BLOCKING. Reasoning column must explain the context
  Gemini lacks. PR reply explaining the FP is required.
- `DEFER` → NON-BLOCKING. Action column must link to backlog/follow-up issue.
- `ACKNOWLEDGED` → NON-BLOCKING. For informational comments.
- If Gemini posts no comments → proceed (no triage table needed).

**Gemini verification rule (MANDATORY):**
Gemini reviews without full codebase context and frequently flags false positives (e.g., missing deferred rollback it didn't read, wrong driver assumptions, buffer limits that don't apply to the actual usage). Before fixing ANY Gemini comment, fire explore subagents to verify the finding against actual code. This saves wasted fix cycles and prevents introducing unnecessary complexity.

**Verification triage output (save to evidence file):**
| Finding | File | Gemini Severity | Verified Verdict | Action |
|---------|------|----------------|-----------------|--------|
| shouldSkip isDir | filter.go | Critical | VALID — no isDir guard exists | Fix |
| pq.Error assertion | queue.go | High | VALID — pgx/v5 never returns pq.Error | Fix |
| defer rollback missing | persist.go | High | FALSE POSITIVE — defer at line 90 covers it | Reply in PR |

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
| 3.3 | Integration tests introduce **no NEW failing packages** vs `docs/harness-baseline.txt` (R91) — pre-existing master debt is tracked by issues, not paid per-PR | `go test -race -tags=integration ./...` failing packages ⊆ baseline |
| 3.4 | `golangci-lint run --new-from-rev=<merge-base>` clean (R91) — only issues introduced by this branch fail the gate | exit code 0 |
| 3.5 | Review Gate pass by an **independent** reviewer (R27 verdict + R88 reviewer ≠ author) | `docs/evidence/review-<story>.md` for THIS story: `Review Verdict: PASS` + a `Reviewer:` naming a separate spawned sub-agent (not self/author/implementer) |
| 3.6 | PR review comments addressed — all critical/high Gemini comments fixed, medium acknowledged | `gh pr view --comments` — no unresolved critical/high comments |
| 3.7 | CI workflow pass | `gh pr checks` all green |
| 3.8 | PR linked to GitHub issue | PR body contains `Closes #N` |
| 3.9 | PR targets `master` | `gh pr view --json baseRefName` = `master` |
| 3.10 | Self-review evidence exists for this story — SKIP for docs-only changes (R92, change-type detected from `git diff --name-only origin/master...HEAD`) | `ls docs/evidence/self-review-*.md` for current story |
| 3.11 | Max 3 feature commits per PR (R29) — counted via `git rev-list --count <merge-base>..HEAD`, NOT the GitHub PR view (which includes base commits when master advanced) | commit count ≤ 3 |
| 3.12 | smoke:e2e evidence for user-feature/bug-fix (R19, R20) — SKIP for docs-only and test-only changes: no runtime surface (R92). E2E workspace test for extractor/indexer stories: build binary → start server → index a real workspace → verify edges stored → query memory_graph → 0 errors in logs. **Privacy:** use placeholder names in evidence files, never real workspace names/paths | `docs/evidence/smoke-e2e-<story>*` contains curl + HTTP responses |
| 3.13 | E2E extractor test pass — all fixtures in `testdata/<feature>/` exercise the extractor with 0 panics, 0 errors | `go test -race -short -run=E2E ./internal/<pkg>/...` — all E2E tests pass. Fixture files are synthetic (never real project content). |
| 3.14 | Capability benchmark pass — recall-based benchmark against live server, no regression vs baseline | `go test -tags=capbench -run=TestCapabilityBenchmark ./benchmarks/<overlay>/capability/` — overall recall >= baseline. Save results to `results_current.json`. |

### Benchmark procedure (gate 3.14)

For any story that adds/modifies an extractor, indexer, or code intelligence feature,
create a **capability benchmark** following the Rails pattern (`benchmarks/rails/capability/`).
This is NOT a raw `go test -bench` — it is an **agent-oriented recall benchmark** that tests
whether the live server can answer developer questions about the codebase.

**Required for:** user-feature, bug-fix (code intelligence scope)
**Skip for:** infrastructure, refactor, docs, dependency-bump

#### Structure (follow `benchmarks/rails/capability/` exactly)

```
benchmarks/<overlay>/capability/
├── runner.go          ← HTTP client: calls /api/v1/graph/*, /api/v1/symbols, /api/v1/query
├── capability_test.go ← Build tag: capbench. Loads dataset, runs tasks, scores recall, compares baseline
├── dataset.json       ← 15-20 developer questions with expect_symbols + expect_files
├── baseline_v1.json   ← Frozen baseline (CAPBENCH_FREEZE=1)
├── setup.sh           ← Bootstrap: create nanobrain_test DB, migrate, start server on :3199
└── README.md
```

#### runner.go must implement

- `Config{ServerURL, Workspace, Freeze}` from env (`NANO_BRAIN_URL`, `NANO_BRAIN_WORKSPACE`, `CAPBENCH_FREEZE`)
- `Task{ID, Category, Question, Tools, Input, ExpectSymbols, ExpectFiles}` — same schema as Rails
- `AgentPlan{Enabled, Tools, MaxSymbolQueries}` — deterministic agent workflow layer
- Tool callers: `callFlow`, `callImpact`, `callTrace`, `callSymbols`, `callQuery` — POST/GET to `/api/v1/graph/*`
- `RunTask` → unions results from fixed tools + agent augmentation → `scoreTask`
- `scoreTask` = `matched / (ExpectSymbols + ExpectFiles)`, case-insensitive substring match
- `Aggregate` → `BenchResults{Version, Overall, ByCategory, Tasks}`

#### capability_test.go must

1. Skip if server unreachable (`t.Skipf`)
2. Load `dataset.json`, run all tasks via `RunTask`
3. Aggregate, print scorecard, write `results_current.json`
4. If `CAPBENCH_FREEZE=1` → write `baseline_v1.json`, return
5. Otherwise compare vs `baseline_v1.json` — **FAIL if overall < baseline - 0.001**

#### dataset.json categories for code intelligence

| Category | Example question |
|----------|-----------------|
| `graph-out` | "What does Button.vue import?" |
| `graph-in` | "What files import Button.vue?" |
| `trace` | "What does handleClick() eventually call?" |
| `impact` | "What breaks if I change Button.vue?" |
| `symbol-lookup` | "Where is the useConfirmation composable?" |
| `multi-tool` | "Show me the flow starting from app.vue and trace all its calls" |

#### Run command

```bash
# Setup (once)
cd benchmarks/<overlay>/capability && ./setup.sh

# Run benchmark
go test -v -tags=capbench -run=TestCapabilityBenchmark ./benchmarks/<overlay>/capability/

# Freeze baseline
CAPBENCH_FREEZE=1 go test -v -tags=capbench -run=TestCapabilityBenchmark ./benchmarks/<overlay>/capability/
```

#### FAIL conditions

- Overall recall < baseline - 0.001 → FAIL
- Server unreachable → SKIP (not FAIL — benchmark requires live server)
- Any task panics → FAIL

#### Privacy

- Use placeholder workspace names in dataset (`vue-app`, `rails-app`)
- Actual workspace hash supplied at runtime via `NANO_BRAIN_WORKSPACE`
- Never commit real workspace names, paths, or hashes

---

## ④ POST-MERGE

Run immediately after the PR merges.

| # | Check | How to verify |
|---|-------|---------------|
| 4.1 | PR merged successfully | `gh pr view --json state` = MERGED |
| 4.2 | GitHub issue auto-closed | `gh issue view <N> --json state` = CLOSED |
| 4.3 | GSD phase completed | `.planning/STATE.md` — current phase marked "completed" or moved to next |
| 4.4 | Feature branch deleted | `git branch -r` no longer has branch |
| 4.5 | `master` validation pass after merge | `go build ./... && go test -race -short ./...` |

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

## Rules added by retro 2026-07-06 (override-pattern recalibration)

Derived from auditing 40 gate overrides across 23 evidence files — see
`docs/evidence/retro-harness-recalibration-2026-07-06.md`.

- **R90 — overlap-based serialization (gate 1.1).** An open PR blocks new work
  only when its changed files intersect the new branch's changed files.
  Zero-overlap PRs proceed in parallel. Rationale: 8/8 gate-1.1 overrides
  documented zero file overlap.
- **R91 — differential quality gates (gates 3.3, 3.4).** A PR must not make
  master worse; it is not required to pay off pre-existing debt. Gate 3.3
  compares failing packages against `docs/harness-baseline.txt`; suspects are
  re-run in isolation before counting as NEW (flaky guard). The baseline is
  **shrink-only, enforced**: gate 3.3 FAILs if the PR's diff adds non-comment
  lines to the baseline (initial seeding exempt); growing it requires an R7
  override. Gate 3.4 uses `golangci-lint run --new-from-rev=<merge-base>`.
  Rationale: 21/40 overrides were pre-existing master failures.
- **R92 — change-type–aware gates (gates 3.10, 3.12).** Change type is
  detected measurably from `git diff --name-only origin/master...HEAD`, by
  **extension only** (never by directory — `docs/foo.go` must not classify as
  docs): all files `.md`/`.txt` → `docs`; all files `_test.go`/`testdata/`/
  `.md`/`.txt` → `test-only`. Docs-only skips 3.10 + 3.12; test-only skips
  3.12. This makes the checker honor the HARNESS.md change-type table it
  previously ignored.
