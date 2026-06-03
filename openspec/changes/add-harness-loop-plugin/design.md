## Context

Two prior arts inform this design:

1. **`code-yeongyu/oh-my-opencode` Ralph/Ultrawork loop** (~1687 LOC TypeScript plugin in `src/hooks/ralph-loop/`) — proves the `session.idle` re-injection pattern works for autonomous loops. Key files studied: `ralph-loop-event-handler.ts`, `loop-state-controller.ts`, `continuation-prompt-builder.ts`, `pending-verification-handler.ts`, and the slash command templates in `src/features/builtin-commands/templates/ralph-loop.ts`.
2. **`capyhome` harness** (`.opencode/skills/harness-check/scripts/run-checkpoint.sh` 607 LOC bash + `harness-state.py` 168 LOC Python) — proves a project-specific runner + stack-agnostic state manager pattern works for real multi-repo workflows. Specifically, the `.sisyphus/harness-state.json` format with `{checkpoints: {<name>: {status, checked_at, story, issue_number, checks: {}}}}` is the de facto state schema we adopt.

Nano-brain currently has the script half of this stack (`scripts/harness-check.sh` 795 LOC bash, 6 phases) but is missing the loop engine — agents must remember to re-invoke after every failure. Comparison verdict (documented at conversation log): capyhome architecture wins 7-3 on flexibility, state, cache, anti-stuck guards, and subagent delegation patterns; nano-brain wins on rule traceability (R1, R7, …), spec rigor, and concrete smoke:e2e recipes. The design merges both.

**Constraints:**

- Plugin runs in OpenCode's Bun/Node runtime, **not** in the nano-brain Go binary. No `cmd/nano-brain/` or `internal/` changes.
- OpenCode plugin API surface limited to `@opencode-ai/plugin` (events: `session.idle`, `session.error`, `session.deleted`; injection via `ctx.client.session.chat`).
- Must work without nano-brain server running (some gates run before the binary builds).
- Must be reusable: capyhome must be able to adopt the same plugin by copying the directory and writing its own `harness.config.json`.

**Stakeholders:**

- Nano-brain maintainer (primary user — runs `/harness-on` to ship features)
- Capyhome maintainer (validates reusability — drops plugin in, points config at `run-checkpoint.sh`)
- Future projects (consume plugin as a template / npm package once stable)

## Goals / Non-Goals

**Goals:**

- **G1**: A single command `/harness-on` autonomously drives the harness from start of a feature to merge-ready, looping on FAILs until PASS or hard-stop conditions trigger.
- **G2**: Plugin is **stack-agnostic** — zero knowledge of Go, Python, JS, gh CLI, or specific gate names. All project-specific logic lives in the runner script.
- **G3**: State survives crashes — `/harness-off` then re-launching OpenCode then `/harness-on` resumes at the same gate with no progress lost.
- **G4**: Hard-stop conditions prevent runaway loops: max iterations per gate, max total iterations, no-progress detection, user cancellation, human override via `[HARNESS-OVERRIDE]: <reason>`.
- **G5**: Capyhome can adopt the plugin verbatim and its `run-checkpoint.sh` works as a runner with only a config-file write (proof of reusability before claiming the design is generic).
- **G6**: Continuation prompts cite rule IDs (R1, R7, R29, R31, R89, FP #37) so agent fixes are traceable to canonical rules.

**Non-Goals:**

- ❌ Re-implementing `harness-check.sh` logic in TypeScript. The plugin orchestrates; bash/python scripts execute.
- ❌ Multi-feature epic-level loops in v1. `/harness-on` handles **one feature/PR end-to-end**; epic transitions are explicit user actions.
- ❌ Watchdog/background mode (auto-fire gates on `git commit`). Tempting but invasive — deferred to v2 if v1 proves stable.
- ❌ Cross-project orchestration (running gates in multiple repos in one loop). Capyhome's multi-repo needs are handled inside its own runner script, not by the plugin.
- ❌ DSL for declaring gates in JSON/YAML. Project owns its runner; declarative configs reinvent Make/Taskfile.
- ❌ Replacing manual `./scripts/harness-check.sh` invocations. Manual single-gate runs remain valid for ad-hoc checks.
- ❌ Modifying the existing `harness-state.json` format that capyhome already uses. We adopt it as-is.

## Decisions

### D1 — Architecture: Plugin = loop engine + state machine; Runner = project-owned script

**Chosen:** Two-layer split. Plugin (TypeScript, in `.opencode/plugin/harness-loop/`) handles: slash commands, session.idle hook, loop iteration, completion detection, prompt injection, state persistence, anti-stuck guards. Runner (bash/python/anything-executable, in `scripts/` per project) handles: actual gate execution, gh CLI calls, build/test commands, project-specific knowledge.

**Alternatives considered:**

- **A) Everything in plugin (hardcoded gates).** Rejected — would require forking the plugin for each project. Fails G2 (stack-agnostic) and G5 (capyhome reuse).
- **B) Everything in runner (no plugin, just a smarter bash loop).** Rejected — bash can't hook into `session.idle` to detect when the agent is done iterating. Loses the Ralph re-injection trick which is the whole reason this works.
- **C) Declarative gate DSL in `harness.config.json` (every check as a JSON entry with cmd, on_fail rules).** Rejected — reinvents Make/Taskfile/just. Project owners already have bash skills; forcing them into a DSL is friction. Also, `harness-check.sh` and `run-checkpoint.sh` already exist as 1400+ LOC of working bash — no value in porting.

