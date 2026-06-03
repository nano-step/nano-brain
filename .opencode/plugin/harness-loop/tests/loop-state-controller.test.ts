import { describe, it, expect, afterEach } from "vitest";
import { mkdirSync, rmSync, existsSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { randomUUID } from "node:crypto";
import {
  createLoopStateController,
  LoopAlreadyActiveError,
} from "../loop-state-controller.js";
import { StateCorruptionError } from "../types.js";
import type { HarnessConfig } from "../types.js";

function makeTempRoot(): string {
  const dir = join(tmpdir(), `harness-ctrl-test-${randomUUID()}`);
  mkdirSync(join(dir, ".opencode"), { recursive: true });
  return dir;
}

const roots: string[] = [];

afterEach(() => {
  for (const root of roots) {
    if (existsSync(root)) {
      rmSync(root, { recursive: true, force: true });
    }
  }
  roots.length = 0;
});

function makeConfig(gates: string[] = ["gate-a", "gate-b"]): HarnessConfig {
  return {
    runner_path: "./scripts/run.sh",
    gates,
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
  };
}

describe("startLoop", () => {
  it("seeds state correctly on first start", () => {
    const root = makeTempRoot();
    roots.push(root);
    const ctrl = createLoopStateController(root);
    const config = makeConfig(["pre-work", "in-progress"]);

    const state = ctrl.startLoop("sess-001", config, "feat-1", 10, "story.md", 5);

    expect(state.loop.active).toBe(true);
    expect(state.loop.current_gate).toBe("pre-work");
    expect(state.loop.gate_iteration).toBe(1);
    expect(state.loop.total_iteration).toBe(1);
    expect(state.loop.session_id).toBe("sess-001");
    expect(state.loop.message_count_at_start).toBe(5);
    expect(state.feature_id).toBe("feat-1");
    expect(state.issue_number).toBe(10);
    expect(state.story).toBe("story.md");
  });

  it("throws LoopAlreadyActiveError when loop is already active", () => {
    const root = makeTempRoot();
    roots.push(root);
    const ctrl = createLoopStateController(root);
    const config = makeConfig();

    ctrl.startLoop("sess-001", config);

    expect(() => ctrl.startLoop("sess-002", config)).toThrow(LoopAlreadyActiveError);
  });

  it("LoopAlreadyActiveError contains session and gate info", () => {
    const root = makeTempRoot();
    roots.push(root);
    const ctrl = createLoopStateController(root);
    const config = makeConfig(["gate-x"]);

    ctrl.startLoop("sess-original", config);

    let caught: unknown;
    try {
      ctrl.startLoop("sess-new", config);
    } catch (err) {
      caught = err;
    }

    expect(caught).toBeInstanceOf(LoopAlreadyActiveError);
    const err = caught as LoopAlreadyActiveError;
    expect(err.existingSessionId).toBe("sess-original");
    expect(err.existingGate).toBe("gate-x");
  });
});

describe("transitionToGate", () => {
  it("resets gate_iteration to 1 and increments total_iteration", () => {
    const root = makeTempRoot();
    roots.push(root);
    const ctrl = createLoopStateController(root);
    const config = makeConfig(["gate-a", "gate-b"]);

    ctrl.startLoop("sess-001", config);
    ctrl.transitionToGate("gate-b");

    const state = ctrl.getState();
    expect(state!.loop.current_gate).toBe("gate-b");
    expect(state!.loop.gate_iteration).toBe(1);
    expect(state!.loop.total_iteration).toBe(2);
  });
});

describe("recordSameErrorHistory", () => {
  it("sliding window stays at 5 entries max", () => {
    const root = makeTempRoot();
    roots.push(root);
    const ctrl = createLoopStateController(root);
    const config = makeConfig();

    ctrl.startLoop("sess-001", config);

    for (let i = 0; i < 7; i++) {
      ctrl.recordSameErrorHistory("gate-a", [`R${i}`]);
    }

    const state = ctrl.getState();
    const history = state!.loop.same_error_history["gate-a"];
    expect(history).toBeDefined();
    expect(history!.length).toBe(5);
    expect(history![0]).toEqual(["R2"]);
    expect(history![4]).toEqual(["R6"]);
  });

  it("does not push when ruleIds is empty", () => {
    const root = makeTempRoot();
    roots.push(root);
    const ctrl = createLoopStateController(root);
    const config = makeConfig();

    ctrl.startLoop("sess-001", config);
    ctrl.recordSameErrorHistory("gate-a", []);

    const state = ctrl.getState();
    expect(state!.loop.same_error_history["gate-a"]).toBeUndefined();
  });
});

describe("cancelLoop", () => {
  it("is a no-op when no loop state exists", () => {
    const root = makeTempRoot();
    roots.push(root);
    const ctrl = createLoopStateController(root);

    expect(() => ctrl.cancelLoop()).not.toThrow();
    expect(ctrl.isActive()).toBe(false);
  });

  it("throws StateCorruptionError when clearing an active loop (clearLoopBlock schema enforcement)", () => {
    const root = makeTempRoot();
    roots.push(root);
    const ctrl = createLoopStateController(root);
    const config = makeConfig();

    ctrl.startLoop("sess-001", config);
    expect(ctrl.isActive()).toBe(true);

    expect(() => ctrl.cancelLoop()).toThrow(StateCorruptionError);
  });
});
