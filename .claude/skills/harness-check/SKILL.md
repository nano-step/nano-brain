---
name: harness-check
description: |
  Run harness gate checks at every development transition point. Enforces
  that all gates pass before the agent proceeds to the next phase.

  Use this skill when:
  - Starting a new feature (pre-work gate)
  - During development after completing a story (in-progress gate)
  - Before creating or merging a PR (pre-merge gate)
  - After a PR merges (post-merge gate)
  - Before starting the next feature (next-ready gate)
  - After an epic completes or on failure patterns (retro gate)

  Triggers on: "harness check", "gate check", "check gates", "run gates",
  "pre-work check", "pre-merge check", "next-ready", "retro gate",
  "can I start next feature", "are gates green".

compatibility: Claude Code with bash + git + go. Optional: gh CLI, openspec CLI, golangci-lint.
---

# Harness Check

Enforce the harness gate specification defined in `docs/HARNESS_GATES.md`.

> **Note:** The autonomous gate loop (`/harness-on`, `.opencode/plugin/harness-loop/`)
> was OpenCode-only. On Claude Code the drivers are: `/harness-gsd <description>`
> (full autonomous pipeline), this skill (manual gate runs — you drive the
> fix-and-re-run loop from the script output), and the PreToolUse hook
> `.claude/hooks/harness-pre-merge-hook.sh` (blocks `gh pr create` while fast
> pre-merge gates FAIL). See `docs/HARNESS.md` § Entry Points.

## Core Rules

- **1 feature = 1 PR = 1 GitHub issue**
- **Agent MUST NOT start the next feature until all gates pass**
- **FAIL on any check = BLOCK** — fix before continuing
- **R88 — independent review is mandatory.** The code-review pass MUST be done by
  a **separate, spawned sub-agent** (e.g. `code-reviewer`/Oracle). The agent that
  wrote the code MUST NOT review or approve its own code in the same context.
  Self-review evidence is still required but does NOT satisfy the review gate.
  Write the verdict to `docs/evidence/review-<story>.md` with a `Reviewer:` line
  naming the independent agent (gate 3.5 fails if it is missing or names the
  author/self/implementer).

## How to Use

### Step 1 — Determine the phase

Map the current situation to a gate phase:

| Situation | Phase |
|-----------|-------|
| About to start a new feature/story | `pre-work` |
| Just finished a story, still in feature | `in-progress` |
| Ready to create PR or merge | `pre-merge` |
| PR just merged | `post-merge` |
| Want to start the next feature | `next-ready` |
| Epic just completed or 3+ story failures | `retro` |

### Step 2 — Run the script

```bash
./scripts/harness-check.sh <phase> [options]
```

Options:
- `--issue <N>` — GitHub issue number (for pre-work)
- `--pr <N>` — PR number (for post-merge)
- `--epic <N>` — Epic number (for retro)
- `--json` — Machine-readable output
- `--no-color` — Disable colored output

### Step 3 — Interpret results

| Result | Meaning | Action |
|--------|---------|--------|
| `[PASS]` | Check passed | Continue |
| `[FAIL]` | Check failed | **STOP.** Fix the failure before proceeding |
| `[SKIP]` | Check not applicable | OK — document reason if non-obvious |

### Step 4 — Handle failures

If any check FAILs:

1. **Read the failure message** — it tells you what's wrong
2. **Fix the root cause** — don't work around it
3. **Re-run the same phase** — all checks must pass
4. **Only then proceed** to the next phase

### Step 5 — Retro gate (after epic)

When the retro gate triggers:

1. Run `./scripts/harness-check.sh retro --epic <N>`
2. Review the metrics output
3. Create `docs/evidence/retro-epic-{N}.md` with:
   - Metrics (PR cycles, CI fails, review-gate fail count)
   - Pattern analysis (recurring error types)
   - Root cause (why patterns repeat)
   - Harness rule updates (proposed changes)
   - Applied changes (commits updating HARNESS.md)
4. **User must approve** any harness rule changes before applying
5. Update `docs/HARNESS.md` and/or `docs/HARNESS_GATES.md` if approved

## Automatic Trigger Points

The orchestrator MUST invoke this skill at these points:

