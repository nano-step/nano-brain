## ADDED Requirements

### Requirement: Plugin SHALL register two slash commands

The plugin SHALL expose `/harness-on` and `/harness-off` as slash commands available in any OpenCode session where the plugin is installed.

#### Scenario: User invokes /harness-on with no active loop
- **WHEN** the user types `/harness-on` in a session and no `loop.active=true` state exists
- **THEN** the plugin SHALL load the project config from `.opencode/harness.config.json`, initialize loop state for the first gate in the configured `gates[]` sequence, write `loop.active=true` to the state file, inject the opening prompt template, and emit a toast notification "Harness loop started — gate: <first-gate>"

#### Scenario: User invokes /harness-on with an active loop already running
- **WHEN** the user types `/harness-on` and the state file shows `loop.active=true`
- **THEN** the plugin SHALL print a warning "Loop already active at gate <current> iteration <N>; use /harness-off first to cancel, or wait for completion" and SHALL NOT start a new loop

#### Scenario: User invokes /harness-off during an active loop
- **WHEN** the user types `/harness-off` and `loop.active=true`
- **THEN** the plugin SHALL wait up to 30 seconds for any in-flight runner subprocess to finish, then send SIGKILL if still running, then set `loop.active=false` in the state file, and emit a toast "Harness loop cancelled at gate <current> iteration <N>"

#### Scenario: /harness-off cancels active watcher subagent
- **WHEN** the user types `/harness-off` and a watcher subagent is currently running (`loop.watcher_task_id` is set)
- **THEN** the plugin SHALL call `background_cancel(taskId=loop.watcher_task_id)` before clearing state, emit toast "Harness loop cancelled — watcher subagent cancelled", and clear `loop.watcher_task_id`

#### Scenario: User invokes /harness-off with no active loop
- **WHEN** the user types `/harness-off` and no `loop.active=true` state exists
- **THEN** the plugin SHALL print "No active harness loop" and SHALL NOT modify the state file

### Requirement: Plugin SHALL register a session.idle event hook

The plugin SHALL register a hook on OpenCode's `session.idle` event that drives loop iteration when a loop is active.

#### Scenario: session.idle fires during an active loop with no completion detected
- **WHEN** the agent emits `session.idle` and `loop.active=true` and no completion promise tag was found in transcript since `message_count_at_start` and the current gate is NOT async
- **THEN** the plugin SHALL invoke the configured runner for the current gate, parse JSON output, evaluate status, and if `FAIL` build a continuation prompt embedding `instructions_for_agent`, `rule_ids_violated`, the effective `gate_instructions.<gate>.doc` path (if resolved), and the effective `gate_instructions.<gate>.skills` list (if non-empty), then inject it into the session

#### Scenario: session.idle fires and current gate has async=true
- **WHEN** the agent emits `session.idle` with `loop.active=true` and `gate_instructions.<current_gate>.async === true` and no watcher subagent is currently running for this gate
- **THEN** the plugin SHALL spawn a background watcher subagent (per harness-loop-plugin async watcher requirement), increment `gate_iteration` by exactly 1, record the watcher's task_id in `loop.watcher_task_id`, and SHALL NOT invoke the runner directly from the main session

#### Scenario: session.idle fires and runner returns PASS for the current gate
- **WHEN** `session.idle` fires, runner returns `{status: "PASS", next_gate: "<next>"}` for the current gate
- **THEN** the plugin SHALL update state to set `current_gate=<next>`, increment `total_iteration`, reset `gate_iteration` to 0, and inject a transition prompt "Gate <previous> passed. Now starting gate <next>."

#### Scenario: session.idle fires and runner returns PASS for the final gate
- **WHEN** `session.idle` fires, runner returns `{status: "PASS", next_gate: null}` and `current_gate` is the last entry in `gates[]`
- **THEN** the plugin SHALL set `loop.active=false`, emit toast "✅ Harness loop complete — all gates passed", and SHALL NOT inject further prompts

