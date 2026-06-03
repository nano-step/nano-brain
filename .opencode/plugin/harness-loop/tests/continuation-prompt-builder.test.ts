import { describe, it, expect } from "vitest";
import { tmpdir } from "node:os";
import { randomUUID } from "node:crypto";
import { join } from "node:path";
import { buildContinuationPrompt } from "../continuation-prompt-builder.js";
import { INSTRUCTIONS_MAX_LENGTH } from "../constants.js";
import type { HarnessConfig, HarnessLoopState, RunnerOutput } from "../types.js";

const projectRoot = join(tmpdir(), `harness-prompt-test-${randomUUID()}`);

function makeConfig(overrides: Partial<HarnessConfig> = {}): HarnessConfig {
  return {
    runner_path: "./run.sh",
    gates: ["pre-work", "in-progress", "pre-merge"],
    fail_policy: "hybrid",
    rule_id_format: "R{id}",
    max_total_iterations: 100,
    max_iterations_per_gate: 10,
    auto_fix_attempts: 3,
    cache_ttl_minutes: 30,
    runner_timeout_seconds: 300,
    completion_promise: "HARNESS-COMPLETE",
    ultrawork_verify_gates: [],
    state_file_path: ".opencode/harness-loop.local.json",
    gate_instructions: {},
    phase_hooks: {},
    strict_instructions: false,
    async_heartbeats: true,
    ...overrides,
  };
}

function makeState(overrides: Partial<HarnessLoopState["loop"]> = {}): HarnessLoopState {
  const loop = {
    active: true,
    current_gate: "pre-work",
    gate_iteration: 1,
    total_iteration: 1,
    max_iterations_per_gate: 10,
    max_total_iterations: 100,
    started_at: new Date().toISOString(),
    session_id: "sess-001",
    config_snapshot: makeConfig(),
    last_runner_output: null,
    no_progress_count: 0,
    override_active: false,
    same_error_history: {},
    verification_pending: false,
    watcher_task_id: null,
    message_count_at_start: 0,
    ...overrides,
  };
  return {
    feature_id: null,
    issue_number: null,
    story: null,
    updated_at: new Date().toISOString(),
    checkpoints: {},
    loop,
  };
}

function makeRunnerOutput(overrides: Partial<RunnerOutput> = {}): RunnerOutput {
  return {
    gate: "pre-work",
    status: "FAIL",
    checks: [],
    rule_ids_violated: [],
    ...overrides,
  };
}

describe("buildContinuationPrompt — FAIL status", () => {
  it("contains the gate name in the output", () => {
    const config = makeConfig();
    const state = makeState();
    const output = makeRunnerOutput({ gate: "pre-work", status: "FAIL" });

    const prompt = buildContinuationPrompt(state, output, config, projectRoot);

    expect(prompt).toContain("pre-work");
  });

  it("contains the rule IDs in the output", () => {
    const config = makeConfig({ rule_id_format: "R{id}" });
    const state = makeState();
    const output = makeRunnerOutput({
      gate: "pre-work",
      status: "FAIL",
      rule_ids_violated: ["89", "31"],
    });

    const prompt = buildContinuationPrompt(state, output, config, projectRoot);

    expect(prompt).toContain("R89");
    expect(prompt).toContain("R31");
  });

  it("numeric rule ID '89' is formatted as 'R89' with format 'R{id}'", () => {
    const config = makeConfig({ rule_id_format: "R{id}" });
    const state = makeState();
    const output = makeRunnerOutput({
      gate: "pre-work",
      status: "FAIL",
      rule_ids_violated: ["89"],
    });

    const prompt = buildContinuationPrompt(state, output, config, projectRoot);

    expect(prompt).toContain("R89");
  });

  it("pre-formatted 'R89' stays as 'R89' (no double-format)", () => {
    const config = makeConfig({ rule_id_format: "R{id}" });
    const state = makeState();
    const output = makeRunnerOutput({
      gate: "pre-work",
      status: "FAIL",
      rule_ids_violated: ["R89"],
    });

    const prompt = buildContinuationPrompt(state, output, config, projectRoot);

    expect(prompt).toContain("R89");
    expect(prompt).not.toContain("RR89");
  });

  it("instructions_for_agent longer than 8000 chars is truncated with '...[truncated]'", () => {
    const config = makeConfig();
    const state = makeState();
    const longInstructions = "x".repeat(INSTRUCTIONS_MAX_LENGTH + 500);
    const output = makeRunnerOutput({
      gate: "pre-work",
      status: "FAIL",
      instructions_for_agent: longInstructions,
    });

    const prompt = buildContinuationPrompt(state, output, config, projectRoot);

    expect(prompt).toContain("...[truncated]");
    const instructionsInPrompt = prompt.indexOf("...[truncated]");
    expect(instructionsInPrompt).toBeGreaterThan(-1);
  });

  it("instructions_for_agent exactly at 8000 chars is NOT truncated", () => {
    const config = makeConfig();
    const state = makeState();
    const exactInstructions = "y".repeat(INSTRUCTIONS_MAX_LENGTH);
    const output = makeRunnerOutput({
      gate: "pre-work",
      status: "FAIL",
      instructions_for_agent: exactInstructions,
    });

    const prompt = buildContinuationPrompt(state, output, config, projectRoot);

    expect(prompt).not.toContain("...[truncated]");
  });
});

describe("buildContinuationPrompt — BLOCKED status", () => {
  it("contains 'BLOCKED' in the output", () => {
    const config = makeConfig();
    const state = makeState();
    const output = makeRunnerOutput({ gate: "pre-work", status: "BLOCKED" });

    const prompt = buildContinuationPrompt(state, output, config, projectRoot);

    expect(prompt).toContain("BLOCKED");
  });

  it("contains the gate name when BLOCKED", () => {
    const config = makeConfig();
    const state = makeState();
    const output = makeRunnerOutput({ gate: "in-progress", status: "BLOCKED" });

    const prompt = buildContinuationPrompt(state, output, config, projectRoot);

    expect(prompt).toContain("in-progress");
  });
});