**Rationale:** The split mirrors Ralph (`ralph-loop-hook.ts` orchestrates, agent does the work). Same pattern, different "work" (gate checks instead of free-form coding). The runner contract is the only coupling point — clean separation.

### D2 — Runner contract: JSON stdout, exit codes, well-known status enum

**Chosen:** Runner is invoked as `<runner> <gate-name> [--feature=ID] [--force]` and MUST output a single JSON object to stdout, exit code matching the status:

```json
{
  "gate": "pre-work",
  "status": "PASS",
  "checks": [{"id": "1.1", "name": "Issue exists", "status": "PASS", "rule_id": "R89"}],
  "next_gate": "in-progress",
  "instructions_for_agent": "string (only when status=FAIL or BLOCKED)",
  "wait_seconds": 60,
  "rule_ids_violated": ["R29", "FP #37"]
}
```

Exit codes: `0=PASS`, `1=FAIL`, `2=SKIP`, `3=WAITING`, `4=BLOCKED`, `5=runner-error`. Status enum: `PASS | FAIL | SKIP | WAITING | BLOCKED | ERROR`.

**Alternatives considered:**

- **A) Plain text + parse pattern (`[PASS]`, `[FAIL]`).** Rejected — fragile, locale-dependent, no structure for rule IDs or instructions.
- **B) gRPC / HTTP between plugin and runner.** Rejected — over-engineered for a one-shot CLI invocation. Adds ports, lifecycle, error handling.
- **C) Bash sourcing the runner as a library.** Rejected — TypeScript plugin can't `source` bash, and forces same-language coupling.

**Rationale:** JSON-on-stdout + exit codes is the simplest cross-language contract. Both bash (`jq`) and Python emit it trivially. Plugin uses standard `child_process.spawn` to invoke. Status enum models all 5 real-world outcomes (capyhome's `BLOCKED`, nano-brain's `WAITING` for CI, etc.). `instructions_for_agent` is the magic field — when present, plugin injects it verbatim into the next continuation prompt.

### D3 — State file: capyhome format adopted unchanged, plus loop-meta extension

**Chosen:** State file at `.opencode/harness-loop.local.json` (gitignored). Schema:

```jsonc
{
  // capyhome compat: read directly by harness-state.py
  "feature_id": "feat-NNN-add-foo",
  "issue_number": 144,
  "story": "11.1",
  "updated_at": "2026-06-02T10:00:00Z",
  "checkpoints": { /* unchanged capyhome shape */ },

  // loop-meta extension (plugin-private)
  "loop": {
    "active": true,
    "current_gate": "in-progress",
    "gate_iteration": 3,
    "total_iteration": 7,
    "max_iterations_per_gate": 10,
    "max_total_iterations": 100,
    "started_at": "2026-06-02T09:00:00Z",
    "session_id": "ses_abc123",
    "config_snapshot": { /* cached harness.config.json */ },
    "last_runner_output": { /* last JSON from runner */ },
    "no_progress_count": 0,
    "override_active": false
  }
}
```

**Alternatives considered:**

- **A) Two separate files (capyhome state + plugin state).** Rejected — agent has to read both, race conditions on write, file proliferation.
- **B) Override capyhome format entirely (plugin-native schema).** Rejected — breaks capyhome's existing `harness-state.py`. Loses reusability.
- **C) Use SQLite for state.** Rejected — overkill, adds dep, harder for humans to inspect/edit.

**Rationale:** Single JSON file, humans can `cat` it, capyhome's existing tools work unchanged on the `checkpoints` block. Plugin owns the `loop` sub-object and ignores everything else. State file location is `.opencode/` not `.sisyphus/` so it co-locates with the plugin (capyhome can override path via `state_file_path` config option).

### D4 — Fail-handling policy: Hybrid (auto-fix N times, then ask)

