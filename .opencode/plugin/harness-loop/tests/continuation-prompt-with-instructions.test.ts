import { describe, it, expect, afterEach } from "vitest";
import { tmpdir } from "node:os";
import { randomUUID } from "node:crypto";
import { join } from "node:path";
import { mkdirSync, writeFileSync, rmSync } from "node:fs";
import { buildContinuationPrompt } from "../continuation-prompt-builder.js";
import { CONVENTION_GATE_DOC_PATH } from "../constants.js";
import type { HarnessConfig, HarnessLoopState, RunnerOutput } from "../types.js";

const tempDirs: string[] = [];

function makeTempDir(): string {
  const dir = join(tmpdir(), `cpb-instructions-test-${randomUUID()}`);
  mkdirSync(dir, { recursive: true });
  tempDirs.push(dir);
  return dir;
}

afterEach(() => {
  for (const dir of tempDirs.splice(0)) {
    rmSync(dir, { recursive: true, force: true });
  }
});

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

function makeState(
  configOverride?: Partial<HarnessConfig>,
  loopOverrides: Partial<HarnessLoopState["loop"]> = {}
): HarnessLoopState {
  const cfg = makeConfig(configOverride);
  const loop: HarnessLoopState["loop"] = {
    active: true,
    current_gate: "pre-merge",
    gate_iteration: 1,
    total_iteration: 1,
    max_iterations_per_gate: 10,
    max_total_iterations: 100,
    started_at: new Date().toISOString(),
    session_id: "sess-001",
    config_snapshot: cfg,
    last_runner_output: null,
    no_progress_count: 0,
    override_active: false,
    same_error_history: {},
    verification_pending: false,
    watcher_task_id: null,
    message_count_at_start: 0,
    ...loopOverrides,
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
    gate: "pre-merge",
    status: "FAIL",
    checks: [],
    rule_ids_violated: [],
    ...overrides,
  };
}

describe("buildContinuationPrompt — doc found, skills present", () => {
  it("prompt contains the doc path and skill names when both are configured", () => {
    const projectRoot = makeTempDir();
    const docRelPath = "docs/harness/gates/pre-merge.md";
    mkdirSync(join(projectRoot, "docs/harness/gates"), { recursive: true });
    writeFileSync(join(projectRoot, docRelPath), "# Pre-merge gate");

    const config = makeConfig({
      gate_instructions: {
        "pre-merge": {
          doc: docRelPath,
          skills: ["review-work", "web-testing"],
          async: false,
          async_max_wait_seconds: 1800,
          async_poll_interval_seconds: 60,
          async_subagent_type: "quick",
          force: false,
        },
      },
    });
    const state = makeState(config);
    const output = makeRunnerOutput({ gate: "pre-merge" });

    const prompt = buildContinuationPrompt(state, output, config, projectRoot);

    expect(prompt).toContain(docRelPath);
    expect(prompt).toContain("review-work");
    expect(prompt).toContain("web-testing");
  });
});

describe("buildContinuationPrompt — doc found, no skills", () => {
  it("prompt contains doc path but no skills section when skills list is empty", () => {
    const projectRoot = makeTempDir();
    const docRelPath = "docs/harness/gates/pre-merge.md";
    mkdirSync(join(projectRoot, "docs/harness/gates"), { recursive: true });
    writeFileSync(join(projectRoot, docRelPath), "# Pre-merge gate");

    const config = makeConfig({
      gate_instructions: {
        "pre-merge": {
          doc: docRelPath,
          skills: [],
          async: false,
          async_max_wait_seconds: 1800,
          async_poll_interval_seconds: 60,
          async_subagent_type: "quick",
          force: false,
        },
      },
    });
    const state = makeState(config);
    const output = makeRunnerOutput({ gate: "pre-merge" });

    const prompt = buildContinuationPrompt(state, output, config, projectRoot);

    expect(prompt).toContain(docRelPath);
    expect(prompt).not.toContain("Load skills");
  });
});

describe("buildContinuationPrompt — no doc, non-strict mode", () => {
  it("does not throw when no doc file exists and strict_instructions is false", () => {
    const projectRoot = makeTempDir();
    const config = makeConfig({ strict_instructions: false });
    const state = makeState(config);
    const output = makeRunnerOutput({ gate: "pre-merge" });

    expect(() =>
      buildContinuationPrompt(state, output, config, projectRoot)
    ).not.toThrow();
  });

  it("prompt contains a warning message instead of a doc path reference", () => {
    const projectRoot = makeTempDir();
    const config = makeConfig({ strict_instructions: false });
    const state = makeState(config);
    const output = makeRunnerOutput({ gate: "pre-merge" });

    const prompt = buildContinuationPrompt(state, output, config, projectRoot);

    expect(prompt).not.toContain("Read project's gate protocol FIRST");
    expect(prompt).toContain("pre-merge");
  });

  it("prompt uses the convention path as a warning reference when no doc is found", () => {
    const projectRoot = makeTempDir();
    const config = makeConfig({ strict_instructions: false });
    const state = makeState(config);
    const output = makeRunnerOutput({ gate: "pre-merge" });

    const prompt = buildContinuationPrompt(state, output, config, projectRoot);

    expect(prompt).toContain("pre-merge");
  });
});

describe("buildContinuationPrompt — convention fallback path appears in prompt", () => {
  it("prompt shows convention doc path when it exists but no explicit config", () => {
    const projectRoot = makeTempDir();
    const conventionDir = join(projectRoot, CONVENTION_GATE_DOC_PATH);
    mkdirSync(conventionDir, { recursive: true });
    writeFileSync(join(conventionDir, "pre-merge.md"), "# Gate instructions");

    const config = makeConfig();
    const state = makeState(config);
    const output = makeRunnerOutput({ gate: "pre-merge" });

    const prompt = buildContinuationPrompt(state, output, config, projectRoot);

    const expectedPath = join(CONVENTION_GATE_DOC_PATH, "pre-merge.md");
    expect(prompt).toContain(expectedPath);
  });
});
