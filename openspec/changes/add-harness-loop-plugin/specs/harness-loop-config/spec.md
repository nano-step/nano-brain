## ADDED Requirements

### Requirement: Config SHALL live at a well-known path with documented schema

The project config SHALL be at `.opencode/harness.config.json` in the project root, matching a documented JSON schema. The plugin SHALL load it on every `/harness-on` invocation (no daemon-style caching across invocations).

#### Scenario: Config file present and valid
- **WHEN** `/harness-on` invoked and `.opencode/harness.config.json` exists with valid schema
- **THEN** the plugin SHALL load it, validate against the schema, and use it for loop initialization

#### Scenario: Config file missing
- **WHEN** `/harness-on` invoked and `.opencode/harness.config.json` does not exist
- **THEN** the plugin SHALL refuse to start, print "No harness.config.json found — create one at .opencode/harness.config.json (see docs/HARNESS_RUNNER_CONTRACT.md)", and NOT modify state

#### Scenario: Config file present but invalid JSON
- **WHEN** the config file has invalid JSON
- **THEN** the plugin SHALL refuse to start, print "harness.config.json parse error: <line:col>"

#### Scenario: Config file present but fails schema validation
- **WHEN** the config has valid JSON but missing required fields (e.g., no `runner_path`)
- **THEN** the plugin SHALL refuse to start, print "harness.config.json schema error: <field> is required"

### Requirement: Config schema SHALL define runner, gates, policies, and overrides

The config schema SHALL accept these fields:

```jsonc
{
  "enabled": "boolean (default: true)",
  "runner_path": "string (required, relative to project root)",
  "gates": ["string"],
  "completion_promise": "string (default: 'HARNESS-COMPLETE')",
  "max_total_iterations": "number (default: 100, min: 1, max: 1000)",
  "max_iterations_per_gate": "number (default: 10, min: 1, max: 100)",
  "cache_ttl_minutes": "number (default: 30, min: 0, max: 1440)",
  "fail_policy": "auto | hybrid | ask (default: hybrid)",
  "auto_fix_attempts": "number (default: 3, used only when fail_policy=hybrid)",
  "runner_timeout_seconds": "number (default: 300, min: 10, max: 3600)",
  "state_file_path": "string (default: '.opencode/harness-loop.local.json')",
  "ultrawork_verify_gates": ["string"],
  "skip_gates": ["string"],
  "rule_id_format": "string (default: '{id}', e.g., 'R{id}' or 'FP #{id}')",
  "strict_instructions": "boolean (default: false) — when true, missing doc files block loop start",
  "gate_instructions": {
    "<gate-name>": {
      "doc": "string (optional, project-root-relative path to a markdown protocol doc)",
      "skills": ["string (optional OpenCode skill names to load before fixing)"],
      "async": "boolean (default: false) — when true, gate runs via background watcher subagent",
      "async_max_wait_seconds": "number (default: 1800, min: 60, max: 7200) — outer wall-clock cap",
      "async_poll_interval_seconds": "number (default: 60, min: 10, max: 600) — interval between watcher polls",
      "async_subagent_type": "string (default: 'quick') — subagent_type to spawn for the watcher",
      "async_heartbeats": "boolean (default: true) — emit periodic 'still waiting' toasts"
    }
  },
  "phase_hooks": {
    "<gate-name>": {
      "before": "string (optional shell command)",
      "after": "string (optional shell command)"
    }
  }
}
```

#### Scenario: Minimal valid config
- **WHEN** config contains only `{"runner_path": "./scripts/harness-check.sh", "gates": ["pre-work", "pre-merge"]}`
- **THEN** the plugin SHALL apply all default values for missing fields and start successfully

#### Scenario: gates array empty
- **WHEN** config has `gates: []`
- **THEN** the plugin SHALL refuse to start, print "harness.config.json: gates array must contain at least one gate"

#### Scenario: skip_gates references unknown gate
- **WHEN** config has `gates: ["pre-work", "pre-merge"]` and `skip_gates: ["unknown-gate"]`
- **THEN** the plugin SHALL log a warning but proceed (skip_gates can have forward-compat entries)

#### Scenario: ultrawork_verify_gates references unknown gate
- **WHEN** config lists `ultrawork_verify_gates: ["non-existent"]`
- **THEN** the plugin SHALL log a warning and ignore unknown entries

#### Scenario: gate_instructions references an unknown gate
- **WHEN** config has `gates: ["pre-work", "pre-merge"]` and `gate_instructions: {"unknown-gate": {"doc": "x.md"}}`
- **THEN** the plugin SHALL log a warning "gate_instructions references unknown gate 'unknown-gate'" but SHALL proceed; the orphan entry has no effect

#### Scenario: gate_instructions doc path is non-string
- **WHEN** config has `gate_instructions: {"e2e": {"doc": 42}}`
- **THEN** the plugin SHALL fail schema validation and refuse to start, printing "harness.config.json schema error: gate_instructions.e2e.doc must be a string"

#### Scenario: gate_instructions skills contains non-string entry
- **WHEN** config has `gate_instructions: {"e2e": {"skills": ["valid-skill", 42]}}`
- **THEN** the plugin SHALL fail schema validation; all entries in skills MUST be strings

#### Scenario: gate_instructions async fields with defaults
- **WHEN** config sets `gate_instructions: {"post-merge-npm-release": {"async": true}}` with no other async fields
- **THEN** the plugin SHALL apply defaults `async_max_wait_seconds=1800`, `async_poll_interval_seconds=60`, `async_subagent_type="quick"`, `async_heartbeats=true`

