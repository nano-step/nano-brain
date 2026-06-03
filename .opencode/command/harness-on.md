---
description: Start the autonomous harness loop — drives the agent through configured gates until all pass
---

Start the harness loop for the current feature. The loop runs `./scripts/harness-check.sh <gate> --json` for each configured gate and injects fix instructions on failure, continuing until all gates PASS or a hard-stop condition triggers.

**Usage**

```
/harness-on [--force] [--max-iter=N] [--skip-gate=<name>] [--config=<path>]
```

**Flags**

| Flag | Description |
|------|-------------|
| `--force` | Ignore cached PASS results and re-run all gates from scratch |
| `--max-iter=N` | Override `max_total_iterations` for this session |
| `--skip-gate=<name>` | Remove a gate from the run list (repeatable) |
| `--config=<path>` | Load a different `harness.config.json` |

**What happens**

1. Loads `.opencode/harness.config.json` (+ `.opencode/harness.override.json` if present, then deletes it)
2. Validates runner is executable and gate instruction docs exist
3. Seeds state in `.opencode/harness-loop.local.json`
4. Injects the opening prompt so the agent knows the loop is active
5. On every `session.idle` the plugin fires the runner, reads the JSON result, and either:
   - **PASS** → transitions to next gate (or completes the loop)
   - **FAIL / ERROR** → injects a continuation prompt with `instructions_for_agent` + rule IDs
   - **BLOCKED** → pauses and asks for human input
   - **WAITING** → sleeps `wait_seconds` then retries
   - **SKIP** → advances to next gate silently

**Completion**

The agent signals completion by including `<promise>HARNESS-COMPLETE</promise>` in any reply, OR when the last gate returns PASS with `next_gate: null`.

**Override**

If the agent includes `[HARNESS-OVERRIDE]: <reason>` in a reply, the loop pauses for human approval.

**Stop the loop**

Run `/harness-off` at any time to cancel.
