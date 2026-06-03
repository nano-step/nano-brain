## 1. Scaffolding and Project Setup

- [x] 1.1 Create directory structure `.opencode/plugin/harness-loop/` with subdirectories `commands/`, `templates/`, `tests/`
- [x] 1.2 Initialize `.opencode/plugin/harness-loop/package.json` declaring `@opencode-ai/plugin` as peer-dep and Zod as a dep (for runtime schema validation)
- [x] 1.3 Add `tsconfig.json` mirroring the strict-mode config used by `oh-my-opencode/src/hooks/ralph-loop/` (target ES2022, module Node16, strict true, no implicit any)
- [x] 1.4 Add `.gitignore` entries: `.opencode/harness-loop.local.json`, `.opencode/harness.override.json`, `.opencode/plugin/harness-loop/node_modules/`, `.opencode/plugin/harness-loop/dist/`
- [x] 1.5 Create `.opencode/plugin/harness-loop/README.md` with adoption instructions: "copy this directory into your project's `.opencode/plugin/`, write a `harness.config.json`, restart OpenCode"

## 2. Type Definitions and Constants

- [x] 2.1 Implement `types.ts` with TypeScript interfaces matching the specs: `HarnessLoopState`, `LoopMeta`, `RunnerOutput`, `RunnerCheck`, `HarnessConfig`, `ConfigSnapshot`
- [x] 2.2 Implement `constants.ts` with hardcoded defaults: `DEFAULT_MAX_TOTAL_ITERATIONS=100`, `DEFAULT_MAX_PER_GATE=10`, `DEFAULT_CACHE_TTL_MINUTES=30`, `DEFAULT_COMPLETION_PROMISE="HARNESS-COMPLETE"`, `DEFAULT_RUNNER_TIMEOUT_SECONDS=300`, `USER_MESSAGE_IN_PROGRESS_WINDOW_MS=2000`, `IDLE_SETTLE_MS=150`, `OVERRIDE_TOKEN_REGEX=/^\[HARNESS-OVERRIDE\]:\s*(.+)$/m`
- [x] 2.3 Define Zod schemas in `types.ts` for `HarnessConfig` and `RunnerOutput` matching the runner contract spec — strict mode, reject unknown fields

## 3. Config Loader

- [x] 3.1 Implement `config-loader.ts` exporting `loadConfig(projectRoot: string, cliArgs: ConfigOverrides): { config: HarnessConfig, override_consumed: boolean }`
- [x] 3.2 Implement layering precedence: defaults → `.opencode/harness.config.json` → `.opencode/harness.override.json` → CLI args (per harness-loop-config spec)
- [x] 3.3 Validate merged config against Zod schema; on failure, throw `HarnessConfigError` with field-level details
- [x] 3.4 Auto-delete `.opencode/harness.override.json` after successful consumption (defensive cleanup)
- [x] 3.5 Write unit tests in `tests/config-loader.test.ts` covering: missing config file, invalid JSON, schema violation, layering precedence, override file deletion, minimal valid config with defaults

## 4. State Storage Layer

- [x] 4.1 Implement `storage.ts` exporting `readState()`, `writeState(state)`, `clearLoopBlock()` with atomic write semantics (temp file + rename)
- [x] 4.2 Schema enforcement: every write validates against `HarnessLoopState` shape; corrupted file triggers `StateCorruptionError`
- [x] 4.3 Preserve `checkpoints` block on `clearLoopBlock()` (only the `loop` sub-object is cleared)
- [x] 4.4 Ensure file mode 0644 and proper fsync between temp-write and rename
- [x] 4.5 Write unit tests in `tests/storage.test.ts`: round-trip read/write, atomic write under concurrent reads, corruption recovery, capyhome-compat (read a fixture file produced by `harness-state.py`)

## 5. State Controller (Business Logic)

- [x] 5.1 Implement `loop-state-controller.ts` exposing the controller interface: `startLoop()`, `cancelLoop()`, `getState()`, `incrementGateIteration()`, `transitionToGate(next)`, `recordRunnerOutput()`, `incrementNoProgress()`, `resetNoProgress()`, `recordSameErrorHistory(gate, ruleIds)`
- [x] 5.2 Implement `startLoop()`: refuses if state file shows active=true (no double-start); seeds the `loop` block with config snapshot, ISO timestamps, gates[0] as current, iteration=1
- [x] 5.3 Implement `transitionToGate()`: resets `gate_iteration=0`, clears `same_error_history[<previous>]`, increments `total_iteration`
- [x] 5.4 Implement `recordSameErrorHistory()`: sliding window of last 5 entries per gate
- [x] 5.5 Write unit tests in `tests/loop-state-controller.test.ts` covering all state transitions and edge cases (resume from crash, double-start refusal, max-iter handling)