#### Scenario: async_max_wait_seconds out of bounds
- **WHEN** config sets `async_max_wait_seconds: 10000` (above 7200 max)
- **THEN** the plugin SHALL fail schema validation, print "harness.config.json schema error: async_max_wait_seconds must be in range [60, 7200]"

#### Scenario: async_poll_interval_seconds exceeds max_wait
- **WHEN** config sets `async_max_wait_seconds: 60` and `async_poll_interval_seconds: 120`
- **THEN** the plugin SHALL fail schema validation, print "async_poll_interval_seconds must be ≤ async_max_wait_seconds (would never poll)"

#### Scenario: async false explicitly (the synchronous default)
- **WHEN** config sets `gate_instructions: {"pre-work": {"async": false}}`
- **THEN** the plugin SHALL treat the gate as synchronous (identical to omitting the `async` field entirely); no watcher subagent spawned

#### Scenario: async true on a gate that also has phase_hooks
- **WHEN** config sets `async: true` for gate "X" and `phase_hooks.X.before: "./prep.sh"` and `phase_hooks.X.after: "./done.sh"`
- **THEN** the plugin SHALL run `phase_hooks.X.before` in the main session before spawning the watcher, and SHALL run `phase_hooks.X.after` in the main session after the watcher returns PASS (no special async handling for hooks themselves — they remain main-session synchronous)

### Requirement: Config SHALL support per-run overrides via a separate file

The plugin SHALL recognize `.opencode/harness.override.json` (gitignored) as a one-shot layer applied on top of the base config for the next `/harness-on` invocation only.

#### Scenario: Override file present
- **WHEN** both `.opencode/harness.config.json` and `.opencode/harness.override.json` exist
- **THEN** the plugin SHALL merge override fields on top of base config (override wins per-field), then auto-delete the override file after loop initialization

#### Scenario: Override file deleted on loop end
- **WHEN** a loop started with an override file completes (any termination)
- **THEN** the override file SHALL be deleted from disk if it still exists (defensive cleanup)

#### Scenario: Override has invalid schema
- **WHEN** override file has invalid JSON or fails schema
- **THEN** the plugin SHALL print "harness.override.json invalid — ignoring" and proceed with base config only

### Requirement: Config layering SHALL follow strict precedence

When the same field is set in multiple layers, the plugin SHALL resolve per this precedence (highest wins): CLI args > override file > project config > plugin defaults.

#### Scenario: CLI arg overrides config
- **WHEN** project config has `max_total_iterations: 100` and user invokes `/harness-on --max-iter=50`
- **THEN** the effective value SHALL be 50

#### Scenario: Override file overrides project config
- **WHEN** project config has `fail_policy: "hybrid"` and override file has `fail_policy: "auto"`
- **THEN** the effective value SHALL be "auto"

#### Scenario: Project config overrides plugin default
- **WHEN** plugin default for `cache_ttl_minutes` is 30 and project config sets it to 10
- **THEN** the effective value SHALL be 10

### Requirement: Config snapshot SHALL be embedded in state file

The plugin SHALL freeze the merged effective config into `loop.config_snapshot` at loop start, so subsequent iterations use the same config even if the user edits files mid-loop.

#### Scenario: Config edited mid-loop
- **WHEN** loop is active and user edits `.opencode/harness.config.json` to change `max_total_iterations`
- **THEN** the current loop SHALL continue using the snapshotted value; next `/harness-on` invocation picks up the new value

#### Scenario: Config snapshot includes resolved layering
- **WHEN** loop starts with CLI arg `--max-iter=50` overriding config `max_total_iterations: 100`
- **THEN** `loop.config_snapshot.max_total_iterations` SHALL be 50 (the resolved effective value, not the pre-merge config value)

### Requirement: Config SHALL document rule ID format for cross-project compat

The `rule_id_format` field SHALL be a template string with `{id}` placeholder, used to format raw rule IDs into project-canonical references in continuation prompts.

#### Scenario: Nano-brain format
- **WHEN** config has `rule_id_format: "R{id}"` and runner emits `rule_ids_violated: ["29", "89"]`
- **THEN** the continuation prompt SHALL reference them as "R29, R89"

#### Scenario: Capyhome format
- **WHEN** config has `rule_id_format: "FP #{id}"` and runner emits `rule_ids_violated: ["37", "21"]`
- **THEN** the continuation prompt SHALL reference them as "FP #37, FP #21"

#### Scenario: Runner pre-formats IDs
- **WHEN** runner emits already-formatted IDs like `rule_ids_violated: ["R29", "FP #37"]`
- **THEN** the plugin SHALL detect that IDs already contain non-numeric characters and bypass formatting (heuristic: if any ID contains a character other than digits, use as-is)

### Requirement: Phase hooks SHALL run before/after gate execution

If `phase_hooks.<gate>.before` is set, the plugin SHALL invoke that shell command before calling the runner for that gate. Same for `.after` after a successful gate PASS.

#### Scenario: before hook runs
- **WHEN** config has `phase_hooks.pre-merge.before: "./scripts/lint.sh"`
- **THEN** the plugin SHALL spawn that command synchronously before invoking the runner for pre-merge; if the hook exits non-zero, the gate is treated as FAIL with stderr captured in instructions

#### Scenario: after hook runs on PASS
- **WHEN** config has `phase_hooks.pre-merge.after: "./scripts/notify-slack.sh"` and runner returns PASS
- **THEN** the plugin SHALL spawn the after hook asynchronously (do not block loop on after-hook completion); hook failures are logged but not blocking

#### Scenario: after hook does not run on FAIL
- **WHEN** runner returns FAIL for pre-merge
- **THEN** the after hook SHALL NOT run
