# Gate: async-pr-review

Async gate that waits for the asynchronous PR review cycle to complete — bot review (e.g. Gemini), findings triaged and resolved, CI checks green.

## How This Gate Works

This is an **async gate** — the harness spawns a background watcher subagent that polls until the review cycle reaches a terminal state (PASS / FAIL / BLOCKED) or the watcher times out (max 15 minutes). The main session stays idle while the watcher polls; on FAIL the watcher injects fix instructions into the agent's next turn.

Expected timeline:
- **Typical**: 1-3 minutes (Gemini reviews quickly on small PRs)
- **Slow**: 10-15 minutes (large diffs, queue contention)
- **Timeout**: 15 minutes (Gemini may be offline / workflow misconfigured)

## Position in Gate Sequence

```
pre-work → in-progress → pre-merge → async-pr-review → post-merge → post-merge-npm-release → next-ready
```

- `pre-merge` answers "is the code locally valid?" (build, test, lint, review evidence). All synchronous.
- `async-pr-review` answers "has the external review loop completed?" (bot review, findings resolved, CI green). Inherently async.
- `post-merge` answers "did the merge succeed?" (merged state, issue closed, OpenSpec archived).

## Checks (5 checks, early-exit on terminal conditions)

| ID | Check | PASS | FAIL | WAITING | BLOCKED |
|-----|-------|------|------|---------|---------|
| 3.5.1 | PR state | OPEN | CLOSED (not merged) | — | DRAFT |
| 3.5.2 | Mergeable | MERGEABLE | — | UNKNOWN (GitHub computing) | CONFLICTING |
| 3.5.3 | Bot review posted | ≥1 review event, OR ≥3min since last push with 0 events | — | <3min, 0 events | — |
| 3.5.4 | Findings resolved | All VALID:critical/high have fix commit, OR `[HARNESS-OVERRIDE]` PR comment | Unresolved finding | — | — |
| 3.5.5 | CI status | All checks pass | Any check fails | Checks in progress | — |

**Early exit:** if PR state is `MERGED`, the gate returns PASS with `next_gate: "post-merge"` (human merged externally; skip to post-merge cleanly).

## Bot Detection

Check 3.5.3 looks for review events from authors whose login matches `gemini|copilot|coderabbit` (case-insensitive). Repos without bot review configured naturally PASS this check after the 3-minute soak time elapses.

## Verification Procedure

```bash
# Manual run for debugging
./scripts/check-pr-review.sh async-pr-review --json | python3 -m json.tool

# What the JSON contract emits:
# - status: PASS | FAIL | WAITING | BLOCKED | SKIP
# - checks: array of per-check results
# - wait_seconds: polling hint (when WAITING)
# - instructions_for_agent: fix prompt (when FAIL / BLOCKED)
# - next_gate: "post-merge" on PASS (or "post-merge" if early-exit on MERGED)
```

## PASS Conditions

- PR exists, OPEN (or already MERGED — early exit)
- No merge conflicts (`MERGEABLE` from `gh pr view --json mergeable`)
- Bot review posted, OR 3-minute soak time elapsed with no review
- All VALID:critical/high findings have `fixed in commit <sha>` in the triage file, OR a PR comment contains `[HARNESS-OVERRIDE]`
- All CI checks have `conclusion: SUCCESS`

## WAITING Conditions

- `mergeable: UNKNOWN` — GitHub still computing
- Bot review hasn't posted yet AND <3 minutes since last push
- Any CI check in `IN_PROGRESS` / `QUEUED` / `PENDING`

The watcher absorbs WAITING internally by sleeping `async_poll_interval_seconds` (30s) and re-polling. The main harness loop never sees WAITING directly.

## FAIL Conditions

- No PR found on current branch → push branch and `gh pr create`
- PR state `CLOSED` (not merged)
- Unresolved VALID:critical/high finding in triage table → fix code, append `fixed in commit <sha>` to row, push
- Bot review exists but no triage file under `docs/evidence/` → run code-review skill
- Any CI check `FAILURE` / `CANCELLED` / `TIMED_OUT` / `ACTION_REQUIRED`

On FAIL the watcher returns to the main loop with `instructions_for_agent` populated. Agent fixes + pushes. Next session-idle tick re-enters the gate; a fresh watcher polls for bot re-review of the new commit.

## BLOCKED Conditions

- PR is in DRAFT state → mark ready: `gh pr ready <number>`
- PR has merge conflicts → rebase: `git fetch origin && git rebase origin/master`

BLOCKED pauses the harness loop and surfaces the issue to the human / agent for explicit resolution (rebase / un-draft are actions outside the fix-and-push cycle).

## Override

If review feedback is judged invalid or environmental, the agent can post a PR comment containing `[HARNESS-OVERRIDE]: <reason>` and check 3.5.4 will treat all findings as resolved. The comment is preserved as an audit trail.

## Configuration

`.opencode/harness.config.json`:
```json
"async-pr-review": {
  "doc": "docs/harness/gates/async-pr-review.md",
  "skills": ["code-review"],
  "async": true,
  "async_max_wait_seconds": 900,
  "async_poll_interval_seconds": 30,
  "async_subagent_type": "quick"
}
```

| Knob | Default | Tune for |
|---|---|---|
| `async_max_wait_seconds` | 900 (15min) | Larger PRs may need higher (up to ~1800) |
| `async_poll_interval_seconds` | 30 | Reduce to 15 for responsive feedback in fast-iteration workflows |
| `async_subagent_type` | `quick` | Use `explore` if extending checks to require codebase reasoning |

## Cross-reference

- Plugin async dispatch: `.opencode/plugin/harness-loop/harness-loop-event-handler.ts:241`
- Watcher prompt builder: `.opencode/plugin/harness-loop/async-watcher-spawner.ts:11`
- Runner contract types: `.opencode/plugin/harness-loop/types.ts:36-65`
- Runner script: `scripts/check-pr-review.sh`
- Companion async gate: [post-merge-npm-release.md](post-merge-npm-release.md)