| Trigger | Gate |
|---------|------|
| `git checkout -b <feature-branch>` | ① PRE-WORK |
| Story marked complete in todo | ② IN-PROGRESS (includes 2.4 self-review) |
| `git push` (code ready for PR) | ② IN-PROGRESS (2.4 prerequisite) |
| `gh pr create` or before merge | ③ PRE-MERGE |
| PR merge confirmed | ④ POST-MERGE |
| Agent about to start next feature | ⑤ NEXT-READY |
| Last story of epic complete | ⑥ RETRO-GATE |

## smoke:e2e Gate (MANDATORY for user-feature and bug-fix)

For any story that adds or changes a REST API endpoint, the agent MUST run a
real E2E smoke test before claiming the story is complete.

### When required

- Change type = **user-feature** or **bug-fix** → MUST run smoke:e2e
- Change type = infrastructure/refactor/docs/deps → SKIP (document reason)

### How to run

```bash
# 1. Build binary
go build -o ./bin/nano-brain ./cmd/nano-brain/

# 2. Start server on non-default port
NANO_BRAIN_DATABASE_URL="postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev?sslmode=disable" \
NANO_BRAIN_SERVER_PORT=3199 \
NANO_BRAIN_EMBEDDING_PROVIDER="" \
./bin/nano-brain &
SERVER_PID=$!

# 3. Wait for health
for i in $(seq 1 15); do curl -sf http://localhost:3199/health >/dev/null && break; sleep 1; done

# 4. Exercise the changed endpoints via curl
#    - Verify HTTP status codes (200, 201, 400, etc.)
#    - Verify response JSON structure (required fields present)
#    - If bug fix: verify the previously broken request now works

# 5. Kill server
kill $SERVER_PID; wait $SERVER_PID 2>/dev/null
```

### Evidence

Paste curl commands + responses in the PR description or story evidence.
**If a bug is found during smoke:e2e** → fix it BEFORE continuing to the next story.

### FAIL conditions

- Server doesn't start → FAIL
- Health endpoint doesn't respond within 15s → FAIL
- Changed endpoint returns wrong status code → FAIL
- Response JSON missing required fields → FAIL
- Server panics on any request → FAIL (critical bug)

## Reference

| File | What it covers |
|---|---|
| `docs/HARNESS.md` | Master process spec (mental model, forbidden practices, Two-Output Model, HUMAN-ONLY rules) |
| `docs/HARNESS_GATES.md` | Gate specification (6 phases × all checks, parallelism rules, verdict-based PR comment triage) |
| `docs/FEATURE_INTAKE.md` | Risk classification, lane assignment, hard gates, R89 skip-issue conditions |
| `docs/GLOSSARY.md` | Closed-set vocabulary (Lane, Verdict, Override, etc.) |
| `docs/CONTEXT_RULES.md` | What to read per phase × lane (token-budget rules) |
| `docs/TRACE_SPEC.md` | Evidence file formats per tier (Tier 1/2/3) |
| `docs/decisions/README.md` | When to write ADRs |
| `docs/evidence/` | Self-review, review, retro, smoke-e2e evidence files |
| `scripts/harness-check.sh` | Gate enforcement script (source of truth for PASS/FAIL logic) |

## Rule IDs

Many gate FAIL messages reference rule IDs like `(R7)`, `(R29)`, `(R89)`. These
trace back to specific rules in `docs/HARNESS.md` and `docs/HARNESS_GATES.md`.
When a gate FAILs with a rule ID, read that rule before fixing — the rule
defines the exact PASS condition.

Current explicit rules with IDs:
- **R1** — 1 PR = 1 issue (gate 3.8)
- **R7** — `[HARNESS-OVERRIDE]: <reason>` mechanism (gate 3.6)
- **R19, R20** — smoke:e2e evidence required (gate 3.12)
- **R27** — Review Verdict: PASS literal required (gate 3.5)
- **R28** — Archive blocked without Review Verdict: PASS (gate 4.3)
- **R29** — Max 3 PR commits (gate 3.11)
- **R31** — Agent-triaged Gemini verdicts (gate 3.6)
- **R56** — Verdict-based (no effort threshold)
- **R88** — Independent review: review by a spawned sub-agent; author ≠ reviewer (gate 3.5)
- **R89** — Measurable skip-issue conditions (gate 1.3)
- **R90** — Gate 1.1 blocks only on file overlap with the open PR; orthogonal PRs run in parallel
- **R91** — Differential quality gates: 3.3 vs `docs/harness-baseline.txt`, 3.4 via `--new-from-rev` (PR must not make master worse)
- **R92** — Change-type from diff: docs-only skips 3.10+3.12, test-only skips 3.12
