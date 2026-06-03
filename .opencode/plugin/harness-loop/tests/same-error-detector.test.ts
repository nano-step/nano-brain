import { describe, it, expect } from "vitest";
import { hasRepeatedSameError } from "../same-error-detector.js";
import type { HarnessLoopState } from "../types.js";

function makeState(
  history: Record<string, string[][]>
): HarnessLoopState {
  return {
    feature_id: null,
    issue_number: null,
    story: null,
    updated_at: new Date().toISOString(),
    checkpoints: {},
    loop: {
      active: true,
      current_gate: "gate-a",
      gate_iteration: 1,
      total_iteration: 1,
      max_iterations_per_gate: 10,
      max_total_iterations: 100,
      started_at: new Date().toISOString(),
      session_id: "sess-001",
      config_snapshot: {
        runner_path: "./run.sh",
        gates: ["gate-a"],
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
      },
      last_runner_output: null,
      no_progress_count: 0,
      override_active: false,
      same_error_history: history,
      verification_pending: false,
      watcher_task_id: null,
      message_count_at_start: 0,
    },
  };
}

describe("hasRepeatedSameError", () => {
  it("returns false when there is no history for the gate", () => {
    const state = makeState({});
    expect(hasRepeatedSameError(state, "gate-a")).toBe(false);
  });

  it("returns false when history has fewer than 3 entries", () => {
    const state = makeState({
      "gate-a": [["R1", "R2"], ["R1", "R2"]],
    });
    expect(hasRepeatedSameError(state, "gate-a")).toBe(false);
  });

  it("returns true when last 3 entries are identical non-empty sets", () => {
    const state = makeState({
      "gate-a": [
        ["R5"],
        ["R1", "R2"],
        ["R1", "R2"],
        ["R1", "R2"],
      ],
    });
    expect(hasRepeatedSameError(state, "gate-a")).toBe(true);
  });

  it("returns false when last 3 entries differ", () => {
    const state = makeState({
      "gate-a": [
        ["R1", "R2"],
        ["R1", "R3"],
        ["R1", "R2"],
      ],
    });
    expect(hasRepeatedSameError(state, "gate-a")).toBe(false);
  });

  it("returns false when any of the last 3 entries is empty", () => {
    const state = makeState({
      "gate-a": [
        ["R1"],
        [],
        ["R1"],
      ],
    });
    expect(hasRepeatedSameError(state, "gate-a")).toBe(false);
  });

  it("compares sets order-independently (sorted comparison)", () => {
    const state = makeState({
      "gate-a": [
        ["R2", "R1"],
        ["R1", "R2"],
        ["R1", "R2"],
      ],
    });
    expect(hasRepeatedSameError(state, "gate-a")).toBe(true);
  });
});