#### Scenario: session.idle fires and runner returns WAITING
- **WHEN** runner returns `{status: "WAITING", wait_seconds: 60}`
- **THEN** the plugin SHALL sleep `wait_seconds`, re-invoke the same runner for the same gate, and SHALL NOT advance `current_gate`

#### Scenario: session.idle fires and runner returns BLOCKED
- **WHEN** runner returns `{status: "BLOCKED", instructions_for_agent: "<msg>"}`
- **THEN** the plugin SHALL pause the loop (state retains `loop.active=true` but no further iterations), emit toast "🚫 Harness loop blocked at gate <current> — user action required", and inject a message to the agent explaining the block and the override mechanism

### Requirement: Plugin SHALL detect completion via promise tag OR structural signal

The plugin SHALL terminate the loop when either explicit completion signal occurs.

#### Scenario: Agent emits the configured completion promise tag
- **WHEN** the transcript contains a literal `<promise>HARNESS-COMPLETE</promise>` substring (or the configured `completion_promise` value) added after `message_count_at_start`
- **THEN** the plugin SHALL set `loop.active=false`, emit toast "✅ Harness loop ended on completion promise", and SHALL NOT invoke the runner again

#### Scenario: Runner signals structural completion at final gate
- **WHEN** runner returns PASS with `next_gate: null` and the current gate index equals `gates.length - 1`
- **THEN** the plugin SHALL terminate the loop as in the completion-promise case, without requiring the agent to emit the tag

### Requirement: Plugin SHALL enforce anti-stuck guards before injection

The plugin SHALL apply these guards in order before injecting any continuation prompt, and abort the iteration if any guard trips.

#### Scenario: total iteration cap reached
- **WHEN** `total_iteration >= max_total_iterations` (default 100)
- **THEN** the plugin SHALL set `loop.active=false`, emit toast "⛔ Max total iterations reached", and inject a final message stating the cap was hit and listing the last gate + runner output

#### Scenario: per-gate iteration cap reached
- **WHEN** `gate_iteration >= max_iterations_per_gate` (default 10) for the current gate
- **THEN** the plugin SHALL set `loop.active=false`, emit toast "⛔ Max iterations for gate <current> reached", and inject a final message listing the gate name and the last `instructions_for_agent`

#### Scenario: no-progress detected between iterations
- **WHEN** the latest assistant turn since `message_count_at_start` for this iteration emitted zero tokens
- **THEN** the plugin SHALL set `loop.active=false`, emit toast "⛔ Loop stopped — agent made no progress", and inject a final message asking the user to inspect manually

#### Scenario: same rule violated 3+ consecutive iterations
- **WHEN** the same set of `rule_ids_violated` is returned by the runner for 3 iterations in a row on the same gate
- **THEN** the plugin SHALL set `loop.active=false`, emit toast "⛔ Loop stopped — repeated failure on rules <rule_ids>", and inject a message stating the agent appears unable to fix the same rule

#### Scenario: in-flight session lock prevents re-entry
- **WHEN** a `session.idle` event fires while another iteration is mid-execution for the same session
- **THEN** the plugin SHALL skip the duplicate event without injecting

#### Scenario: user typed message within 2-second window
- **WHEN** `session.idle` fires and the most recent message of role=user was created less than 2000ms ago
- **THEN** the plugin SHALL defer iteration and wait for the next `session.idle` event

#### Scenario: background subagent still active
- **WHEN** `session.idle` fires and `backgroundManager.getTasksByParentSession()` returns any task with status `pending` or `running`
- **THEN** the plugin SHALL skip the iteration and wait for the next idle after subagents complete

### Requirement: Plugin SHALL embed per-gate instruction references in continuation prompts

When building a continuation prompt for a FAIL or BLOCKED iteration, the plugin SHALL resolve the gate's instruction doc (configured or via convention fallback) and skills list, and embed them at the top of the prompt above the runner instructions.

