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

compatibility: OpenCode with bash + git + go. Optional: gh CLI, openspec CLI, golangci-lint.
---

# Harness Check

Enforce the harness gate specification defined in `docs/HARNESS_GATES.md`.

## Core Rules

- **1 feature = 1 PR = 1 GitHub issue**
- **Agent MUST NOT start the next feature until all gates pass**
- **FAIL on any check = BLOCK** — fix before continuing

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

The orchestrator (Sisyphus) MUST invoke this skill at these points:

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

- Gate specification: `docs/HARNESS_GATES.md`
- Harness process: `docs/HARNESS.md`
- Evidence directory: `docs/evidence/`
- Check script: `scripts/harness-check.sh`
