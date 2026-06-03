import {
  describe,
  it,
  expect,
  vi,
  afterEach,
} from "vitest";
import { mkdirSync, rmSync, existsSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { randomUUID } from "node:crypto";
import { DEFAULT_STATE_FILE_PATH } from "../constants.js";
import type { HarnessConfig, RunnerOutput } from "../types.js";

vi.mock("../runner-invoker.js", () => ({
  invokeRunner: vi.fn(),
}));

vi.mock("../continuation-prompt-builder.js", () => ({
  buildContinuationPrompt: vi.fn().mockReturnValue("fix this"),
}));

vi.mock("../gate-instructions-resolver.js", () => ({
  resolveGateInstructions: vi.fn().mockReturnValue({
    docPath: null,
    skills: [],
    warning: null,
    isAsync: false,
    asyncConfig: null,
  }),
}));

vi.mock("../async-watcher-result-handler.js", () => ({
  collectWatcherResult: vi.fn().mockResolvedValue(null),
}));

vi.mock("../templates/continuation-prompt.js", () => ({
  buildCompletionPrompt: vi.fn().mockReturnValue("loop complete"),
}));
vi.mock("../templates/ultrawork-verification.js", () => ({
  buildUltraworkVerificationPrompt: vi.fn().mockReturnValue("verify"),
}));
vi.mock("../templates/opening-prompt.js", () => ({
  buildOpeningPrompt: vi.fn().mockReturnValue("opening"),
}));

vi.mock("../storage.js", async (importOriginal) => {
  const original = await importOriginal<typeof import("../storage.js")>();
  return {
    ...original,
    clearLoopBlock: (statePath: string) => {
      const state = original.readState(statePath);
      if (state === null) return;
      state.loop.active = false;
      state.loop.current_gate = "";
      state.loop.session_id = "";
      state.updated_at = new Date().toISOString();
      original.writeState(statePath, state);
    },
  };
});

import { handleSessionIdle, type PluginContext } from "../harness-loop-event-handler.js";
import { invokeRunner } from "../runner-invoker.js";
import { createLoopStateController } from "../loop-state-controller.js";
import { writeState, readState, getStatePath } from "../storage.js";

const mockedInvokeRunner = vi.mocked(invokeRunner);

const dirs: string[] = [];

function makeProjectRoot(): string {
  const dir = join(tmpdir(), `harness-eh-test-${randomUUID()}`);
  mkdirSync(join(dir, ".opencode"), { recursive: true });
  dirs.push(dir);
  return dir;
}

function makeConfig(overrides: Partial<HarnessConfig> = {}): HarnessConfig {
  return {
    runner_path: "./run.sh",
    gates: ["pre-work", "in-progress", "pre-merge"],
    fail_policy: "auto",
    rule_id_format: "{id}",
    max_total_iterations: 100,
    max_iterations_per_gate: 10,
    auto_fix_attempts: 3,
    cache_ttl_minutes: 0,
    runner_timeout_seconds: 300,
    completion_promise: "HARNESS-COMPLETE",
    ultrawork_verify_gates: [],
    state_file_path: DEFAULT_STATE_FILE_PATH,
    gate_instructions: {},
    phase_hooks: {},
    strict_instructions: false,
    async_heartbeats: false,
    ...overrides,
  };
}

function makeContext(
  projectRoot: string,
  sessionId: string,
  messages: Array<{ role: string; content: string }> = []
): PluginContext {
  return {
    sessionId,
    projectRoot,
    getMessages: () => messages,
    injectMessage: vi.fn().mockResolvedValue(undefined),
    showToast: vi.fn(),
    hasActiveBackgroundTasks: () => false,
    latestUserMessageTimestamp: () => Date.now() - 60_000,
  };
}

function startLoop(projectRoot: string, sessionId: string, config: HarnessConfig) {
  return createLoopStateController(projectRoot).startLoop(sessionId, config);
}

function makePassOutput(gate: string, nextGate?: string | null): RunnerOutput {
  return {
    gate,
    status: "PASS",
    checks: [],
    rule_ids_violated: [],
    next_gate: nextGate === undefined ? null : nextGate,
  };
}

function makeFailOutput(gate: string): RunnerOutput {
  return {
    gate,
    status: "FAIL",
    checks: [],
    rule_ids_violated: ["R1"],
    instructions_for_agent: "Fix the issue",
  };
}

async function runIdle(ctx: PluginContext): Promise<void> {
  vi.useFakeTimers();
  const p = handleSessionIdle(ctx);
  await vi.runAllTimersAsync();
  await p;
  vi.useRealTimers();
}

afterEach(() => {
  vi.useRealTimers();
  vi.clearAllMocks();
  for (const dir of dirs) {
    if (existsSync(dir)) {
      rmSync(dir, { recursive: true, force: true });
    }
  }
  dirs.length = 0;
});

describe("handleSessionIdle — guard conditions", () => {
  it("returns early without calling runner when no state file exists", async () => {
    const projectRoot = makeProjectRoot();
    const ctx = makeContext(projectRoot, "sess-1");
    await runIdle(ctx);
    expect(mockedInvokeRunner).not.toHaveBeenCalled();
  });

  it("returns early without calling runner when loop.active is false", async () => {
    const projectRoot = makeProjectRoot();
    const sessionId = "sess-2";
    createLoopStateController(projectRoot).startLoop(sessionId, makeConfig());
    const statePath = getStatePath(projectRoot);
    const state = readState(statePath)!;
    state.loop.active = false;
    writeState(statePath, state);

    const ctx = makeContext(projectRoot, sessionId);
    await runIdle(ctx);
    expect(mockedInvokeRunner).not.toHaveBeenCalled();
  });

  it("returns early when session ID does not match", async () => {
    const projectRoot = makeProjectRoot();
    startLoop(projectRoot, "session-A", makeConfig());
    const ctx = makeContext(projectRoot, "session-B");
    await runIdle(ctx);
    expect(mockedInvokeRunner).not.toHaveBeenCalled();
  });

  it("returns early when user message was sent too recently", async () => {
    const projectRoot = makeProjectRoot();
    const sessionId = "sess-recent";
    startLoop(projectRoot, sessionId, makeConfig());
    const ctx = makeContext(projectRoot, sessionId);
    ctx.latestUserMessageTimestamp = () => Date.now() - 500;
    await runIdle(ctx);
    expect(mockedInvokeRunner).not.toHaveBeenCalled();
  });

  it("returns early when active background tasks exist", async () => {
    const projectRoot = makeProjectRoot();
    const sessionId = "sess-tasks";
    startLoop(projectRoot, sessionId, makeConfig());
    const ctx = makeContext(projectRoot, sessionId);
    ctx.hasActiveBackgroundTasks = () => true;
    await runIdle(ctx);
    expect(mockedInvokeRunner).not.toHaveBeenCalled();
  });
});

describe("handleSessionIdle — FAIL then PASS gate transitions", () => {
  it("injects fix prompt and keeps current gate on FAIL", async () => {
    const projectRoot = makeProjectRoot();
    const sessionId = "sess-fail";
    startLoop(projectRoot, sessionId, makeConfig());
    mockedInvokeRunner.mockResolvedValueOnce(makeFailOutput("pre-work"));

    const ctx = makeContext(projectRoot, sessionId);
    await runIdle(ctx);

    expect(ctx.injectMessage).toHaveBeenCalledOnce();
    expect(ctx.showToast).toHaveBeenCalledWith(expect.stringContaining("pre-work"), "warning");

    const state = createLoopStateController(projectRoot).getState();
    expect(state?.loop.current_gate).toBe("pre-work");
    expect(state?.loop.active).toBe(true);
  });

  it("transitions to next gate on PASS", async () => {
    const projectRoot = makeProjectRoot();
    const sessionId = "sess-pass";
    startLoop(projectRoot, sessionId, makeConfig());
    mockedInvokeRunner.mockResolvedValueOnce(makePassOutput("pre-work"));

    const ctx = makeContext(projectRoot, sessionId);
    await runIdle(ctx);

    const state = createLoopStateController(projectRoot).getState();
    expect(state?.loop.current_gate).toBe("in-progress");
    expect(state?.loop.active).toBe(true);
  });

  it("shows completion toast and injects prompt when last gate PASSes", async () => {
    const projectRoot = makeProjectRoot();
    const sessionId = "sess-complete";
    startLoop(projectRoot, sessionId, makeConfig({ gates: ["only-gate"] }));
    mockedInvokeRunner.mockResolvedValueOnce(makePassOutput("only-gate", null));

    const ctx = makeContext(projectRoot, sessionId);
    await runIdle(ctx);

    expect(ctx.showToast).toHaveBeenCalledWith(expect.stringContaining("complete"), "info");
    expect(ctx.injectMessage).toHaveBeenCalledOnce();
    expect(createLoopStateController(projectRoot).getState()?.loop.active).toBe(false);
  });
});

describe("handleSessionIdle — 5-iteration simulated loop (task 10.16)", () => {
  it("FAIL x2 on pre-work, then PASS through all 3 gates, loop ends", async () => {
    const projectRoot = makeProjectRoot();
    const sessionId = "sess-5iter";
    startLoop(projectRoot, sessionId, makeConfig());
    const ctx = makeContext(projectRoot, sessionId);

    mockedInvokeRunner.mockResolvedValueOnce(makeFailOutput("pre-work"));
    await runIdle(ctx);
    expect(createLoopStateController(projectRoot).getState()?.loop.current_gate).toBe("pre-work");

    mockedInvokeRunner.mockResolvedValueOnce(makeFailOutput("pre-work"));
    await runIdle(ctx);
    expect(createLoopStateController(projectRoot).getState()?.loop.current_gate).toBe("pre-work");

    mockedInvokeRunner.mockResolvedValueOnce(makePassOutput("pre-work"));
    await runIdle(ctx);
    expect(createLoopStateController(projectRoot).getState()?.loop.current_gate).toBe("in-progress");

    mockedInvokeRunner.mockResolvedValueOnce(makePassOutput("in-progress"));
    await runIdle(ctx);
    expect(createLoopStateController(projectRoot).getState()?.loop.current_gate).toBe("pre-merge");

    mockedInvokeRunner.mockResolvedValueOnce(makePassOutput("pre-merge", null));
    await runIdle(ctx);

    const finalState = createLoopStateController(projectRoot).getState();
    expect(finalState?.loop.active).toBe(false);
    expect(ctx.showToast).toHaveBeenCalledWith(expect.stringContaining("complete"), "info");
    expect(ctx.injectMessage).toHaveBeenCalled();
  });
});

describe("handleSessionIdle — max total iterations hard cap", () => {
  it("shows error toast and skips runner when total iterations exceeded", async () => {
    const projectRoot = makeProjectRoot();
    const sessionId = "sess-maxiter";
    const config = makeConfig({ max_total_iterations: 3 });

    const controller = createLoopStateController(projectRoot);
    controller.startLoop(sessionId, config);
    controller.incrementTotalIteration();
    controller.incrementTotalIteration();

    const ctx = makeContext(projectRoot, sessionId);
    await runIdle(ctx);

    expect(ctx.showToast).toHaveBeenCalledWith(
      expect.stringContaining("Max total iterations"),
      "error"
    );
    expect(mockedInvokeRunner).not.toHaveBeenCalled();
    expect(createLoopStateController(projectRoot).getState()?.loop.active).toBe(false);
  });
});

describe("handleSessionIdle — no-progress guard", () => {
  it("shows error toast after 3 consecutive empty assistant turns", async () => {
    const projectRoot = makeProjectRoot();
    const sessionId = "sess-noprogress";
    startLoop(projectRoot, sessionId, makeConfig());

    const messages = [{ role: "assistant", content: "" }];
    const ctx = makeContext(projectRoot, sessionId, messages);

    mockedInvokeRunner.mockResolvedValue(makeFailOutput("pre-work"));

    await runIdle(ctx);
    await runIdle(ctx);
    await runIdle(ctx);

    expect(ctx.showToast).toHaveBeenCalledWith(
      expect.stringContaining("No progress"),
      "error"
    );
    expect(createLoopStateController(projectRoot).getState()?.loop.active).toBe(false);
  });
});

describe("createLoopStateController — state machine unit tests", () => {
  it("starts loop with correct initial field values", () => {
    const projectRoot = makeProjectRoot();
    const controller = createLoopStateController(projectRoot);
    const state = controller.startLoop("sess-sm", makeConfig());

    expect(state.loop.active).toBe(true);
    expect(state.loop.current_gate).toBe("pre-work");
    expect(state.loop.gate_iteration).toBe(1);
    expect(state.loop.total_iteration).toBe(1);
  });

  it("transitionToGate resets gate_iteration and advances the gate", () => {
    const projectRoot = makeProjectRoot();
    const controller = createLoopStateController(projectRoot);
    controller.startLoop("sess-trans", makeConfig());
    controller.incrementGateIteration();
    controller.transitionToGate("in-progress");

    const state = controller.getState()!;
    expect(state.loop.current_gate).toBe("in-progress");
    expect(state.loop.gate_iteration).toBe(1);
  });

  it("incrementNoProgress accumulates; resetNoProgress clears to 0", () => {
    const projectRoot = makeProjectRoot();
    const controller = createLoopStateController(projectRoot);
    controller.startLoop("sess-np", makeConfig());

    controller.incrementNoProgress();
    controller.incrementNoProgress();
    expect(controller.getState()?.loop.no_progress_count).toBe(2);

    controller.resetNoProgress();
    expect(controller.getState()?.loop.no_progress_count).toBe(0);
  });

  it("throws on double startLoop when loop is already active", () => {
    const projectRoot = makeProjectRoot();
    const controller = createLoopStateController(projectRoot);
    controller.startLoop("sess-dbl", makeConfig());

    expect(() => controller.startLoop("sess-dbl2", makeConfig())).toThrow();
  });

  it("recordRunnerOutput persists checkpoint with correct status", () => {
    const projectRoot = makeProjectRoot();
    const controller = createLoopStateController(projectRoot);
    controller.startLoop("sess-ro", makeConfig());
    controller.recordRunnerOutput(makePassOutput("pre-work"));

    const state = controller.getState()!;
    expect(state.checkpoints["pre-work"]?.status).toBe("PASS");
  });
});