#### Scenario: Doc and skills both present
- **WHEN** the current gate has a resolved doc path AND a non-empty skills list
- **THEN** the prompt SHALL contain a "📖 Read project's gate protocol FIRST (mandatory): <path>" block followed by a "🔧 Load skills before attempting fix:" block listing each skill as a bullet, both appearing above the "Runner instructions:" section

#### Scenario: Doc present, no skills
- **WHEN** the current gate has a resolved doc but `skills` is empty or absent
- **THEN** the prompt SHALL include the doc reference block and SHALL NOT include any "Load skills" section

#### Scenario: Skills present, no doc resolved (neither configured nor convention)
- **WHEN** the current gate has skills but no doc path resolves
- **THEN** the prompt SHALL include a warning block "⚠️ No protocol doc found for gate <gate>. Use general best practices." followed by the skills block

#### Scenario: Neither doc nor skills
- **WHEN** the current gate has no configured `gate_instructions` and no convention-path doc exists
- **THEN** the prompt SHALL include the "⚠️ No protocol doc found" warning and SHALL skip the skills block entirely

#### Scenario: Doc path appears verbatim in the prompt
- **WHEN** the resolved doc path is `docs/harness/gates/e2e.md`
- **THEN** the literal string `docs/harness/gates/e2e.md` SHALL appear in the prompt (not URL-escaped, not modified)

### Requirement: Plugin SHALL spawn a background watcher subagent for async gates

When the current gate has `gate_instructions.<gate>.async === true`, the plugin SHALL delegate runner polling to a background subagent of type `gate_instructions.<gate>.async_subagent_type` (default `"quick"`), passing a constrained prompt that polls the runner until terminal status or timeout.

#### Scenario: Watcher spawn on async gate
- **WHEN** the loop reaches a gate with `async: true` and no active watcher exists
- **THEN** the plugin SHALL spawn a subagent via `task(subagent_type=<async_subagent_type>, run_in_background=true, load_skills=[], prompt=<watcher prompt template>)` and store the returned `task_id` in `loop.watcher_task_id`

#### Scenario: Watcher prompt is constrained to bash and JSON output
- **WHEN** building the watcher prompt
- **THEN** the prompt SHALL include the literal CONSTRAINTS block listing: "Do NOT modify any files", "Do NOT spawn other subagents", "Do NOT use any tool other than bash", "Total wall-clock time MUST be bounded by <max_wait>s"; and SHALL specify "OUTPUT FORMAT: Exactly one JSON object matching the RunnerOutput contract. Nothing else."

#### Scenario: Watcher completes with PASS
- **WHEN** the watcher subagent completes and its final output parses as `{status: "PASS", ...}`
- **THEN** the plugin SHALL clear `loop.watcher_task_id`, treat the result as if returned by a synchronous runner invocation, transition to the next gate, and emit the success toast

#### Scenario: Watcher completes with FAIL
- **WHEN** the watcher completes with `{status: "FAIL", instructions_for_agent: "..."}`
- **THEN** the plugin SHALL clear `loop.watcher_task_id`, inject the standard FAIL continuation prompt into the main session (so the agent can fix), and apply the configured `fail_policy`

#### Scenario: Watcher times out (internal watcher timeout)
- **WHEN** the watcher hits its own `async_max_wait_seconds` cap and returns the synthesized `{status: "FAIL", instructions_for_agent: "Watcher timed out after Ns; gate did not reach terminal status", rule_ids_violated: ["watcher-timeout"]}`
- **THEN** the plugin SHALL treat this as a normal FAIL (per the rule above) — no special timeout handling at the plugin level

#### Scenario: Watcher does not return within outer grace window (crash mitigation)
- **WHEN** `async_max_wait_seconds + 30` seconds have elapsed since watcher spawn and no completion notification has been received from the subagent system
- **THEN** the plugin SHALL cancel the watcher via `background_cancel(taskId=loop.watcher_task_id)`, treat the gate as FAIL with synthesized message "Watcher subagent did not return result within outer deadline (likely crashed)", record the task_id in `loop.last_runner_output.diagnostic_task_id` for post-mortem inspection, and apply `fail_policy`



