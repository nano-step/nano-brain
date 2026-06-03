## Why

Today, running the nano-brain harness (`./scripts/harness-check.sh`) requires the agent to manually invoke the script after every transition, interpret PASS/FAIL output, fix failures, and remember to re-run the same phase. The agent frequently forgets gates, skips checkpoints under context pressure, or stops mid-phase when a check fails — leaving features partially verified.

A working reference exists: `code-yeongyu/oh-my-opencode` ships built-in Ralph/Ultrawork loop commands (`/ralph-loop`, `/ulw-loop`) that hijack `session.idle` events to auto-inject continuation prompts until the agent emits a completion promise. A parallel project (`capyhome`) has independently evolved a more sophisticated harness with persistent state, TTL cache, E2E round counters, and concrete subagent delegation patterns — proving the harness pattern is reusable across stacks but the runtime engine is missing.

We need a **generic loop engine** (`/harness-on`) that drives any project's harness script to completion autonomously, with state persistence, anti-stuck guards, and per-project overrides — without hardcoding nano-brain's specific gates into the plugin.

## What Changes

- Publish a new standalone npm package `@nano-step/harness-loop` — an OpenCode plugin that can be consumed by any project via a single line in `opencode.json`: `"plugin": ["@nano-step/harness-loop"]`. OpenCode installs it automatically via Bun at startup, no manual copy required.
- Expose a new slash command `/harness-on` that starts an autonomous harness execution loop for the current feature
- Expose `/harness-off` to cancel an active loop (mirrors `/cancel-ralph`)
- Define a **runner contract**: plugin invokes `<runner> <gate> --json` and parses stdout for `{status, checks, next_gate, instructions_for_agent, wait_seconds}` to decide next action
- Introduce per-project config at `.opencode/harness.config.json` declaring: runner path, gate sequence, max iterations, fail-handling policy, cache TTL, override list
- Persist live loop state to `.opencode/harness-loop.local.json` (gitignored) so the loop survives crashes and `/harness-off → /harness-on` resumes
- Patch `scripts/harness-check.sh` to add proper `--json` output matching the contract (currently the flag is declared but output is not contract-conformant)
- Inject continuation prompts that reference nano-brain's rule IDs (R1, R7, R29, R31, R89) so agent fixes trace back to the canonical rule
- Honor `[HARNESS-OVERRIDE]: <reason>` escape hatch (R7) so humans can force-advance past a blocked gate
- Add **per-gate instruction docs + skills mapping** so agents fixing failures know HOW the project verifies that specific gate. Without this, the runner says "TC-03 failed" but the agent doesn't know whether to launch a browser (capyhome) or curl an HTTP endpoint (nano-brain). Config declares `gate_instructions: {<gate>: {doc, skills}}` and the plugin embeds doc references + skill-load hints in every continuation prompt.
- Add **async gate support** for long-running external waits (CI release pipeline, npm publish propagation, deployment health checks) via config flag `async: true` per gate. Plugin spawns a `quick` background subagent that polls the runner without bloating the main session's context, then resumes the main loop when the watcher reports terminal status (PASS / FAIL / TIMEOUT). Solves the case where nano-brain merges to master, GitHub Actions takes 3-30 minutes to publish to npm, and the main agent should not be polling synchronously the whole time.
- **Plugin is stack-agnostic.** Nano-brain ships the first runner and is listed first in `opencode.json`; capyhome can adopt by adding `"@nano-step/harness-loop"` to its own `opencode.json` and pointing `harness.config.json` at `run-checkpoint.sh` — no file copying required.

## Capabilities

### New Capabilities

- `harness-loop-plugin`: The OpenCode plugin itself — slash commands, hook registration, lifecycle. Covers `/harness-on`, `/harness-off`, `session.idle` event handling, completion promise detection.
- `harness-loop-state`: Persistent state machine for the loop — schema, transitions, crash recovery, TTL cache for gate results.
- `harness-runner-contract`: The contract between plugin and project-specific gate runner script — JSON output shape, status codes, exit semantics, `instructions_for_agent` payload.
- `harness-loop-config`: Per-project config schema (`harness.config.json`) — runner path, gate sequence, fail policy, override mechanism, rule-id formatting.
- `harness-gate-instructions`: Per-gate instruction document and skill mapping — declares HOW each project verifies each gate (e2e via browser? via curl? via k6?). Plugin embeds doc-path references and skill-load hints in continuation prompts; agent reads docs and loads skills before attempting fixes. Solves the "same gate name, different protocol per project" problem.

### Modified Capabilities

None. This is purely additive — no existing nano-brain capability changes its spec. The `harness-smoke-ui-gate` capability remains a leaf check that the new loop may invoke but does not redefine.

## Impact

- **New files:**
  - `.opencode/plugin/harness-loop/` (TypeScript plugin, ~6 files modeled on Ralph)
  - `.opencode/command/harness-on.md`, `harness-off.md` (slash command templates)
  - `.opencode/harness.config.json` (project config — includes `gate_instructions` mapping)
  - `.sisyphus/harness-loop.local.json` (live state, gitignored)
  - `docs/HARNESS_RUNNER_CONTRACT.md` (contract spec for projects)
  - `docs/harness/gates/<gate>.md` (one instruction doc per gate — nano-brain ships its own; capyhome ships its own; convention-based default path)
- **Modified files:**
  - `scripts/harness-check.sh` — add JSON output mode conforming to runner contract
  - `.gitignore` — add `.sisyphus/harness-loop.local.json`, `.opencode/harness-loop.local.*`
  - `docs/HARNESS.md` — reference the new `/harness-on` flow as the recommended invocation
  - `.opencode/skills/harness-check/SKILL.md` — note that `/harness-on` is preferred for end-to-end runs; keep manual `./scripts/harness-check.sh` for ad-hoc single-gate checks
- **No code changes** to existing `cmd/nano-brain/` or `internal/` packages — the plugin runs entirely in OpenCode's runtime, not in the nano-brain binary
- **No new runtime dependencies** in nano-brain binary (Go side untouched). Plugin uses only `@opencode-ai/plugin` SDK (already provided by OpenCode runtime)
- **No DB schema changes**
- **No public API contract changes**
- **Reusability:** capyhome can adopt the same plugin verbatim by copying `.opencode/plugin/harness-loop/` and writing its own `harness.config.json` pointing at `run-checkpoint.sh` — proves the design