## 6. Runner Invoker

- [x] 6.1 Implement `runner-invoker.ts` exporting `invokeRunner(config, gateName, opts): Promise<RunnerOutput>`
- [x] 6.2 Use `child_process.spawn` with explicit argv (no shell), inherit env, cwd=projectRoot, timeout from config
- [x] 6.3 Capture stdout/stderr separately; on timeout SIGTERM then SIGKILL after 5s grace
- [x] 6.4 Parse stdout strictly: exactly one JSON object, no prefix/suffix; multiple objects = ERROR; non-JSON = ERROR
- [x] 6.5 Validate parsed object with Zod `RunnerOutput` schema; on failure return synthetic `{status: "ERROR", instructions_for_agent: "Runner contract violation: <field details>"}`
- [x] 6.6 Verify gate name in response matches gate name in request; mismatch = ERROR
- [x] 6.7 Log exit-code-vs-status mismatch as warning (trust JSON status as authoritative)
- [x] 6.8 Pre-flight check: runner file exists and is executable; if not, return ERROR without spawning
- [x] 6.9 Write unit tests in `tests/runner-invoker.test.ts`: happy path, timeout, multi-JSON output, non-JSON output, missing required fields, gate mismatch, missing/non-executable runner

## 7. Completion Detector

- [x] 7.1 Implement `completion-detector.ts` exporting `detectCompletion(ctx, state, opts): Promise<"promise_tag" | "structural" | null>`
- [x] 7.2 Scan session transcript via `ctx.client.session.messages` for literal `<promise>${config.completion_promise}</promise>` appearing after `message_count_at_start`
- [x] 7.3 Implement structural completion check: runner returned PASS with `next_gate: null` and current gate is `gates[gates.length - 1]`
- [x] 7.4 Return source enum so the handler knows which signal fired
- [x] 7.5 Write unit tests in `tests/completion-detector.test.ts`: promise tag found, promise tag absent, structural fire on final gate, structural false on non-final gate

## 8. Anti-Stuck Detectors