**Chosen:** Three-policy config option in `harness.config.json`:

- `"auto"` — Inject `instructions_for_agent`, loop forever (up to `max_iterations_per_gate`). Ralph-style. Default for `infrastructure` / `refactor` change types.
- `"hybrid"` — Auto-fix up to `auto_fix_attempts` (default 3), then escalate via `chrome-devtools_port_19263_*` toast / question tool asking user "retry | abort | skip | override". **Default for `user-feature` / `bug-fix`.**
- `"ask"` — Ask user on first FAIL. Conservative. Default for `high-risk` lane.

**Alternatives considered:**

- **A) Always auto-loop.** Rejected — agent can burn tokens on unfixable structural issues (e.g., missing CI secret).
- **B) Always ask user.** Rejected — defeats the whole point of autonomy.
- **C) ML-based "should I keep trying" heuristic.** Rejected — premature, no training data, agent's own no-progress detector is enough signal.

**Rationale:** Hybrid mirrors Ralph's `latestAssistantTurnMadeNoProgress` (auto-stop) + adds explicit user escalation. Defaults wired to change type so user doesn't have to configure for common cases. `auto` mode is opt-in for users who want fully-hands-off mode (and accept the token risk).

### D5 — Anti-stuck guards: Six guards inherited from Ralph + one from capyhome FP #37

**Chosen:** Plugin enforces these hard stops:

1. `max_total_iterations` (default 100, hard cap) — global ceiling, never exceed.
2. `max_iterations_per_gate` (default 10) — per-gate ceiling, prevents one stuck gate from consuming entire budget.
3. **No-progress detection** — if `latestAssistantTurnMadeNoProgress` (zero tokens emitted between two idle events), stop and report. Borrowed from Ralph.
4. **Same-gate-same-error counter** — if same `rule_ids_violated[]` returned 3 times in a row from same gate, BLOCKED and ask user. Borrowed from capyhome FP #37 (`e2e-round --action check`).
5. **In-flight session lock** — `inFlightSessions: Set<sessionID>` prevents re-entry races. Direct steal from Ralph `ralph-loop-event-handler.ts`.
6. **User-in-progress window** — if user typed in last 2s, defer injection. Borrowed from Ralph `USER_MESSAGE_IN_PROGRESS_WINDOW_MS`.
7. **Background-task wait** — if explore/oracle/etc subagent active, wait for it before injecting. Borrowed from Ralph `hasActiveBackgroundTasks`.

**Alternatives considered:** Only Ralph's set (1-3, 5-7). Rejected — capyhome's FP #37 same-error-repeats-N-times is a different signal than no-progress (agent might be making changes but they don't help). Both signals add value.

### D6 — Completion signal: Single `<promise>HARNESS-COMPLETE</promise>` tag, plus structural fallback

**Chosen:** Loop ends when one of three things happens:

1. Agent emits `<promise>HARNESS-COMPLETE</promise>` literal (Ralph-style explicit signal). Configurable via `completion_promise` in config (default `HARNESS-COMPLETE`).
2. Runner returns `status: "PASS"` AND `next_gate: null` AND current gate is the final gate in `gates[]`. Structural fallback — agent doesn't need to remember the tag.
3. User runs `/harness-off`.

**Alternatives considered:**

- **A) Only the promise tag.** Rejected — agent forgets, especially after long multi-gate flows. Capyhome's `before-next` checkpoint naturally signals "done" without an explicit tag.
- **B) Only structural detection.** Rejected — loses explicit agent intent. If agent wants to bail early ("I think we're done, let user verify"), it has no way to signal.

**Rationale:** Two signals, OR'd together, max compatibility. Plugin honors whichever fires first.

### D7 — Ultrawork-style Oracle verification: Optional per-gate flag

**Chosen:** Config option `ultrawork_verify_gates: ["pre-merge"]` — list of gate names that require Oracle verification after the runner returns PASS. Implementation borrowed verbatim from Ralph's `pending-verification-handler.ts` + `ULTRAWORK_VERIFICATION_PROMPT` template. Defaults to empty list (opt-in).

**Rationale:** Some gates (especially `pre-merge`) benefit from a second-eye check beyond automated CI. Oracle verification adds cost (one Opus call per verified gate) so default off, but easy to enable for high-risk lanes.

### D8 — Config layering: Defaults → project config → CLI args, with override file for one-shot exceptions

**Chosen:** Four layers, last-write-wins:

1. **Plugin defaults** (hardcoded constants in TypeScript)
2. **Project config** `.opencode/harness.config.json`
3. **Per-run override file** `.opencode/harness.override.json` (gitignored, optional, auto-deleted on loop end)
4. **CLI args** to `/harness-on` (e.g., `/harness-on --skip-gate=in-progress --max-iter=20`)

