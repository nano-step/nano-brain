# Harness Loop Plugin

Autonomous gate-driven development workflow plugin for OpenCode.

## What It Does

The harness loop plugin automates your development workflow by driving your project through configured gates (build, test, lint, etc.) until all pass or a hard-stop condition triggers. It hooks into OpenCode's `session.idle` event to continuously re-check gates and inject continuation prompts when fixes are needed.

## Quick Start

### 1. Copy the plugin

```bash
# From your project root
cp -r /path/to/nano-brain/.opencode/plugin/harness-loop .opencode/plugin/
```

### 2. Install dependencies

```bash
cd .opencode/plugin/harness-loop
npm install
npm run build
cd ../../..
```

### 3. Create a harness config

Create `.opencode/harness.config.json`:

```json
{
  "runner_path": "./scripts/harness-check.sh",
  "gates": ["pre-work", "in-progress", "pre-merge", "post-merge", "next-ready"],
  "rule_id_format": "R{id}",
  "fail_policy": "hybrid"
}
```

### 4. Implement a runner

Your runner script receives a gate name and must output JSON matching the runner contract:

```bash
#!/bin/bash
# Example: scripts/harness-check.sh

GATE="$1"
shift

# Your gate logic here...

# Output JSON (to stdout)
cat <<EOF
{
  "gate": "$GATE",
  "status": "PASS",
  "checks": [],
  "next_gate": null
}
EOF
```

### 5. Start the loop

```
/harness-on
```

## Configuration Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `runner_path` | string | **required** | Path to runner script (relative to project root) |
| `gates` | string[] | **required** | Ordered list of gate names |
| `fail_policy` | "auto" \| "hybrid" \| "ask" | "hybrid" | How to handle failures |
| `rule_id_format` | string | `"{id}"` | Format string for rule IDs (e.g., `"R{id}"`, `"FP #{id}"`) |
| `max_total_iterations` | number | 100 | Hard cap on total iterations across all gates |
| `max_iterations_per_gate` | number | 10 | Max attempts per gate before escalating |
| `auto_fix_attempts` | number | 3 | In hybrid mode, auto-fix N times before asking user |
| `cache_ttl_minutes` | number | 30 | How long to cache PASS results |
| `runner_timeout_seconds` | number | 300 | Runner subprocess timeout |
| `completion_promise` | string | "HARNESS-COMPLETE" | Promise tag the agent emits to signal completion |
| `ultrawork_verify_gates` | string[] | [] | Gates that require Oracle verification after PASS |
| `state_file_path` | string | ".opencode/harness-loop.local.json" | Where to store loop state |
| `gate_instructions` | object | {} | Per-gate doc paths and skill lists |

### Gate Instructions

Point each gate to project-specific documentation and skills:

```json
{
  "gate_instructions": {
    "pre-merge": {
      "doc": "docs/harness/gates/pre-merge.md",
      "skills": ["review-work"]
    },
    "smoke-e2e": {
      "doc": "docs/harness/gates/smoke-e2e.md",
      "skills": ["playwright"]
    }
  }
}
```

If `doc` is omitted, the plugin tries `docs/harness/gates/<gate>.md` by convention.

### Async Gates

For gates that depend on external systems (CI, deploys, npm publish):

```json
{
  "gate_instructions": {
    "post-merge-npm-release": {
      "doc": "docs/harness/gates/post-merge-npm-release.md",
      "async": true,
      "async_max_wait_seconds": 1800,
      "async_poll_interval_seconds": 60
    }
  }
}
```

## Runner Contract

Your runner must:

1. Accept `<gate-name> [--feature=<id>] [--force] [--json]` as arguments
2. Output exactly one JSON object to stdout
3. Exit with code matching status (0=PASS, 1=FAIL, 2=SKIP, 3=WAITING, 4=BLOCKED, 5=ERROR)

### Output Schema

```json
{
  "gate": "pre-work",
  "status": "PASS | FAIL | SKIP | WAITING | BLOCKED | ERROR",
  "checks": [
    { "id": "1.1", "name": "Issue exists", "status": "PASS", "rule_id": "R89" }
  ],
  "next_gate": "in-progress",
  "instructions_for_agent": "Required when status is FAIL or BLOCKED",
  "wait_seconds": 60,
  "rule_ids_violated": ["R29", "R31"]
}
```

## Commands

| Command | Description |
|---------|-------------|
| `/harness-on` | Start the harness loop |
| `/harness-on --force` | Start fresh, ignoring cached results |
| `/harness-off` | Cancel the active loop |

## How to Adopt in Your Project

1. **Copy the plugin directory** into your project:
   ```bash
   cp -r .opencode/plugin/harness-loop <your-project>/.opencode/plugin/
   ```

2. **Install and build:**
   ```bash
   cd <your-project>/.opencode/plugin/harness-loop
   npm install && npm run build
   cd ../../..
   ```

3. **Create `.opencode/harness.config.json`** (minimal example):
   ```json
   {
     "runner_path": "./scripts/harness-check.sh",
     "gates": ["pre-work", "in-progress", "pre-merge"],
     "fail_policy": "hybrid"
   }
   ```

4. **Write or adapt a runner script** that accepts `<gate> [--json]` and outputs the runner contract JSON.

5. **Start the loop:**
   ```
   /harness-on
   ```

## Override Mechanism

Two mechanisms let you pause the loop when automatic fixing isn't possible:

**Agent token** — add this anywhere in the agent's reply:
```
[HARNESS-OVERRIDE]: <reason for human intervention>
```
The loop pauses and waits for user approval before continuing.

**File override** — create `.opencode/harness.override.json` before running `/harness-on`:
```json
{
  "max_iterations_per_gate": 20,
  "skip_gates": ["e2e"]
}
```
The config is merged once at loop start and the file is auto-deleted when the loop ends.

## Gate Instructions Config

The `gate_instructions` field maps each gate name to a doc path and optional skill list. The plugin reads the doc at loop start and prepends it to the agent's context for that gate.

```json
{
  "gate_instructions": {
    "pre-merge": {
      "doc": "docs/harness/gates/pre-merge.md",
      "skills": ["review-work", "code-review"]
    },
    "smoke-e2e": {
      "doc": "docs/harness/gates/smoke-e2e.md"
    },
    "post-merge": {
      "skills": ["git-master"]
    }
  }
}
```

- **`doc` + `skills`** — explicit doc injected, plus named skills loaded.
- **`doc` only** — doc injected; no extra skills.
- **`skills` only (no `doc`)** — skills loaded; plugin falls back to `docs/harness/gates/<gate>.md` by convention. A warning is logged if that file doesn't exist.

When `doc` is omitted entirely, the convention path is tried silently.

## Authoring Gate Instruction Docs

Gate instruction docs tell the agent exactly what to do for each gate. Use this structure:

```markdown
## Hard Rules
- <rule that must never be violated>

## Step-by-Step Procedure
1. <first action>
2. <second action>

## Evidence Requirements
- Paste output of: <command>

## FAIL Conditions
- <condition that causes an automatic FAIL>
```

Example (`docs/harness/gates/smoke-e2e.md`):

```markdown
## Hard Rules
- Never claim smoke:e2e passes without showing actual curl output.

## Step-by-Step Procedure
1. Build: `go build -o ./bin/nano-brain ./cmd/nano-brain/`
2. Start server on port 3199
3. Wait for GET /health → `{"ready":true}`
4. Exercise changed endpoints with curl
5. Kill the server

## Evidence Requirements
- Paste all curl commands and responses in the PR description.

## FAIL Conditions
- Server fails to start
- Any curl returns non-2xx
- Response JSON is missing required fields
```

## Strict vs Flexible Mode

Set `strict_instructions: true` in `harness.config.json` to require every gate to have an instruction doc before the loop starts:

```json
{
  "strict_instructions": true
}
```

With `strict_instructions: true`, `/harness-on` refuses to start and prints a list of gates missing docs. Fix by adding a `doc` path to `gate_instructions` or creating the convention path file.

The default (`false`) warns about missing docs but continues. Useful when adopting the plugin incrementally.

## Async Gate Semantics

Use `async: true` for gates that depend on external systems where the result isn't available immediately (CI pipelines, npm publish, deploy health checks).

```json
{
  "gate_instructions": {
    "post-merge-release": {
      "doc": "docs/harness/gates/post-merge-release.md",
      "async": true,
      "async_max_wait_seconds": 1800,
      "async_poll_interval_seconds": 60,
      "async_subagent_type": "explore"
    }
  }
}
```

**`async_max_wait_seconds`** — size this at 2-3x your expected CI or deploy time. A release workflow that normally takes 8 minutes should get at least 1800 seconds (30 min) to account for queue delays.

**`async_subagent_type`** — controls which subagent polls the external system. Use `"explore"` for read-only checks (gh CLI, curl). Use `"hephaestus"` when the polling subagent may need to take corrective action.

**Heartbeat toasts** — while waiting, the plugin emits a toast every `async_poll_interval_seconds`: `"⏳ Waiting for post-merge-release (elapsed: 2m30s / max: 30m)"`. These are visible in the OpenCode status bar.

When `async_max_wait_seconds` expires without a terminal result, the gate emits `FAIL` with `instructions_for_agent` set to the timeout message.

## Writing an Async-Aware Runner

For async gates, your runner returns `WAITING` when the external system hasn't settled yet. The plugin re-polls after `wait_seconds`.

```bash
#!/bin/bash
# scripts/harness-check.sh post-merge-release --json

status=$(gh run list --workflow=release.yml --limit 1 --json status --jq '.[0].status')

if [[ "$status" == "in_progress" || "$status" == "queued" ]]; then
  # Not done yet — tell the plugin to wait 60 seconds and re-poll
  echo '{"gate":"post-merge-release","status":"WAITING","checks":[],"wait_seconds":60}'
  exit 3
fi

conclusion=$(gh run list --workflow=release.yml --limit 1 --json conclusion --jq '.[0].conclusion')

if [[ "$conclusion" == "success" ]]; then
  echo '{"gate":"post-merge-release","status":"PASS","checks":[],"next_gate":null}'
  exit 0
else
  echo "{\"gate\":\"post-merge-release\",\"status\":\"FAIL\",\"checks\":[],\"instructions_for_agent\":\"Release workflow failed with conclusion: $conclusion. Check the workflow logs.\"}"
  exit 1
fi
```

Key points:
- Exit code `3` signals `WAITING` to the plugin.
- `wait_seconds` in the JSON body overrides `async_poll_interval_seconds` for that specific poll cycle.
- Terminal statuses (`PASS`, `FAIL`, `SKIP`, `BLOCKED`, `ERROR`) use exit codes 0, 1, 2, 4, 5 respectively.

## License

MIT
