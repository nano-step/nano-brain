# Harness Runner Contract

> **Status: legacy interface.** The autonomous gate-loop *plugin* that consumed
> this contract was OpenCode-only (`.opencode/plugin/harness-loop/`, `/harness-on`)
> and is no longer the driver. On Claude Code, gates are driven by the
> `/harness-gsd` command (autonomous pipeline) or the `harness-check` skill
> (manual runs) — see `docs/HARNESS.md` § Entry points. This document remains
> the authoritative spec for `scripts/harness-check.sh --json` output, which
> any driver (hook, CI, future plugin) can consume. "Plugin" below refers to
> the legacy OpenCode consumer.

Any script or binary can serve as a harness runner. This document defines the contract a driver expects.

## Invocation

```
<runner_path> <gate-name> [--json] [--feature=<id>] [--force]
```

| Argument | Required | Description |
|----------|----------|-------------|
| `<gate-name>` | yes | The gate being checked (e.g., `pre-work`, `pre-merge`) |
| `--json` | no | When present, the runner MUST write exactly one JSON object to stdout and nothing else |
| `--feature=<id>` | no | Feature or story identifier for context |
| `--force` | no | Bypass cached results and re-run all checks |

The runner MUST accept unknown flags gracefully (ignore them).

## Output

When `--json` is passed, stdout MUST contain exactly one JSON object matching the schema below. Non-JSON content on stdout is a contract violation.

Logs, warnings, and diagnostics go to **stderr** only.

### Schema

```json
{
  "gate":                  "<string: gate name matching the request>",
  "status":                "<PASS | FAIL | SKIP | WAITING | BLOCKED | ERROR>",
  "checks":                [ ... ],
  "next_gate":             "<string | null>",
  "instructions_for_agent":"<string>",
  "wait_seconds":          60,
  "rule_ids_violated":     ["R29", "R31"]
}
```

#### Field rules

| Field | Required | Notes |
|-------|----------|-------|
| `gate` | always | Must match the gate name in the request. Mismatch → plugin synthesizes ERROR. |
| `status` | always | One of the six status values. |
| `checks` | always | Array (may be empty). Each check has `id`, `name`, `status` (PASS/FAIL/SKIP), and optional `rule_id` and `message`. |
| `next_gate` | recommended | The gate that follows this one. Omit or `null` on the last gate. Plugin falls back to config order. |
| `instructions_for_agent` | required on FAIL / BLOCKED | Imperative instructions the agent will act on. Omit on PASS/SKIP/WAITING. |
| `wait_seconds` | required on WAITING | Seconds to wait before the plugin retries. |
| `rule_ids_violated` | always | Empty array `[]` when status is PASS/SKIP. Populated with rule IDs on FAIL. |

Unknown fields are rejected by the plugin's Zod schema (strict mode). Do not add extra fields.

#### Check object schema

```json
{
  "id":      "1.1",
  "name":    "GitHub issue exists",
  "status":  "PASS",
  "rule_id": "R89",
  "message": "optional human-readable detail"
}
```

`rule_id` and `message` are optional. `status` is one of `PASS`, `FAIL`, `SKIP`.

## Exit Codes

Exit code is treated as advisory — the plugin trusts the JSON `status` field as authoritative. A mismatch is logged as a warning but does not stop the loop.

| Exit code | Meaning |
|-----------|---------|
| 0 | PASS |
| 1 | FAIL |
| 2 | SKIP |
| 3 | WAITING |
| 4 | BLOCKED |
| 5 | ERROR |

## Status Semantics

| Status | Meaning | Plugin Action |
|--------|---------|---------------|
| PASS | All checks passed | Transition to `next_gate` (or complete the loop) |
| FAIL | One or more checks failed | Inject `instructions_for_agent` as a continuation prompt |
| SKIP | Gate does not apply in current context | Advance to `next_gate` silently |
| WAITING | External system not yet ready | Sleep `wait_seconds` and retry the same gate |
| BLOCKED | Human intervention required | Pause loop, inject `instructions_for_agent`, ask user |
| ERROR | Runner internal error or contract violation | Terminate loop with error toast |

## Stdout Constraint

When `--json` is passed:

- Stdout MUST contain exactly one JSON object.
- No text before or after the JSON object.
- No ANSI escape codes on stdout.
- All diagnostic output goes to stderr.

Violations:
- Multiple JSON objects → ERROR (plugin cannot parse)
- Non-JSON prefix/suffix → ERROR
- Color codes mixed into stdout → parse failure

## Async Gates

For gates that poll external systems (CI pipelines, npm publish, Kubernetes deploys):

1. Return WAITING with `wait_seconds` while polling.
2. Return PASS/FAIL when the external system reaches a terminal state.

When `async: true` is set in the plugin config for a gate, the plugin spawns a background subagent that runs the runner in a poll loop, freeing the session. The subagent returns the final PASS/FAIL output once available.

The runner itself does not need to know about the async flag — it just returns WAITING or a terminal status as usual.

## Example: Minimal Bash Runner

```bash
#!/bin/bash
set -euo pipefail

GATE="$1"
shift

JSON_OUTPUT=false
for arg in "$@"; do
  [[ "$arg" == "--json" ]] && JSON_OUTPUT=true
done

# Run your checks here...
status="PASS"
checks='[]'

if [[ "$JSON_OUTPUT" == true ]]; then
  printf '{"gate":"%s","status":"%s","checks":%s,"next_gate":null,"rule_ids_violated":[]}\n' \
    "$GATE" "$status" "$checks"
fi

[[ "$status" == "PASS" ]] && exit 0 || exit 1
```

## Example: FAIL with instructions

```json
{
  "gate": "pre-work",
  "status": "FAIL",
  "checks": [
    {"id": "1.1", "name": "GitHub issue exists", "status": "FAIL", "rule_id": "R89"}
  ],
  "next_gate": "in-progress",
  "instructions_for_agent": "Create a GitHub issue before starting work: `gh issue create --repo nano-step/nano-brain --title '...' --label enhancement`. Then re-run /harness-on.",
  "rule_ids_violated": ["R89"]
}
```

## Validation

Run the runner pre-flight check before starting the loop:

```bash
./scripts/harness-check.sh --validate
```

This verifies the runner is executable and produces valid JSON for each configured gate.
