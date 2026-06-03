import { describe, it, expect, afterEach } from "vitest";
import { writeFileSync, mkdirSync, rmSync, existsSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { randomUUID } from "node:crypto";
import { loadConfig } from "../config-loader.js";
import { HarnessConfigError } from "../types.js";

function makeTempDir(): string {
  const dir = join(tmpdir(), `harness-test-${randomUUID()}`);
  mkdirSync(dir, { recursive: true });
  mkdirSync(join(dir, ".opencode"), { recursive: true });
  return dir;
}

function writeConfig(dir: string, content: unknown): void {
  writeFileSync(
    join(dir, ".opencode", "harness.config.json"),
    typeof content === "string" ? content : JSON.stringify(content),
    "utf-8"
  );
}

const dirs: string[] = [];

afterEach(() => {
  for (const dir of dirs) {
    if (existsSync(dir)) {
      rmSync(dir, { recursive: true, force: true });
    }
  }
  dirs.length = 0;
});

describe("loadConfig", () => {
  it("throws HarnessConfigError when config file is missing", () => {
    const dir = makeTempDir();
    dirs.push(dir);

    expect(() => loadConfig(dir)).toThrow(HarnessConfigError);
    expect(() => loadConfig(dir)).toThrow(/Config file not found/);
  });

  it("throws HarnessConfigError when JSON is invalid", () => {
    const dir = makeTempDir();
    dirs.push(dir);
    writeConfig(dir, "{ not valid json");

    expect(() => loadConfig(dir)).toThrow(HarnessConfigError);
    expect(() => loadConfig(dir)).toThrow(/Invalid JSON/);
  });

  it("throws HarnessConfigError when required field gates is missing", () => {
    const dir = makeTempDir();
    dirs.push(dir);
    writeConfig(dir, { runner_path: "./scripts/run.sh" }); // no 'gates'

    expect(() => loadConfig(dir)).toThrow(HarnessConfigError);
  });

  it("throws HarnessConfigError when gates array is empty", () => {
    const dir = makeTempDir();
    dirs.push(dir);
    writeConfig(dir, { runner_path: "./scripts/run.sh", gates: [] });

    expect(() => loadConfig(dir)).toThrow(HarnessConfigError);
  });

  it("returns config with defaults applied for minimal valid config", () => {
    const dir = makeTempDir();
    dirs.push(dir);
    writeConfig(dir, {
      runner_path: "./scripts/harness-check.sh",
      gates: ["pre-work", "in-progress"],
    });

    const result = loadConfig(dir);

    expect(result.config.runner_path).toBe("./scripts/harness-check.sh");
    expect(result.config.gates).toEqual(["pre-work", "in-progress"]);
    expect(result.config.fail_policy).toBe("hybrid");
    expect(result.config.max_total_iterations).toBe(100);
    expect(result.config.max_iterations_per_gate).toBe(10);
    expect(result.config.completion_promise).toBe("HARNESS-COMPLETE");
    expect(result.overrideConsumed).toBe(false);
  });

  it("CLI maxIter override replaces max_total_iterations", () => {
    const dir = makeTempDir();
    dirs.push(dir);
    writeConfig(dir, {
      runner_path: "./scripts/run.sh",
      gates: ["gate-a"],
    });

    const result = loadConfig(dir, { maxIter: 42 });

    expect(result.config.max_total_iterations).toBe(42);
  });

  it("CLI skipGate removes the gate from the gates array", () => {
    const dir = makeTempDir();
    dirs.push(dir);
    writeConfig(dir, {
      runner_path: "./scripts/run.sh",
      gates: ["pre-work", "in-progress", "pre-merge"],
    });

    const result = loadConfig(dir, { skipGate: ["in-progress"] });

    expect(result.config.gates).toEqual(["pre-work", "pre-merge"]);
  });

  it("CLI skipGate removing all gates throws HarnessConfigError", () => {
    const dir = makeTempDir();
    dirs.push(dir);
    writeConfig(dir, {
      runner_path: "./scripts/run.sh",
      gates: ["only-gate"],
    });

    expect(() => loadConfig(dir, { skipGate: ["only-gate"] })).toThrow(
      HarnessConfigError
    );
  });
});
