import { describe, it, expect } from "vitest";
import {
  detectCompletion,
  detectOverrideToken,
} from "../completion-detector.js";
import type { HarnessConfig, HarnessLoopState } from "../types.js";

function makeConfig(overrides: Partial<HarnessConfig> = {}): HarnessConfig {
  return {
    runner_path: "./run.sh",
    gates: ["gate-a", "gate-b"],
    fail_policy: "hybrid",
    rule_id_format: "{id}",
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
    current_gate: "gate-a",
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

describe("detectCompletion — promise tag", () => {
  it("returns 'promise_tag' when assistant message contains the promise tag", () => {
    const config = makeConfig({ completion_promise: "HARNESS-COMPLETE" });
    const state = makeState({ message_count_at_start: 0 });
    const messages = [
      { role: "user", content: "please finish" },
      { role: "assistant", content: "Done! <promise>HARNESS-COMPLETE</promise>" },
    ];

    const result = detectCompletion(messages, state, null, config);

    expect(result).toBe("promise_tag");
  });

  it("returns null when promise tag is absent", () => {
    const config = makeConfig({ completion_promise: "HARNESS-COMPLETE" });
    const state = makeState({ message_count_at_start: 0 });
    const messages = [
      { role: "assistant", content: "Still working..." },
    ];

    const result = detectCompletion(messages, state, null, config);

    expect(result).toBeNull();
  });

  it("returns null when promise tag is only in messages before messageCountAtStart", () => {
    const config = makeConfig({ completion_promise: "HARNESS-COMPLETE" });
    const state = makeState({ message_count_at_start: 2 });
    const messages = [
      { role: "assistant", content: "<promise>HARNESS-COMPLETE</promise>" },
      { role: "user", content: "ok, continue" },
      { role: "assistant", content: "Starting new loop iteration." },
    ];

    const result = detectCompletion(messages, state, null, config);

    expect(result).toBeNull();
  });
});

describe("detectOverrideToken", () => {
  it("returns found=true and extracts reason from last assistant message", () => {
    const messages = [
      { role: "user", content: "fix it" },
      { role: "assistant", content: "I tried but failed.\n[HARNESS-OVERRIDE]: blocked by external dependency" },
    ];

    const result = detectOverrideToken(messages);

    expect(result.found).toBe(true);
    expect(result.reason).toBe("blocked by external dependency");
  });

  it("returns found=false when no override token present", () => {
    const messages = [
      { role: "assistant", content: "All done, no issues." },
    ];

    const result = detectOverrideToken(messages);

    expect(result.found).toBe(false);
    expect(result.reason).toBeNull();
  });

  it("only checks the last assistant message", () => {
    const messages = [
      { role: "assistant", content: "[HARNESS-OVERRIDE]: old override" },
      { role: "user", content: "ok" },
      { role: "assistant", content: "No override here." },
    ];

    const result = detectOverrideToken(messages);

    expect(result.found).toBe(false);
  });

  it("returns found=false when message list is empty", () => {
    const result = detectOverrideToken([]);

    expect(result.found).toBe(false);
    expect(result.reason).toBeNull();
  });
});