**Rationale:** Standard layering. The override file is the escape hatch for human exceptions ("just this once skip the e2e gate") without modifying the committed config. Layer 4 wins over 3 wins over 2 wins over 1.

### D9 — Distribution: Inline in nano-brain repo first, extract to npm package only after capyhome adoption proves design

**Chosen:** Plugin lives at `.opencode/plugin/harness-loop/` inside nano-brain repo for v1. Documented as "copy this directory to adopt in your project." After capyhome successfully adopts the plugin (G5), extract to `@nano-step/harness-loop` npm package as a follow-up change.

**Rationale:** Premature packaging adds publish/version overhead before the design is battle-tested. Inline-first lets us iterate on contract + state schema without breaking external consumers. Two-project validation (nano-brain + capyhome) is the bar for "ready to extract."

### D10 — Continuation prompt format: Reuse Ralph's `SYSTEM_DIRECTIVE_PREFIX`, embed rule IDs and runner instructions

**Chosen:** Continuation prompt template:

```
[SYSTEM_DIRECTIVE — HARNESS LOOP gate={{GATE}} iter={{ITER}}/{{MAX_PER_GATE}} total={{TOTAL}}/{{MAX_TOTAL}}]

Gate "{{GATE}}" failed. Rules violated: {{RULE_IDS}}.

Runner instructions:
{{INSTRUCTIONS_FOR_AGENT}}

Fix the listed failures, then continue. The harness will re-check {{GATE}} automatically on your next idle.

If you cannot fix and need human input, add comment "[HARNESS-OVERRIDE]: <reason>" to your reply and the loop will pause for user approval.

Original feature: {{FEATURE_ID}}
```

**Rationale:** `SYSTEM_DIRECTIVE_PREFIX` (proven in Ralph) tells the agent this is system-injected, not user-typed. Embedding rule IDs satisfies G6 (traceability). The override hint satisfies the R7 escape hatch from nano-brain.

### D11 — Plugin file layout: Mirror Ralph's directory structure

**Chosen:**

```
.opencode/plugin/harness-loop/
  index.ts                          # plugin entry, registers hook + commands
  types.ts                          # HarnessLoopState, RunnerOutput, ConfigSchema
  constants.ts                      # DEFAULT_MAX_ITERATIONS, completion promise, etc.
  loop-state-controller.ts          # startLoop/cancelLoop/getState/incrementIteration
  storage.ts                        # read/write .opencode/harness-loop.local.json
  harness-loop-event-handler.ts     # session.idle handler — heart of the loop
  runner-invoker.ts                 # spawn runner, parse JSON, validate contract
  continuation-prompt-builder.ts    # build prompt from state + runner output
  completion-detector.ts            # scan transcript for promise tag, check structural completion
  no-progress-detector.ts           # borrowed from Ralph
  same-error-detector.ts            # FP #37 equivalent — same rule_ids N times
  config-loader.ts                  # layer defaults → project → override → CLI
  commands/
    harness-on.ts                   # /harness-on entry
    harness-off.ts                  # /harness-off entry
  templates/
    continuation-prompt.ts          # template strings
    opening-prompt.ts               # initial /harness-on injection
  README.md                         # how to adopt in other projects
  tests/
    runner-invoker.test.ts
    completion-detector.test.ts
    config-loader.test.ts
    loop-state-controller.test.ts
    ...
```

**Rationale:** One file per concept, easy to test in isolation. Layout matches Ralph's `src/hooks/ralph-loop/` so contributors familiar with Ralph can navigate immediately.

### D12 — Per-gate instruction docs + skill mapping (project supplies domain knowledge)

**Problem:** The runner contract returns `instructions_for_agent: "TC-03 failed: login flow broken"`. That tells the agent *what* failed but not *how* the project verifies that gate. Same gate name "e2e" means:
- Capyhome: launch Chrome via Playwright, click real buttons, follow `e2e.md` hard rules (1 OTP per attempt, screenshot evidence required, no session reuse)
- Nano-brain: build binary, start server on port 3199, curl endpoints, validate JSON shape against MCP contract
- A future project: k6 load test, Postman collection, CLI integration test

Plugin cannot know which protocol applies. Runner can't sensibly embed a 500-line procedure inside every JSON output (bloats prompts × 10 iterations).

**Chosen:** Config declares per-gate instruction mapping. Plugin embeds **references** (doc path + skill names) in every continuation prompt; the agent uses standard `read` + `skill` tools to fetch full content on demand.

