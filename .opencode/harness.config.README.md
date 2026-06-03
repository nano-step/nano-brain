# `.opencode/harness.config.json` Reference

This file configures the harness loop plugin for the nano-brain project. The plugin reads it on every `/harness-on` invocation.

## Fields

### Core (required)

| Field | Type | Description |
|-------|------|-------------|
| `runner_path` | string | Path to the runner script, relative to project root. Must be executable. |
| `gates` | string[] | Ordered list of gate names. The loop runs them left-to-right. |

### Failure Handling

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `fail_policy` | `"auto"` \| `"hybrid"` \| `"ask"` | `"hybrid"` | `auto` = always inject fix prompt; `ask` = always ask human; `hybrid` = auto-fix N times then ask |
| `auto_fix_attempts` | number | 3 | In `hybrid` mode, how many times to auto-inject before switching to ask-user |
| `max_iterations_per_gate` | number | 10 | Hard cap on fix attempts per gate before escalating |
| `max_total_iterations` | number | 100 | Hard cap across all gates combined |

### Caching

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `cache_ttl_minutes` | number | 30 | How long a PASS checkpoint stays fresh. The plugin skips re-running a gate if its last PASS is within this window. |

### Runner Execution

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `runner_timeout_seconds` | number | 300 | Subprocess kill timeout. Runner gets SIGTERM then SIGKILL after 5s grace. |
| `rule_id_format` | string | `"{id}"` | Format string for rule ID display. `{id}` is replaced with the raw rule ID. Examples: `"R{id}"` → `R89`, `"FP #{id}"` → `FP #42` |

### Completion

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `completion_promise` | string | `"HARNESS-COMPLETE"` | The agent emits `<promise>HARNESS-COMPLETE</promise>` in any reply to signal the loop is done. Change this if you use a different token. |
| `ultrawork_verify_gates` | string[] | `[]` | Gates that require Oracle (ultrawork) verification after PASS before the loop advances. Use for high-stakes gates like `pre-merge`. |

### State

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `state_file_path` | string | `.opencode/harness-loop.local.json` | Where the loop stores its state. Gitignored. |

### Per-Gate Instructions (`gate_instructions`)

Each key is a gate name. Fields:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `doc` | string | (convention fallback) | Path to the gate instruction doc, relative to project root. If omitted, the plugin tries `docs/harness/gates/<gate>.md`. |
| `skills` | string[] | `[]` | OpenCode skills to load in the continuation prompt for this gate. |
| `async` | boolean | `false` | When true, the plugin spawns a background subagent to poll the runner instead of blocking the session. |
| `async_max_wait_seconds` | number | 1800 | Maximum time (seconds) the background watcher will wait for a terminal status. |
| `async_poll_interval_seconds` | number | 60 | How often the watcher re-runs the runner while it returns WAITING. |
| `async_subagent_type` | string | `"quick"` | Category of the background subagent. Use `"quick"` for polling-only tasks. |

### Async Heartbeats

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `async_heartbeats` | boolean | `true` | When true, the plugin emits a toast every `async_max_wait_seconds / 3` seconds while a watcher is active, so you know the loop is alive. Set to `false` to suppress. |

## One-Shot Overrides

Create `.opencode/harness.override.json` with any subset of fields before running `/harness-on`. The file is merged on top of `harness.config.json` and then **automatically deleted** — it applies only once.

Useful for: temporarily increasing `max_iterations_per_gate`, skipping a gate, switching `fail_policy` to `auto` for a batch run.

## Nano-brain Config (this file)

```json
{
  "runner_path": "./scripts/harness-check.sh",
  "gates": ["pre-work", "in-progress", "pre-merge", "post-merge", "post-merge-npm-release", "next-ready"],
  "rule_id_format": "R{id}",
  "fail_policy": "hybrid",
  "ultrawork_verify_gates": ["pre-merge"],
  "gate_instructions": {
    "post-merge-npm-release": {
      "async": true,
      "async_max_wait_seconds": 1800,
      "async_poll_interval_seconds": 60
    }
  }
}
```

`post-merge-npm-release` is async because the GitHub Actions release pipeline takes 3–30 minutes. The plugin spawns a background subagent that polls `./scripts/check-npm-release.sh` until it returns PASS or FAIL.
