import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { EventEmitter } from "node:events";
import type { ChildProcess } from "node:child_process";

// Mock node:child_process before importing the module under test
vi.mock("node:child_process", () => ({
  spawn: vi.fn(),
}));

// Mock node:fs to control existsSync / accessSync
vi.mock("node:fs", () => ({
  existsSync: vi.fn(),
  accessSync: vi.fn(),
  constants: { X_OK: 1 },
}));

import { spawn } from "node:child_process";
import { existsSync, accessSync } from "node:fs";
import { invokeRunner } from "../runner-invoker.js";
import type { HarnessConfig } from "../types.js";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const mockSpawn = vi.mocked(spawn);
const mockExistsSync = vi.mocked(existsSync);
const mockAccessSync = vi.mocked(accessSync);

/** Minimal valid HarnessConfig for tests. */
function makeConfig(overrides: Partial<HarnessConfig> = {}): HarnessConfig {
  return {
    runner_path: "./scripts/harness-check.sh",
    gates: ["pre-work"],
    fail_policy: "hybrid",
    rule_id_format: "{id}",
    max_total_iterations: 100,
    max_iterations_per_gate: 10,
    auto_fix_attempts: 3,
    cache_ttl_minutes: 30,
    runner_timeout_seconds: 5,
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

/**
 * Creates a fake ChildProcess that emits stdout data then closes.
 * exitCode defaults to 0.
 */
function fakeProc(
  stdoutData: string | null,
  exitCode: number = 0,
  delayMs: number = 0
): ChildProcess {
  const proc = new EventEmitter() as unknown as ChildProcess;
  const stdoutEmitter = new EventEmitter();
  (proc as unknown as Record<string, unknown>)["stdout"] = stdoutEmitter;
  (proc as unknown as Record<string, unknown>)["stderr"] = new EventEmitter();
  (proc as unknown as Record<string, unknown>)["kill"] = vi.fn();

  setTimeout(() => {
    if (stdoutData !== null) {
      stdoutEmitter.emit("data", Buffer.from(stdoutData));
    }
    proc.emit("close", exitCode);
  }, delayMs);

  return proc;
}

/** Creates a fake ChildProcess that emits an error event (e.g. ENOENT). */
function fakeProcWithError(err: NodeJS.ErrnoException): ChildProcess {
  const proc = new EventEmitter() as unknown as ChildProcess;
  (proc as unknown as Record<string, unknown>)["stdout"] = new EventEmitter();
  (proc as unknown as Record<string, unknown>)["stderr"] = new EventEmitter();
  (proc as unknown as Record<string, unknown>)["kill"] = vi.fn();

  setTimeout(() => {
    proc.emit("error", err);
  }, 0);

  return proc;
}

// ---------------------------------------------------------------------------
// Default setup: runner exists and is executable
// ---------------------------------------------------------------------------

beforeEach(() => {
  mockExistsSync.mockReturnValue(true);
  mockAccessSync.mockReturnValue(undefined);
});

afterEach(() => {
  vi.clearAllMocks();
});

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("invokeRunner", () => {
  it("happy path — valid JSON with matching gate returns parsed RunnerOutput", async () => {
    const validOutput = {
      gate: "pre-work",
      status: "PASS",
      checks: [],
      rule_ids_violated: [],
    };

    mockSpawn.mockReturnValue(fakeProc(JSON.stringify(validOutput)));

    const result = await invokeRunner(makeConfig(), "pre-work", "/project");

    expect(result.gate).toBe("pre-work");
    expect(result.status).toBe("PASS");
    expect(result.checks).toEqual([]);
  });

  it("timeout — runner exceeds timeoutMs and returns synthetic ERROR output", async () => {
    vi.useFakeTimers();

    // Process never closes naturally — needs to be killed
    const proc = new EventEmitter() as unknown as ChildProcess;
    const stdoutEmitter = new EventEmitter();
    (proc as unknown as Record<string, unknown>)["stdout"] = stdoutEmitter;
    (proc as unknown as Record<string, unknown>)["stderr"] = new EventEmitter();
    const killFn = vi.fn().mockImplementation((signal: string) => {
      if (signal === "SIGTERM") {
        // Simulate process dying after SIGTERM
        setTimeout(() => proc.emit("close", 143), 0);
      }
    });
    (proc as unknown as Record<string, unknown>)["kill"] = killFn;

    mockSpawn.mockReturnValue(proc);

    const config = makeConfig({ runner_timeout_seconds: 1 });
    const resultPromise = invokeRunner(config, "pre-work", "/project");

    // Advance past the 1s timeout
    await vi.runAllTimersAsync();

    const result = await resultPromise;

    expect(result.status).toBe("ERROR");
    expect(result.gate).toBe("pre-work");
    expect(result.instructions_for_agent).toMatch(/timed out/i);

    vi.useRealTimers();
  });

  it("non-zero exit code — still returns parsed output (exit code mismatch is warned, not fatal)", async () => {
    // Per runner-invoker.ts: non-zero exit just warns — the JSON content is still parsed
    const validOutput = {
      gate: "pre-work",
      status: "FAIL",
      checks: [],
      rule_ids_violated: [],
      instructions_for_agent: "Something failed",
    };

    mockSpawn.mockReturnValue(fakeProc(JSON.stringify(validOutput), 99));

    const result = await invokeRunner(makeConfig(), "pre-work", "/project");

    expect(result.status).toBe("FAIL");
    expect(result.gate).toBe("pre-work");
  });

  it("valid JSON but wrong gate — returns synthetic ERROR with gate mismatch message", async () => {
    const wrongGateOutput = {
      gate: "post-merge",
      status: "PASS",
      checks: [],
      rule_ids_violated: [],
    };

    mockSpawn.mockReturnValue(fakeProc(JSON.stringify(wrongGateOutput)));

    const result = await invokeRunner(makeConfig(), "pre-work", "/project");

    expect(result.status).toBe("ERROR");
    expect(result.gate).toBe("pre-work");
    expect(result.instructions_for_agent).toMatch(/gate mismatch/i);
  });

  it("non-JSON stdout — returns synthetic ERROR with parse error message", async () => {
    mockSpawn.mockReturnValue(fakeProc("hello world\n"));

    const result = await invokeRunner(makeConfig(), "pre-work", "/project");

    expect(result.status).toBe("ERROR");
    expect(result.gate).toBe("pre-work");
    expect(result.instructions_for_agent).toBeTruthy();
  });

  it("missing required field (no status) — returns synthetic ERROR with schema violation message", async () => {
    const missingStatus = {
      gate: "pre-work",
      checks: [],
      rule_ids_violated: [],
      // status is intentionally absent
    };

    mockSpawn.mockReturnValue(fakeProc(JSON.stringify(missingStatus)));

    const result = await invokeRunner(makeConfig(), "pre-work", "/project");

    expect(result.status).toBe("ERROR");
    expect(result.gate).toBe("pre-work");
    expect(result.instructions_for_agent).toMatch(/contract violation/i);
  });

  it("empty stdout — returns synthetic ERROR", async () => {
    mockSpawn.mockReturnValue(fakeProc(""));

    const result = await invokeRunner(makeConfig(), "pre-work", "/project");

    expect(result.status).toBe("ERROR");
    expect(result.gate).toBe("pre-work");
    expect(result.instructions_for_agent).toMatch(/no output/i);
  });

  it("runner not found (existsSync returns false) — returns synthetic ERROR without spawning", async () => {
    mockExistsSync.mockReturnValue(false);

    const result = await invokeRunner(makeConfig(), "pre-work", "/project");

    expect(result.status).toBe("ERROR");
    expect(result.gate).toBe("pre-work");
    expect(result.instructions_for_agent).toMatch(/runner not found/i);
    expect(mockSpawn).not.toHaveBeenCalled();
  });

  it("spawn emits ENOENT error — returns synthetic ERROR with spawn failure message", async () => {
    const err = Object.assign(new Error("spawn ENOENT"), {
      code: "ENOENT",
    }) as NodeJS.ErrnoException;

    mockSpawn.mockReturnValue(fakeProcWithError(err));

    const result = await invokeRunner(makeConfig(), "pre-work", "/project");

    expect(result.status).toBe("ERROR");
    expect(result.gate).toBe("pre-work");
    expect(result.instructions_for_agent).toMatch(/failed to spawn/i);
  });
});