```jsonc
// .opencode/harness.config.json
{
  "gate_instructions": {
    "e2e": {
      "doc": "docs/harness/gates/e2e.md",
      "skills": ["e2e-test-generator", "playwright"]
    },
    "pre-merge": {
      "doc": "docs/harness/gates/pre-merge.md",
      "skills": ["review-work"]
    },
    "smoke-e2e": {
      "doc": "docs/harness/gates/smoke-e2e.md",
      "skills": []
    }
  }
}
```

**Continuation prompt template includes a clearly-marked instructions block** (extending the D10 template):

```
[SYSTEM_DIRECTIVE — HARNESS LOOP gate=e2e iter=3/10 total=7/100]

Gate "e2e" failed. Rules violated: FP #37, TC-03.

📖 Read project's gate protocol FIRST (mandatory):
   docs/harness/gates/e2e.md

🔧 Load skills before attempting fix:
   - e2e-test-generator
   - playwright

Runner instructions:
TC-03 failed: login flow broken at /home

Fix per docs/harness/gates/e2e.md. The harness will re-check e2e on your next idle.
...
```

**Conventions:**

- **Convention-based fallback path**: If `gate_instructions.<gate>.doc` is omitted, plugin auto-tries `docs/harness/gates/<gate>.md`. If that file also doesn't exist, plugin embeds a warning in the prompt instead of failing the loop.
- **Skills are optional**: If `skills` array is empty or omitted, the prompt simply omits the "Load skills" section.
- **Flexible mode (default)**: Missing doc files → warning toast + prompt warns agent "no protocol doc found for this gate, use general best practices". Loop continues. Strict mode is opt-in via config.
- **Single doc per gate (v1)**: `doc` is a single string, not an array. Multi-doc is a deliberate non-goal for v1; if needed, project can have one master doc that references others.
- **Doc is project-owned**: Plugin reads no contents from the doc. It just embeds the path. Agent reads it with the standard `read` tool. This keeps the plugin language-agnostic and the docs version-controlled with the project.

**Alternatives considered:**

- **A) Inline full instructions in runner output JSON.** Rejected — bloats prompt (500-line markdown × 10 iter = thousands of wasted tokens), couples instruction maintenance to runner script changes, no benefit over a separate doc file.
- **B) Skill-only (no doc files).** Rejected — forces every project to scaffold a full SKILL.md + skill description + skill-creator workflow just to give the agent gate-fix knowledge. Heavy setup. Doc files are lighter. We support both via `doc + skills` hybrid instead.
- **C) Plugin reads doc and inlines content into prompt.** Rejected — plugin would need to track doc updates, handle markdown rendering, manage prompt budget. Standard `read` tool already does this on demand from the agent side. Plugin stays simple.

**Why doc + skills together:**

- **Doc**: project-specific protocol (capyhome's `e2e.md` with 14 hard rules, nano-brain's smoke:e2e bash recipe). Markdown is the project's source of truth for "how do we do this here".
- **Skills**: reusable cross-project knowledge (`e2e-test-generator`, `review-work`, `playwright`). When a doc says "use Playwright to click the login button", the skill knows the Playwright API surface. Doc supplies project-specific *what*; skill supplies generic *how*.

**Capability assignment:** This becomes its own capability `harness-gate-instructions` (see spec). It is orthogonal to the runner contract (runner doesn't care about docs) and to the state machine (state doesn't store doc content). It only touches: config schema (new field), continuation prompt builder (embed references), and project file layout (where docs live).

### D13 — Async gates: config-driven background watcher subagent

**Problem:** Some gates depend on external systems that take minutes to settle:

- Nano-brain `post-merge-npm-release`: after merge to master, `auto-tag.yml` → `release.yml` → `npm publish` runs in GitHub Actions. Typical 3-5 min, worst-case 30 min (runner queue).
- Capyhome equivalent: `gh pr merge --squash` → CI deploy → Vercel cold start → smoke test endpoint.
- Future projects: k8s rollout, DNS propagation, CDN cache invalidation.

The existing `status: "WAITING"` mechanism (D2) handles this in theory: runner returns WAITING + `wait_seconds`, plugin sleeps and re-polls. But three real problems emerge for **long** waits:

1. **Iteration accounting wrong.** Every poll counts against `max_iterations_per_gate` (default 10). A 30-min wait with 60s poll interval = 30 polls > 10 → loop falsely BLOCKED.
2. **Context bloat in main session.** Each `session.idle` cycle adds toast notifications, status messages, runner output snapshots. 30 polls = noticeable context spend.
3. **No clean abort.** User pressing `/harness-off` mid-30-min-wait should kill the watcher cleanly, but synchronous polling in the main session is fragile to cancel.