- [x] 8.1 Implement `no-progress-detector.ts` exporting `latestAssistantTurnMadeNoProgress(ctx, state): Promise<boolean>` (port from Ralph's implementation)
- [x] 8.2 Implement `same-error-detector.ts` exporting `hasRepeatedSameError(state, currentGate): boolean` — checks last 3 entries of `same_error_history[currentGate]` are identical and non-empty
- [x] 8.3 Implement override-detector helper in `completion-detector.ts` or new file: scan latest assistant message for `[HARNESS-OVERRIDE]: <reason>` regex match
- [x] 8.4 Write unit tests in `tests/no-progress-detector.test.ts` and `tests/same-error-detector.test.ts` covering all guard trip conditions

## 9. Continuation Prompt Builder

- [x] 9.1 Implement `templates/continuation-prompt.ts` with the exact template from design.md D10, including `SYSTEM_DIRECTIVE_PREFIX` (copy the constant from Ralph for consistency)
- [x] 9.2 Implement `templates/opening-prompt.ts` for the initial `/harness-on` injection: introduces the loop, lists configured gates, explains completion promise + override mechanism
- [x] 9.3 Implement `continuation-prompt-builder.ts` exporting `buildContinuationPrompt(state, runnerOutput, config): string`
- [x] 9.4 Apply rule-id formatting per config `rule_id_format` (with heuristic for already-formatted IDs)
- [x] 9.5 Truncate `instructions_for_agent` to 8000 chars with "...[truncated]" suffix if exceeded
- [x] 9.6 Add separate template for BLOCKED-status injection (different tone: ask for human input)
- [x] 9.7 Add separate template for ultrawork verification trigger (port from Ralph's `ULTRAWORK_VERIFICATION_PROMPT`) for gates listed in `ultrawork_verify_gates`
- [x] 9.8 Write unit tests in `tests/continuation-prompt-builder.test.ts`: standard FAIL prompt, BLOCKED prompt, rule-id formatting with `R{id}` vs `FP #{id}` vs pre-formatted, truncation, override mention always present

## 9b. Gate Instructions Resolver

- [x] 9b.1 Implement `gate-instructions-resolver.ts` exporting `resolveGateInstructions(config, gateName, projectRoot): { docPath: string | null, skills: string[], warning: string | null }`
- [x] 9b.2 Resolution order: explicit `config.gate_instructions.<gate>.doc` → convention fallback `docs/harness/gates/<gate>.md` → null
- [x] 9b.3 Validate doc file existence with `fs.existsSync`; if missing, populate `warning` field with the missing-doc message
- [x] 9b.4 Resolve skills list: copy from config if present, else empty array
- [x] 9b.5 Add pre-flight validation in `commands/harness-on.ts`: iterate all `gates[]`, call resolver, emit consolidated toast for any missing docs (unless `strict_instructions: true` — then refuse start)
- [x] 9b.6 Add skill registry validation (best-effort): for each gate's skills list, attempt to query OpenCode skill registry; warn on unknown skills, never block
- [x] 9b.7 Update `continuation-prompt-builder.ts` to call resolver and embed doc reference + skills list at the top of the prompt per the harness-gate-instructions spec
- [x] 9b.8 Update `templates/continuation-prompt.ts` to include the new sections: `📖 Read project's gate protocol FIRST` and `🔧 Load skills before attempting fix`
- [x] 9b.9 Handle the four cases in templates: doc+skills, doc-only, skills-only-no-doc (with warning), neither (with warning)
- [x] 9b.10 Write unit tests in `tests/gate-instructions-resolver.test.ts`: explicit doc wins over convention, convention fallback works, missing files produce warning, strict mode blocks on missing files, skills array preserved, empty skills handled
- [x] 9b.11 Write integration test in `tests/continuation-prompt-with-instructions.test.ts`: end-to-end prompt assembly for all four cases, verifying exact text matches spec

## 10. Session Idle Event Handler (Heart of the Loop)

- [x] 10.1 Implement `harness-loop-event-handler.ts` mirroring Ralph's `ralph-loop-event-handler.ts` structure
- [x] 10.2 Maintain in-process `inFlightSessions: Set<sessionId>` and `runtimeRetried: Map<sessionId, iteration>` (Ralph pattern)
- [x] 10.3 On `session.idle`: load state, abort if not active, abort if session_id mismatch (Ralph's matchesParentSession logic), abort if `latestUserMessageIsInProgress`, abort if `hasActiveBackgroundTasks`
- [x] 10.4 Apply settle window: `await sleep(IDLE_SETTLE_MS)` then re-load state and re-check guards
- [x] 10.5 Call completion detector; if any source fires, terminate loop and emit success toast
- [x] 10.6 Apply no-progress guard; if triggered, terminate with guard toast
- [x] 10.7 Check cache: if `checkpoints[currentGate].status==PASS && fresh`, skip runner and transition to next gate
- [x] 10.8 Run `phase_hooks.<gate>.before` if configured; on hook failure treat as FAIL
- [x] 10.9 Invoke runner; record output to state; apply rule-id history; check same-error guard
- [x] 10.10 Branch on status: PASS → transition (run `phase_hooks.after` async); FAIL → build continuation prompt → inject; WAITING → sleep + re-run same gate; BLOCKED → pause + ask user; SKIP → transition without injection; ERROR → terminate loop with error toast
- [x] 10.11 On `session.error` event: similar to Ralph's error retry path with retry budget; on `MessageAbortedError` honor the cancel signal
- [x] 10.12 On `session.deleted` event: clear state if it matches deleted session
- [x] 10.13 Apply hybrid fail policy: if `fail_policy=hybrid` and `gate_iteration >= auto_fix_attempts`, switch to ask-user mode for this gate
- [x] 10.14 Apply ultrawork verification: after PASS on a gate listed in `ultrawork_verify_gates`, set `verification_pending=true` and inject ultrawork verification prompt (do not transition until Oracle emits VERIFIED)
- [x] 10.15 Inject continuation prompt via `ctx.client.session.chat({sessionID, parts: [{type:"text", text: prompt}]})`
- [x] 10.16 Write integration test in `tests/event-handler.test.ts` that simulates a 5-iteration loop using a stub runner that returns FAIL, FAIL, PASS, PASS, PASS for sequential gates

## 10b. Async Gate Watcher Subagent

- [x] 10b.1 Implement `async-watcher-spawner.ts` exporting `spawnWatcher(ctx, gateName, config, state): Promise<string>` (returns task_id)
- [x] 10b.2 Build watcher prompt from template — embed runner path, gate name, feature_id, max_wait, poll_interval, constraints block per design D13
- [x] 10b.3 Invoke `task()` via OpenCode plugin API with `subagent_type=<async_subagent_type>`, `run_in_background=true`, `load_skills=[]`
- [x] 10b.4 Store returned task_id in `loop.watcher_task_id` via state controller
- [x] 10b.5 Implement `async-watcher-result-handler.ts` listening for subagent completion notifications and parsing the watcher's final output as RunnerOutput
- [x] 10b.6 Update `harness-loop-event-handler.ts` to detect `async: true` for current gate before invoking runner — short-circuit to watcher spawn if async, only invoke runner directly if synchronous
- [x] 10b.7 Implement outer grace timeout: per-watcher timer = `async_max_wait_seconds + 30s`; on expiry, call `background_cancel(task_id)` and synthesize FAIL RunnerOutput with `diagnostic_task_id`
- [x] 10b.8 Implement heartbeat scheduler: timer fires every `async_max_wait_seconds / 3`, checks `loop.watcher_task_id` still set, emits heartbeat toast (max 3 per gate)
- [x] 10b.9 Honor `async_heartbeats: false` config to suppress heartbeats entirely
- [x] 10b.10 Update `harness-off` command to call `background_cancel(loop.watcher_task_id)` before clearing state
- [x] 10b.11 Iteration accounting: spawnWatcher increments `gate_iteration` by exactly 1 (not per-poll), records same-error history once per watcher result
- [x] 10b.12 Write unit tests in `tests/async-watcher-spawner.test.ts`: prompt builds correctly, task_id stored, defaults applied
- [x] 10b.13 Write integration test in `tests/async-watcher-lifecycle.test.ts`: spawn → heartbeats fire → completion → state cleared; cover PASS, FAIL, watcher-timeout, outer-grace-timeout cases
- [x] 10b.14 Write integration test for `/harness-off` cancelling active watcher: verify `background_cancel` called, state cleaned

## 11. Slash Commands

- [x] 11.1 Implement `commands/harness-on.ts` exporting the command handler: parses CLI args (`--force`, `--max-iter=N`, `--skip-gate=X`, `--config=path`), loads config, calls `controller.startLoop()`, injects opening prompt
- [x] 11.2 Detect crash-recovery scenario (existing active state with different session_id); prompt user via question tool with resume/cancel-and-restart/abort options
- [x] 11.3 Implement `commands/harness-off.ts`: cancels active loop, waits up to 30s for in-flight runner subprocess, then SIGKILL
- [x] 11.4 Implement `commands/harness-on.md` and `commands/harness-off.md` slash-command markdown templates that invoke the TS handlers (these go in `.opencode/command/` not the plugin dir — separate registration)
- [x] 11.5 Write integration tests in `tests/commands.test.ts`: start fresh loop, double-start refusal, /harness-off cancels in-flight runner

## 12. Plugin Entry Point

- [x] 12.1 Implement `index.ts` exporting a plugin factory matching `@opencode-ai/plugin`'s expected shape
- [x] 12.2 Register `session.idle`, `session.error`, `session.deleted` event handlers
- [x] 12.3 Register `/harness-on` and `/harness-off` commands (via slash-command files outside plugin dir, but plugin exposes the handler functions)
- [x] 12.4 Implement no-op fast-path: if state file does not exist or `loop.active=false`, return from event handlers immediately after a single file existence check
- [x] 12.5 Log plugin load with version on first invocation per session

## 13. Patch nano-brain harness-check.sh for JSON Contract Compliance

- [x] 13.1 Audit current `scripts/harness-check.sh --json` output for contract compliance against the schema in harness-runner-contract spec
- [x] 13.2 Add/update JSON serialization to include: `gate`, `status`, `checks[]`, `next_gate`, `instructions_for_agent` (FAIL/BLOCKED only), `wait_seconds` (WAITING only), `rule_ids_violated`
- [x] 13.3 Map nano-brain phase names to gate names 1:1 (no renaming needed; phases ARE gates)
- [x] 13.4 Populate `rule_ids_violated` from existing rule references already in FAIL messages (R1, R7, R19, R20, R27, R28, R29, R31, R56, R89)
- [x] 13.5 Emit logs to stderr only; stdout must be JSON-only when `--json` flag passed
- [x] 13.6 Update exit codes to match contract: 0=PASS, 1=FAIL, 2=SKIP, 3=WAITING, 4=BLOCKED, 5=ERROR
- [x] 13.7 Add a `--validate` subcommand or special invocation that allows the plugin's pre-flight check to verify runner readiness
- [x] 13.8 Update `scripts/harness-check.sh` shell tests (or add new) verifying JSON output contract for each phase
- [x] 13.9 Add new gate `post-merge-npm-release` to `harness-check.sh` (or as separate sibling script `scripts/check-npm-release.sh` invoked by harness-check via dispatch) that:
  - Reads latest `gh release list --limit 1 --json tagName`
  - Queries `npm view @nano-step/nano-brain version`
  - Returns PASS if versions match
  - Returns WAITING (with `wait_seconds: 60`) if `gh run list --workflow=release.yml` shows `status=in_progress`
  - Returns FAIL otherwise (with `instructions_for_agent` explaining how to inspect `gh run view <id>` for failure)
- [x] 13.10 Add unit tests for the npm release runner mock scenarios: matching versions, mid-publish (WAITING), workflow failure (FAIL), no recent tag (skip)

## 14. Project Config + Instruction Docs for nano-brain

- [x] 14.1 Create `.opencode/harness.config.json` for nano-brain with: `runner_path: "./scripts/harness-check.sh"`, `gates: ["pre-work", "in-progress", "pre-merge", "post-merge", "post-merge-npm-release", "next-ready"]`, `rule_id_format: "R{id}"`, `fail_policy: "hybrid"` (note: `post-merge-npm-release` is the new async gate that waits for GitHub Actions `release.yml` → npm publish)
- [x] 14.2 Set `ultrawork_verify_gates: ["pre-merge"]` for nano-brain (high-stakes gate)
- [x] 14.3 Add `gate_instructions` mapping for nano-brain pointing to docs created in 14.5, including async config for `post-merge-npm-release`:
  ```jsonc
  "post-merge-npm-release": {
    "doc": "docs/harness/gates/post-merge-npm-release.md",
    "skills": ["dd-pup"],
    "async": true,
    "async_max_wait_seconds": 1800,
    "async_poll_interval_seconds": 60,
    "async_subagent_type": "quick"
  }
  ```
- [x] 14.4 Add inline comments in JSON5/JSONC-style or sibling README explaining each field
- [x] 14.5 Create `docs/harness/gates/` directory with one md file per gate:
  - `pre-work.md` — issue/branch/lane verification protocol (lift from current `harness-check.sh` pre-work section)
  - `in-progress.md` — story-completion validation, self-review, evidence requirements
  - `pre-merge.md` — full validation ladder, Oracle review, Gemini comment triage
  - `smoke-e2e.md` — extract the full bash recipe from `.opencode/skills/harness-check/SKILL.md` lines 117-152 (server build → port 3199 → curl → assertion)
  - `post-merge.md` — issue auto-close, branch deletion, b-main validation
  - `post-merge-npm-release.md` — async gate protocol: how to verify `gh run list --workflow=release.yml` succeeded, how to check `npm view @nano-step/nano-brain version` matches the new tag, timeout expectations (3-30 min)
  - `next-ready.md` — WIP checks, openspec archive, no stale PRs
- [x] 14.6 Write `docs/harness/gates/README.md` explaining what these files are and how the plugin references them
- [x] 14.7 Cross-link each gate doc back to the relevant section of `docs/HARNESS_GATES.md` so the canonical spec stays as the source of truth

## 15. Documentation

- [x] 15.1 Write `docs/HARNESS_RUNNER_CONTRACT.md` — full spec for any project that wants to write a compatible runner: JSON schema, exit codes, examples in bash/python/Go
- [x] 15.2 Update `docs/HARNESS.md` to reference `/harness-on` as the recommended flow, with manual `./scripts/harness-check.sh` invocation kept as ad-hoc fallback
- [x] 15.3 Update `.opencode/skills/harness-check/SKILL.md` to mention `/harness-on` exists and link to the plugin README
- [x] 15.4 Add a "How to adopt in your project" section to the plugin README with copy-paste steps and a minimal `harness.config.json` example
- [x] 15.5 Document the override mechanism: how to use `[HARNESS-OVERRIDE]: <reason>` token and the `harness.override.json` file
- [x] 15.6 Document `gate_instructions` config field with examples: how to point a gate at a doc, how to add skills, when to use convention fallback vs explicit path
- [x] 15.7 Write a "How to author a gate instruction doc" mini-guide in the plugin README — recommended structure (Hard Rules, Step-by-Step Procedure, Evidence Requirements, FAIL Conditions), example from nano-brain's `smoke-e2e.md` and capyhome's `e2e.md`
- [x] 15.8 Document strict vs flexible mode behavior and when to enable strict
- [x] 15.9 Document async gate semantics in plugin README: when to use `async: true`, how to size `async_max_wait_seconds`, choosing `async_subagent_type`, what to expect from heartbeat toasts
- [x] 15.10 Add a "Writing an async-aware runner" mini-guide: how the runner should return `WAITING` vs terminal status, example bash snippet for polling CI/npm/k8s

## 16. Capyhome Adoption Validation (Goal G5)

- [ ] 16.1 Copy `.opencode/plugin/harness-loop/` from nano-brain into capyhome's `.opencode/plugin/`
- [ ] 16.2 Write `.opencode/harness.config.json` for capyhome pointing at its `run-checkpoint.sh` with gates `["before-branch", "before-pr", "after-review-fix", "before-merge", "before-next"]` and `rule_id_format: "FP #{id}"`
- [ ] 16.3 Add `gate_instructions` mapping in capyhome's config pointing to existing capyhome docs:
  - `e2e` → capyhome's existing E2E protocol doc (likely `capy-home-web/test-case/e2e.md` or similar — locate during adoption)
  - `before-pr` → capyhome's PR-readiness protocol
  - `after-review-fix` → capyhome's review-fix protocol
- [ ] 16.4 Verify capyhome existing docs satisfy the "imperative protocol" expectation (hard rules, step-by-step, evidence requirements). If gaps, add to capyhome's docs (do NOT modify the plugin).
- [ ] 16.5 Patch capyhome's `run-checkpoint.sh` if needed to emit contract-compliant JSON (likely small additive change)
- [ ] 16.6 Run end-to-end test on capyhome: trigger `/harness-on` for a small real story, observe loop progresses through all 5 gates without manual intervention; specifically verify the agent reads `e2e.md` when fixing an E2E failure (vs. improvising)
- [ ] 16.7 Document any issues found in capyhome adoption back into nano-brain's plugin (fix bugs, clarify contract, update docs)
- [ ] 16.8 Validate async gate in capyhome (G5 async proof): identify capyhome's async-suitable gate (likely post-merge Vercel deploy or CI smoke test), wire it as `async: true` in capyhome's config, run `/harness-on` end-to-end and observe watcher spawn + heartbeats + completion across a real 5-10 min CI cycle

## 17. Dogfooding

- [ ] 17.1 Use `/harness-on` for the next 3 nano-brain features after this change merges
- [ ] 17.2 Capture pain points / unexpected behavior in `docs/HARNESS_BACKLOG.md` as new friction items
- [ ] 17.3 If 3 features complete cleanly, mark plugin as "ready for npm extraction" and open follow-up change

## 18. Validation Ladder

- [x] 18.1 Run `bun test` (or equivalent) on all plugin tests — all green
- [x] 18.2 Run `bun run typecheck` (or `tsc --noEmit`) on plugin — zero errors
- [x] 18.3 Verify plugin loads cleanly in a fresh OpenCode session (no console errors)
- [x] 18.4 Verify `/harness-on` and `/harness-off` appear in OpenCode's slash command palette
- [x] 18.5 Run `openspec validate add-harness-loop-plugin --strict --no-interactive` — passes

## 19. Archive

- [ ] 19.1 After merge + dogfood + capyhome validation, run `openspec archive "add-harness-loop-plugin"` to move into `openspec/archive/`
