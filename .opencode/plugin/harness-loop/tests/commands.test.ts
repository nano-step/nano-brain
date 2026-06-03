import {
  describe,
  it,
  expect,
  vi,
  afterEach,
} from "vitest";
import {
  mkdirSync,
  rmSync,
  existsSync,
  writeFileSync,
  chmodSync,
} from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { randomUUID } from "node:crypto";

vi.mock("../templates/opening-prompt.js", () => ({
  buildOpeningPrompt: vi.fn().mockReturnValue("opening prompt"),
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

import { handleHarnessOn, type HarnessOnContext } from "../commands/harness-on.js";
import { handleHarnessOff, type HarnessOffContext } from "../commands/harness-off.js";
import { createLoopStateController } from "../loop-state-controller.js";

const dirs: string[] = [];

function makeProjectRoot(): string {
  const dir = join(tmpdir(), `harness-cmd-test-${randomUUID()}`);
  mkdirSync(join(dir, ".opencode"), { recursive: true });
  dirs.push(dir);
  return dir;
}

function writeConfig(projectRoot: string, overrides: Record<string, unknown> = {}): void {
  const config = {
    runner_path: "./run.sh",
    gates: ["pre-work", "in-progress"],
    ...overrides,
  };
  writeFileSync(
    join(projectRoot, ".opencode", "harness.config.json"),
    JSON.stringify(config),
    "utf-8"
  );
}

function writeExecutableRunner(projectRoot: string): void {
  const runnerPath = join(projectRoot, "run.sh");
  writeFileSync(runnerPath, "#!/bin/bash\necho '{}'\n", "utf-8");
  chmodSync(runnerPath, 0o755);
}

function makeOnContext(projectRoot: string, sessionId: string): HarnessOnContext {
  return {
    projectRoot,
    sessionId,
    getMessageCount: () => 0,
    injectMessage: vi.fn().mockResolvedValue(undefined),
    showToast: vi.fn(),
    askQuestion: vi.fn().mockResolvedValue("Abort"),
  };
}

function makeOffContext(projectRoot: string): HarnessOffContext {
  return {
    projectRoot,
    showToast: vi.fn(),
  };
}

afterEach(() => {
  vi.clearAllMocks();
  for (const dir of dirs) {
    if (existsSync(dir)) {
      rmSync(dir, { recursive: true, force: true });
    }
  }
  dirs.length = 0;
});

describe("handleHarnessOn — start fresh loop", () => {
  it("creates state with loop.active=true after successful start", async () => {
    const projectRoot = makeProjectRoot();
    writeConfig(projectRoot);
    writeExecutableRunner(projectRoot);

    const ctx = makeOnContext(projectRoot, "sess-fresh");
    await handleHarnessOn(ctx, []);

    const state = createLoopStateController(projectRoot).getState();
    expect(state).not.toBeNull();
    expect(state?.loop.active).toBe(true);
    expect(state?.loop.session_id).toBe("sess-fresh");
    expect(state?.loop.current_gate).toBe("pre-work");
  });

  it("injects an opening prompt and shows started toast after start", async () => {
    const projectRoot = makeProjectRoot();
    writeConfig(projectRoot);
    writeExecutableRunner(projectRoot);

    const ctx = makeOnContext(projectRoot, "sess-prompt");
    await handleHarnessOn(ctx, []);

    expect(ctx.injectMessage).toHaveBeenCalledOnce();
    expect(ctx.showToast).toHaveBeenCalledWith(expect.stringContaining("started"), "info");
  });

  it("shows error toast when config file is missing", async () => {
    const projectRoot = makeProjectRoot();

    const ctx = makeOnContext(projectRoot, "sess-noconfig");
    await handleHarnessOn(ctx, []);

    expect(ctx.showToast).toHaveBeenCalledWith(
      expect.stringContaining("Config file not found"),
      "error"
    );
    expect(createLoopStateController(projectRoot).getState()?.loop.active).toBeFalsy();
  });

  it("shows error toast when runner is not found", async () => {
    const projectRoot = makeProjectRoot();
    writeConfig(projectRoot, { runner_path: "./missing.sh" });

    const ctx = makeOnContext(projectRoot, "sess-norunner");
    await handleHarnessOn(ctx, []);

    expect(ctx.showToast).toHaveBeenCalledWith(
      expect.stringContaining("Runner not found"),
      "error"
    );
  });

  it("shows error toast when runner is not executable", async () => {
    const projectRoot = makeProjectRoot();
    writeConfig(projectRoot);
    const runnerPath = join(projectRoot, "run.sh");
    writeFileSync(runnerPath, "#!/bin/bash\n", "utf-8");
    chmodSync(runnerPath, 0o644);

    const ctx = makeOnContext(projectRoot, "sess-notexec");
    await handleHarnessOn(ctx, []);

    expect(ctx.showToast).toHaveBeenCalledWith(
      expect.stringContaining("not executable"),
      "error"
    );
  });
});

describe("handleHarnessOn — double-start handling", () => {
  it("asks what to do when loop is already active", async () => {
    const projectRoot = makeProjectRoot();
    writeConfig(projectRoot);
    writeExecutableRunner(projectRoot);

    const ctx = makeOnContext(projectRoot, "sess-double");
    await handleHarnessOn(ctx, []);

    ctx.injectMessage = vi.fn().mockResolvedValue(undefined);
    ctx.showToast = vi.fn();

    await handleHarnessOn(ctx, []);

    expect(ctx.askQuestion).toHaveBeenCalledWith(
      expect.objectContaining({
        choices: expect.arrayContaining(["Resume", "Cancel and restart", "Abort"]),
      })
    );
  });

  it("shows aborted toast when user selects Abort on second start", async () => {
    const projectRoot = makeProjectRoot();
    writeConfig(projectRoot);
    writeExecutableRunner(projectRoot);

    const ctx = makeOnContext(projectRoot, "sess-abort");
    ctx.askQuestion = vi.fn().mockResolvedValue("Abort");

    await handleHarnessOn(ctx, []);
    await handleHarnessOn(ctx, []);

    expect(ctx.showToast).toHaveBeenCalledWith(
      expect.stringContaining("aborted"),
      "warning"
    );
    expect(createLoopStateController(projectRoot).getState()?.loop.active).toBe(true);
  });

  it("resumes loop in new session when user selects Resume", async () => {
    const projectRoot = makeProjectRoot();
    writeConfig(projectRoot);
    writeExecutableRunner(projectRoot);

    const firstCtx = makeOnContext(projectRoot, "sess-original");
    await handleHarnessOn(firstCtx, []);

    const secondCtx = makeOnContext(projectRoot, "sess-resume");
    secondCtx.askQuestion = vi.fn().mockResolvedValue("Resume");

    await handleHarnessOn(secondCtx, []);

    const state = createLoopStateController(projectRoot).getState();
    expect(state?.loop.session_id).toBe("sess-resume");
    expect(state?.loop.active).toBe(true);
  });

  it("restarts loop from first gate when user selects Cancel and restart", async () => {
    const projectRoot = makeProjectRoot();
    writeConfig(projectRoot);
    writeExecutableRunner(projectRoot);

    const firstCtx = makeOnContext(projectRoot, "sess-first");
    await handleHarnessOn(firstCtx, []);

    const controller = createLoopStateController(projectRoot);
    controller.transitionToGate("in-progress");
    expect(controller.getState()?.loop.current_gate).toBe("in-progress");

    const secondCtx = makeOnContext(projectRoot, "sess-restart");
    secondCtx.askQuestion = vi.fn().mockResolvedValue("Cancel and restart");

    await handleHarnessOn(secondCtx, []);

    const state = createLoopStateController(projectRoot).getState();
    expect(state?.loop.active).toBe(true);
    expect(state?.loop.session_id).toBe("sess-restart");
    expect(state?.loop.current_gate).toBe("pre-work");
  });
});

describe("handleHarnessOff — cancel loop", () => {
  it("shows info toast when no active loop exists", async () => {
    const projectRoot = makeProjectRoot();
    const offCtx = makeOffContext(projectRoot);
    await handleHarnessOff(offCtx);

    expect(offCtx.showToast).toHaveBeenCalledWith(
      expect.stringContaining("No active harness loop"),
      "info"
    );
  });

  it("cancels an active loop and shows cancelled toast", async () => {
    const projectRoot = makeProjectRoot();
    writeConfig(projectRoot);
    writeExecutableRunner(projectRoot);

    const onCtx = makeOnContext(projectRoot, "sess-off");
    await handleHarnessOn(onCtx, []);
    expect(createLoopStateController(projectRoot).getState()?.loop.active).toBe(true);

    const offCtx = makeOffContext(projectRoot);
    await handleHarnessOff(offCtx);

    expect(createLoopStateController(projectRoot).getState()?.loop.active).toBe(false);
    expect(offCtx.showToast).toHaveBeenCalledWith(
      expect.stringContaining("cancelled"),
      "info"
    );
  });

  it("harness-on then harness-off leaves loop.active as false", async () => {
    const projectRoot = makeProjectRoot();
    writeConfig(projectRoot);
    writeExecutableRunner(projectRoot);

    const onCtx = makeOnContext(projectRoot, "sess-toggle");
    await handleHarnessOn(onCtx, []);

    const offCtx = makeOffContext(projectRoot);
    await handleHarnessOff(offCtx);

    expect(createLoopStateController(projectRoot).getState()?.loop.active).toBe(false);
  });

  it("cancels watcher background task when one is active", async () => {
    const projectRoot = makeProjectRoot();
    writeConfig(projectRoot);
    writeExecutableRunner(projectRoot);

    const onCtx = makeOnContext(projectRoot, "sess-watcher");
    await handleHarnessOn(onCtx, []);

    const controller = createLoopStateController(projectRoot);
    controller.setWatcherTaskId("task-xyz");

    const cancelFn = vi.fn().mockResolvedValue(undefined);
    const offCtx: HarnessOffContext = {
      projectRoot,
      showToast: vi.fn(),
      cancelBackgroundTask: cancelFn,
    };

    await handleHarnessOff(offCtx);

    expect(cancelFn).toHaveBeenCalledWith("task-xyz");
  });
});