**Chosen:** Per-gate `async: true` flag in `gate_instructions`. When set, the plugin spawns a `quick` background subagent that owns the polling loop entirely, leaving the main session idle until the watcher reports terminal status.

```jsonc
// .opencode/harness.config.json
{
  "gate_instructions": {
    "post-merge-npm-release": {
      "doc": "docs/harness/gates/post-merge-npm-release.md",
      "skills": ["dd-pup"],
      "async": true,
      "async_max_wait_seconds": 1800,
      "async_poll_interval_seconds": 60,
      "async_subagent_type": "quick"
    }
  }
}
```

**Flow:**

```
Main session: harness loop reaches gate "post-merge-npm-release"
  ↓
Plugin detects async=true → does NOT invoke runner from main session
  ↓
Plugin spawns subagent via task(subagent_type="quick", run_in_background=true, ...)
  ↓ subagent runs in background:
  │   while elapsed < max_wait:
  │     invoke runner; parse JSON
  │     if status in [PASS, FAIL, BLOCKED] → return that status
  │     if status == WAITING → sleep poll_interval, repeat
  │   on timeout → return synthesized FAIL with "watcher timed out after Ns"
  ↓
Main session: plugin waits for subagent completion notification
  ↓
On notification: plugin reads subagent result → treats it as if runner returned that status directly → normal loop transitions
```

**Toast schedule (live status, minimal):**

- **Start**: "🕐 Watching gate <name> via background subagent (max <N>min)" — variant=info
- **Heartbeat**: every `async_max_wait_seconds / 3` (e.g., 10min for a 30min cap), emit "⏳ Still watching <name>... (<elapsed>/<max>min)" — variant=info, only if still WAITING
- **End-success**: "✅ Gate <name> passed (took <elapsed>s)" — variant=info
- **End-fail**: "❌ Gate <name> failed: <short reason>" — variant=warning
- **End-timeout**: "⏰ Gate <name> timed out after <max>s" — variant=warning, treated as FAIL per the design decision

Total: 3-4 toasts across a 30-min wait. Not silent, not spammy.

**Iteration accounting (fixes Problem 1):**

The async wait counts as **exactly one** iteration of the parent gate, regardless of how many internal polls the watcher does. `gate_iteration` increments by 1 when the subagent is spawned, not per-poll. This means `max_iterations_per_gate` correctly limits "how many times agent attempted this gate," not "how long we waited."

**Context isolation (fixes Problem 2):**

The watcher subagent runs in its own session with its own context budget. Main session receives only:
- 1 toast on spawn
- 0-2 heartbeat toasts during wait
- 1 final toast + 1 internal state update with the final RunnerOutput

Main session context grows by ~50-100 tokens total per async gate, regardless of wait duration.

**Cancellation semantics (fixes Problem 3):**

`/harness-off` clears `loop.active=false` and additionally calls `background_cancel(taskId=watcher_task_id)`. The watcher subagent terminates within seconds (its next sleep wake-up checks an abort signal via plugin shared state). State is cleaned up uniformly.

**Watcher subagent prompt template:**

```
You are a harness loop background watcher for gate "{{GATE}}".

TASK: Poll the runner until it returns a terminal status (PASS, FAIL, BLOCKED) or timeout.

RUNNER COMMAND:
{{RUNNER_PATH}} {{GATE}} --json --feature={{FEATURE_ID}}

POLL LOOP:
1. Run the runner command, capture stdout.
2. Parse JSON. Extract `status` field.
3. If status in [PASS, FAIL, BLOCKED] → output the JSON verbatim as your final response and stop.
4. If status == WAITING → sleep {{POLL_INTERVAL}} seconds.
5. If total elapsed > {{MAX_WAIT}} seconds → output a synthesized JSON:
   {"gate": "{{GATE}}", "status": "FAIL", "instructions_for_agent": "Watcher timed out after {{MAX_WAIT}}s; gate did not reach terminal status", "rule_ids_violated": ["watcher-timeout"]}
   and stop.
6. Otherwise repeat from step 1.

OUTPUT FORMAT: Exactly one JSON object matching the RunnerOutput contract. Nothing else. No prose, no explanations.

CONSTRAINTS:
- Do NOT modify any files.
- Do NOT spawn other subagents.
- Do NOT use any tool other than bash.
- Total wall-clock time MUST be bounded by {{MAX_WAIT}} seconds.
```

The subagent has a tightly scoped prompt and is constrained to one tool (`bash`) to keep behavior predictable and cheap.

**Alternatives considered:**

