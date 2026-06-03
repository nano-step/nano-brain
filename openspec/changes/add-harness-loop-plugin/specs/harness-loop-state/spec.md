## ADDED Requirements

### Requirement: State SHALL persist to a single JSON file at a configurable path

The harness loop state SHALL live in one JSON file (default `.opencode/harness-loop.local.json`, overridable via config `state_file_path`). The file SHALL be in `.gitignore`.

#### Scenario: First-ever loop start with no existing state file
- **WHEN** `/harness-on` is invoked and the state file does not exist
- **THEN** the plugin SHALL create the file with mode 0644, populate the schema with initial values, and atomically write via temp-file-then-rename

#### Scenario: Subsequent loop iteration writes
- **WHEN** any state mutation occurs (iteration increment, gate transition, runner output cache)
- **THEN** the plugin SHALL serialize the entire state object to a `.tmp` sibling file, fsync, then `rename()` over the live file to guarantee atomicity

#### Scenario: State file corrupted on read
- **WHEN** the plugin attempts to read the state file and JSON parsing fails
- **THEN** the plugin SHALL log the error, treat the file as if absent (no active loop), and refuse to start a new loop without explicit user confirmation via the question tool

### Requirement: State schema SHALL include both capyhome-compat checkpoints block and plugin-private loop block

The state file SHALL contain a top-level `checkpoints` object (using capyhome's exact format) and a top-level `loop` object (plugin-private metadata). The plugin SHALL NOT modify fields inside `checkpoints` directly; only the runner writes to it.

#### Scenario: State schema contains both blocks
- **WHEN** the state file is read at any point during an active loop
- **THEN** the parsed object SHALL contain keys `feature_id` (string|null), `issue_number` (number|null), `story` (string|null), `updated_at` (ISO 8601 string), `checkpoints` (object — runner-owned), and `loop` (object — plugin-owned)

#### Scenario: Loop block contains required fields
- **WHEN** `loop.active=true`
- **THEN** the `loop` object SHALL contain `active`, `current_gate`, `gate_iteration`, `total_iteration`, `max_iterations_per_gate`, `max_total_iterations`, `started_at`, `session_id`, `config_snapshot`, `last_runner_output`, `no_progress_count`, `override_active`, and `watcher_task_id` (nullable, set only when an async watcher is currently active for the current gate) fields

#### Scenario: Capyhome's harness-state.py reads the file without error
- **WHEN** capyhome's existing `harness-state.py read --checkpoint <name>` is run against a state file produced by this plugin
- **THEN** the script SHALL succeed and return the checkpoint data (proves backward-compat with capyhome tooling)

### Requirement: State SHALL cache runner results with TTL

The plugin SHALL cache the last runner output per gate in `checkpoints.<gate>.checked_at` and skip re-invocation when the cached result is fresh (PASS and within TTL).

#### Scenario: Cached gate result is fresh
- **WHEN** the plugin needs to invoke gate G, and `checkpoints[G].status == "PASS"` and `now - checked_at < cache_ttl_minutes` (default 30 minutes from config)
- **THEN** the plugin SHALL skip runner invocation, reuse the cached result, and advance to the next gate

#### Scenario: Cached gate result is stale (TTL expired)
- **WHEN** the cached result exists with status PASS but `now - checked_at >= cache_ttl_minutes`
- **THEN** the plugin SHALL invoke the runner fresh and overwrite the cache entry

#### Scenario: Cached gate result is FAIL
- **WHEN** the cached result is `FAIL` or `BLOCKED`, regardless of age
- **THEN** the plugin SHALL invoke the runner (never trust a stale failure to remain a failure)

#### Scenario: User passes --force to /harness-on
- **WHEN** `/harness-on --force` is invoked
- **THEN** the plugin SHALL bypass all cache checks and re-invoke every gate from scratch

### Requirement: State SHALL support crash recovery via resume semantics

If OpenCode crashes or the host reboots mid-loop, restarting OpenCode and invoking `/harness-on` again SHALL resume at the same gate without losing progress.

#### Scenario: Crash mid-iteration, restart, /harness-on
- **WHEN** the state file contains `loop.active=true` from a prior session and the user invokes `/harness-on`
- **THEN** the plugin SHALL detect the existing active state, prompt the user via question tool "Resume loop at gate <current> iteration <N>? [resume | cancel-and-restart | abort]"

#### Scenario: User chooses resume
- **WHEN** user selects "resume" in the recovery prompt
- **THEN** the plugin SHALL re-bind the `loop.session_id` to the current session, re-invoke the runner for `current_gate`, and continue normally

#### Scenario: User chooses cancel-and-restart
- **WHEN** user selects "cancel-and-restart"
- **THEN** the plugin SHALL clear the state file's `loop` block (but preserve `checkpoints` cache), then start a new loop from the first gate

### Requirement: State SHALL track no-progress count and same-error count for anti-stuck

The state SHALL maintain counters that the anti-stuck guards consume.

#### Scenario: No-progress counter increments on zero-token turn
- **WHEN** an iteration completes and the latest assistant turn produced zero tokens
- **THEN** `loop.no_progress_count` SHALL increment by 1

#### Scenario: No-progress counter resets on productive turn
- **WHEN** an iteration completes and the latest assistant turn produced > 0 tokens
- **THEN** `loop.no_progress_count` SHALL reset to 0

#### Scenario: Same-error history tracked per gate
- **WHEN** the runner returns `rule_ids_violated: ["R29"]` for the current gate
- **THEN** the plugin SHALL append the rule_ids array to `loop.same_error_history[current_gate]` (sliding window of last 5 iterations)

#### Scenario: watcher_task_id set on watcher spawn
- **WHEN** the plugin spawns a watcher subagent for an async gate
- **THEN** `loop.watcher_task_id` SHALL be set to the returned subagent task_id

#### Scenario: watcher_task_id cleared on watcher completion or cancellation
- **WHEN** the watcher subagent completes (any status) OR `/harness-off` cancels the watcher OR the outer grace timeout fires
- **THEN** `loop.watcher_task_id` SHALL be cleared to `null`

#### Scenario: watcher_task_id absent when no async gate active
- **WHEN** the loop is on a synchronous gate (no `async: true` in config)
- **THEN** `loop.watcher_task_id` SHALL be `null` (no leftover ids from previous async gates)

#### Scenario: Same-error trigger fires
- **WHEN** the last 3 entries in `loop.same_error_history[current_gate]` are identical and non-empty
- **THEN** the anti-stuck guard SHALL trip (per harness-loop-plugin spec)

### Requirement: State writes SHALL be observable for debugging

A human SHALL be able to inspect the state file with `cat` or `jq` at any time and understand the loop's current position.

#### Scenario: Human inspects state during active loop
- **WHEN** a human runs `cat .opencode/harness-loop.local.json | jq .loop` during an active loop
- **THEN** the output SHALL show all fields with human-readable values: ISO timestamps, gate names, iteration counts, last runner output JSON

#### Scenario: State diff between iterations is small
- **WHEN** consecutive iterations write to the state file
- **THEN** the JSON diff SHALL be limited to: `updated_at`, `loop.gate_iteration`, `loop.total_iteration`, `loop.last_runner_output`, `loop.no_progress_count`, and optionally `loop.same_error_history[<gate>]` — no full-file rewrites of unrelated data