The plugin SHALL pause the loop and surface override requests when the agent emits the literal token `[HARNESS-OVERRIDE]: <reason>` in a message.

#### Scenario: Agent emits override token in a message
- **WHEN** the latest assistant message contains a line matching `[HARNESS-OVERRIDE]: <reason>` (case-sensitive)
- **THEN** the plugin SHALL set `loop.override_active=true`, emit toast "⏸️ Loop paused — override requested: <reason>", and ask the user via the question/prompt tool to "approve override and skip current gate | reject override and continue loop"

#### Scenario: User approves the override
- **WHEN** the user selects "approve override" in response to the pause
- **THEN** the plugin SHALL force the current gate's status to SKIP (with `override_reason` stored in state), advance to the next gate, and resume the loop

#### Scenario: User rejects the override
- **WHEN** the user selects "reject override"
- **THEN** the plugin SHALL clear `loop.override_active`, re-inject the standard continuation prompt for the current gate, and resume the loop

### Requirement: Plugin SHALL emit visible toast notifications on key events

The plugin SHALL call `ctx.client.tui.showToast` (best-effort, swallow errors) on every loop state transition so the user has visibility.

#### Scenario: Loop start
- **WHEN** `loop.active` transitions from false to true
- **THEN** toast "Harness loop started — gate: <first-gate>" appears with variant=info, duration=2000ms

#### Scenario: Gate transition (PASS → next gate)
- **WHEN** the runner returns PASS for the current gate and a next gate exists
- **THEN** toast "Gate <previous> ✅ → gate <next>" appears with variant=info, duration=2000ms

#### Scenario: Iteration progress
- **WHEN** an iteration completes (any status except PASS final gate)
- **THEN** toast "Iter <total>/<max_total> · gate <current> <iter>/<max_per_gate>" appears with variant=info, duration=1500ms

#### Scenario: Loop end on success
- **WHEN** the loop terminates via completion promise or structural completion
- **THEN** toast "✅ Harness loop complete" appears with variant=info, duration=5000ms

#### Scenario: Loop end on guard trip
- **WHEN** the loop terminates via any anti-stuck guard
- **THEN** toast with variant=warning, duration=5000ms naming the specific guard

#### Scenario: Async watcher spawn toast
- **WHEN** a watcher subagent is spawned for an async gate
- **THEN** toast "🕐 Watching gate <name> via background subagent (max <N>min)" appears with variant=info, duration=3000ms

#### Scenario: Async watcher heartbeat toast
- **WHEN** the watcher has been running for `async_max_wait_seconds / 3` (or multiples thereof) and `async_heartbeats=true`
- **THEN** toast "⏳ Still watching <name>... (<elapsed>/<max>min)" appears with variant=info, duration=2000ms; capped at 3 heartbeats per gate to avoid spam

#### Scenario: Async watcher heartbeat suppressed when disabled
- **WHEN** `async_heartbeats=false` in config for a gate
- **THEN** the plugin SHALL emit ONLY the spawn toast and the final-result toast for that gate; no intermediate heartbeats

#### Scenario: Async watcher end-success toast
- **WHEN** the watcher returns PASS
- **THEN** toast "✅ Gate <name> passed (took <elapsed>s)" appears with variant=info, duration=3000ms

#### Scenario: Async watcher end-fail toast
- **WHEN** the watcher returns FAIL (including watcher-timeout)
- **THEN** toast "❌ Gate <name> failed: <short reason from instructions_for_agent, truncated to 80 chars>" appears with variant=warning, duration=5000ms

### Requirement: Plugin SHALL be no-op when no loop state exists

The plugin SHALL impose no measurable overhead on sessions where the user never invokes `/harness-on`.

#### Scenario: session.idle fires with no loop state
- **WHEN** `session.idle` fires and no state file exists or `loop.active=false`
- **THEN** the plugin SHALL return immediately without reading config, invoking runner, or doing any other work beyond a single file-existence check