- **A) New runner status `ASYNC`.** Rejected — forces agent to learn a 7th status, runner needs to know which gates are async (vs config knowing). Config-driven is simpler and matches the existing `gate_instructions` pattern.
- **B) Plugin runs the polling loop itself in main session.** Rejected — that's the existing `WAITING` mechanism. Doesn't solve the 3 problems for long waits.
- **C) Use a separate "watcher" plugin or external tool (cron, GitHub webhook).** Rejected — adds operational complexity, external dependencies, requires user setup outside OpenCode.
- **D) Run watcher in a detached shell process via `nohup`.** Rejected — opaque to OpenCode, hard to cancel, no toast integration.

**Why `quick` as default subagent_type:**

The watcher does almost no reasoning — just runs bash, parses JSON, sleeps, repeats. Haiku (which `quick` maps to) is sufficient and cheap. Override via `async_subagent_type` is available for gates needing more intelligence (e.g., diagnostic interpretation on FAIL).

## Risks / Trade-offs

- **[R1] Runner contract drift.** If projects implement the JSON contract slightly differently (missing fields, wrong status names), plugin can hang or crash → **Mitigation**: Plugin validates with a strict Zod/Valibot schema on every runner output. On schema violation, status auto-set to `ERROR` and loop stops with clear error message naming the missing/invalid field.

- **[R2] Token explosion in `auto` mode.** Agent loops 100 times on an unfixable issue, burns thousands of tokens → **Mitigation**: `max_total_iterations` hard cap (default 100); `auto` mode is opt-in not default; defaults wire to `hybrid` (escalate to user after 3 attempts).

- **[R3] State file corruption mid-write.** Plugin crash during JSON write leaves invalid file → **Mitigation**: Atomic write (write to `.tmp` then `rename`), as already implemented in Ralph's `storage.ts`. Lift the same pattern.

- **[R4] Plugin loads on every OpenCode session, slowing startup even when no loop active.** → **Mitigation**: Hook is no-op when state file absent (`active != true`). Cost is one file existence check per `session.idle` event — negligible.

- **[R5] Capyhome reuse fails because its runner uses Python while plugin expects bash.** → **Mitigation**: Runner contract is language-agnostic — plugin invokes via `child_process.spawn` with `runner_path` from config. Any executable that emits valid JSON to stdout works.

- **[R6] Rule ID format collisions between projects (nano-brain uses `R89`, capyhome uses `FP #37`).** → **Mitigation**: Plugin treats `rule_ids_violated` as opaque strings, passes through unchanged. No parsing. Projects pick their own format.

- **[R7] User cancels via `/harness-off` mid-runner-invocation.** Race: runner already spawned, plugin already wrote `loop.active=false` → **Mitigation**: Plugin waits for in-flight runner subprocess to finish (with 30s timeout, then SIGKILL) before clearing state. Same as Ralph's `inFlightSessions` lock.

- **[R8] OpenCode plugin API changes (we use undocumented internals).** Ralph uses `ctx.client.session.chat`, `tui.showToast`, etc. — if OpenCode 1.16+ breaks these → **Mitigation**: Pin OpenCode version in `.opencode/version-pin` or similar; document tested OpenCode version in plugin README; add regression tests against current Bun-bundled plugin SDK types.

- **[R9] Continuation prompt injection during user typing creates jarring UX.** → **Mitigation**: `USER_MESSAGE_IN_PROGRESS_WINDOW_MS=2000` defer guard (D5 #6). Plus a visible toast notification per iteration so user knows the loop is active.

- **[R10] Trade-off: Stack-agnostic plugin means nano-brain-specific optimizations are impossible.** E.g., we can't have the plugin directly call `go build` with parallelism flags → **Acceptable**: Project-specific perf belongs in the runner, not the plugin. This is the explicit price of reusability.

- **[R11] Agent ignores instruction doc reference and improvises.** Plugin embeds `docs/harness/gates/e2e.md` path in the prompt, but the agent might skip reading it and try generic fixes → **Mitigation**: Continuation prompt uses imperative phrasing ("Read project's gate protocol FIRST (mandatory)") and includes the doc path twice (in header and in instructions). Skill `harness-check` documentation explicitly trains the agent to read the doc before fixing. Same-error guard (D5 #4) will catch repeated failures within 3 iterations and surface to the user — naturally penalizes ignoring the doc.

- **[R12] Instruction doc gets stale relative to runner script.** Project updates `run-checkpoint.sh` adding a new check but forgets to update `gates/e2e.md` → **Mitigation**: Doc is referenced not embedded, so the agent gets the runner's `instructions_for_agent` field (live data) + the doc's protocol (background context). Runner output is authoritative for what failed; doc is authoritative for how to verify. Both can drift independently without breaking the loop. Recommend a `docs/harness/gates/README.md` or similar pointer to remind maintainers.

- **[R14] Watcher subagent crashes mid-poll.** Subagent process dies, no result returned to main session, loop hangs → **Mitigation**: Plugin sets an outer timeout = `async_max_wait_seconds + 30s` grace window. If no completion notification within outer timeout, plugin assumes watcher crash, marks gate FAIL with reason "watcher subagent did not return result within outer deadline," and continues per fail policy. Plugin records the watcher's task_id in state so a debug command can fetch its partial logs later.

- **[R15] Watcher subagent ignores constraints (modifies files, spawns more subagents, exceeds wall-clock).** Despite the constrained prompt, the watcher might violate the contract → **Mitigation**: Watcher prompt explicitly lists CONSTRAINTS. Subagent type `quick` (haiku) has minimal capability to disobey. If watcher returns non-conformant JSON, plugin treats as ERROR status (existing handling). Wall-clock bound enforced by outer timeout (R14).

- **[R16] Async gate placed early in gate sequence (anti-pattern).** User declares `gates: ["post-merge-npm-release", "pre-work"]` reversing logical order; async gate at index 0 blocks loop start → **Mitigation**: Document recommended pattern (async gates last). No hard validation — user owns gate ordering. Async gates running before sync gates is technically valid, just unusual.

- **[R13] Skills referenced in `gate_instructions.<gate>.skills` don't exist in the OpenCode skill registry.** Project misspells skill name or skill was uninstalled → **Mitigation**: Plugin validates skill names on `/harness-on` start against the registry (best-effort — registry API may not be available in all OpenCode versions). On unknown skill, emit warning but proceed. Prompt embeds skill names as plain references; agent's own `skill` tool will return "not found" gracefully if the skill is missing, and the agent can proceed with doc-only guidance.

## Migration Plan

This is a purely additive feature — no migration needed for existing nano-brain usage. The plugin coexists with manual `./scripts/harness-check.sh` invocations indefinitely.

**Rollout steps:**

1. Land plugin + config + JSON patch to `harness-check.sh` behind nano-brain's normal PR flow.
2. Dogfood: maintainer uses `/harness-on` for next 3 features. If issues, file follow-ups against this change before extracting to npm package.
3. Adopt in capyhome: copy `.opencode/plugin/harness-loop/`, write `harness.config.json` pointing at `run-checkpoint.sh`, validate G5 (proves reusability).
4. If capyhome adoption is clean, open a follow-up change `extract-harness-loop-to-npm-package` to publish as `@nano-step/harness-loop` and document install via `npx`.

**Rollback:** Delete `.opencode/plugin/harness-loop/`, `.opencode/command/harness-on.md`, `.opencode/command/harness-off.md`, `.opencode/harness.config.json`. Revert the JSON patch to `scripts/harness-check.sh`. Manual `./scripts/harness-check.sh` invocations continue to work — they were never modified destructively.

## Open Questions

- **Q1** — Should `/harness-on` accept a `--start-from-gate=<name>` argument to resume at a specific gate, or always start from the first gate and let the state file's cache TTL skip already-passing gates? Leaning toward the latter (simpler, exploits existing cache).
- **Q2** — Where exactly should `.opencode/harness-loop.local.json` live? `.opencode/` (co-located with plugin) or `.sisyphus/` (capyhome convention)? Vote: `.opencode/` as default with config-overridable `state_file_path`.
- **Q3** — Do we need `/harness-status` command to inspect current loop state without cancelling? Probably yes, but defer to a follow-up change if not strictly needed for v1.
- **Q4** — Should the override file (`.opencode/harness.override.json`) auto-delete on loop completion, or persist for the next run? Vote: auto-delete (matches `[HARNESS-OVERRIDE]: <reason>` one-shot semantics).
- **Q5** — Multi-doc per gate (e.g., `doc: ["hard-rules.md", "tc-template.md"]`)? Decided NO for v1 (single string only). If needed in v2, can switch to `string | string[]` with backward compat.
- **Q6** — Should missing instruction docs be strict (block loop) or flexible (warn + continue)? Decided **flexible** for v1. Strict mode opt-in via `config.strict_instructions: true` if added later.
- **Q7** — Async gates: should watcher heartbeat toasts be opt-in vs default? Decided **default ON with sensible cadence** (every `async_max_wait_seconds / 3`, max 3 heartbeats per gate). Users with strong silent preference can set `async_heartbeats: false` at config level to disable.
- **Q8** — Should multiple async gates be supported in parallel (e.g., wait for CI release AND wait for docs deploy simultaneously)? Decided **NO for v1** — async gates run sequentially in `gates[]` order. Parallel async is a meaningful v2 extension if proven needed.
