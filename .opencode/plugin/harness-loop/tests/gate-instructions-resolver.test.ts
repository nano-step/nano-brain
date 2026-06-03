import { describe, it, expect, afterEach } from "vitest";
import { tmpdir } from "node:os";
import { randomUUID } from "node:crypto";
import { join } from "node:path";
import { mkdirSync, writeFileSync, rmSync } from "node:fs";
import {
  resolveGateInstructions,
  validateAllGateInstructions,
} from "../gate-instructions-resolver.js";
import { CONVENTION_GATE_DOC_PATH } from "../constants.js";
import type { HarnessConfig } from "../types.js";

const tempDirs: string[] = [];

function makeTempDir(): string {
  const dir = join(tmpdir(), `resolver-test-${randomUUID()}`);
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

describe("resolveGateInstructions — explicit doc path", () => {
  it("returns the relative doc path and docPath is non-null when the file exists", () => {
    const projectRoot = makeTempDir();
    const docRelPath = "path/to/doc.md";
    const docAbsPath = join(projectRoot, docRelPath);
    mkdirSync(join(projectRoot, "path/to"), { recursive: true });
    writeFileSync(docAbsPath, "# Gate doc");

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

    const result = resolveGateInstructions(config, "pre-merge", projectRoot);

    expect(result.docPath).toBe(docRelPath);
    expect(result.warning).toBeNull();
  });

  it("docPath is null and warning is set when the explicit file does not exist", () => {
    const projectRoot = makeTempDir();

    const config = makeConfig({
      gate_instructions: {
        "pre-merge": {
          doc: "missing/doc.md",
          skills: [],
          async: false,
          async_max_wait_seconds: 1800,
          async_poll_interval_seconds: 60,
          async_subagent_type: "quick",
          force: false,
        },
      },
    });

    const result = resolveGateInstructions(config, "pre-merge", projectRoot);

    expect(result.docPath).toBeNull();
    expect(result.warning).not.toBeNull();
    expect(result.warning).toContain("missing/doc.md");
  });
});

describe("resolveGateInstructions — convention fallback", () => {
  it("returns convention path when no explicit doc is configured but convention file exists", () => {
    const projectRoot = makeTempDir();
    const conventionDir = join(projectRoot, CONVENTION_GATE_DOC_PATH);
    mkdirSync(conventionDir, { recursive: true });
    writeFileSync(join(conventionDir, "pre-merge.md"), "# Pre-merge");

    const config = makeConfig();

    const result = resolveGateInstructions(config, "pre-merge", projectRoot);

    const expectedPath = join(CONVENTION_GATE_DOC_PATH, "pre-merge.md");
    expect(result.docPath).toBe(expectedPath);
    expect(result.warning).toBeNull();
  });

  it("docPath is null when neither explicit nor convention file exists", () => {
    const projectRoot = makeTempDir();
    const config = makeConfig();

    const result = resolveGateInstructions(config, "pre-merge", projectRoot);

    expect(result.docPath).toBeNull();
  });
});

describe("resolveGateInstructions — missing files produce warning, not throw", () => {
  it("does not throw when neither explicit nor convention file exists", () => {
    const projectRoot = makeTempDir();
    const config = makeConfig();

    expect(() =>
      resolveGateInstructions(config, "pre-merge", projectRoot)
    ).not.toThrow();
  });

  it("returns a non-null warning when no doc is found", () => {
    const projectRoot = makeTempDir();
    const config = makeConfig();

    const result = resolveGateInstructions(config, "pre-merge", projectRoot);

    expect(result.warning).not.toBeNull();
  });

  it("warning mentions the gate name", () => {
    const projectRoot = makeTempDir();
    const config = makeConfig();

    const result = resolveGateInstructions(config, "pre-merge", projectRoot);

    expect(result.warning).toContain("pre-merge");
  });
});

describe("validateAllGateInstructions — strict mode", () => {
  it("returns valid:false when strict=true and docs are missing", () => {
    const projectRoot = makeTempDir();
    const config = makeConfig({
      gates: ["pre-merge"],
      strict_instructions: true,
    });

    const result = validateAllGateInstructions(config, projectRoot, true);

    expect(result.valid).toBe(false);
    expect(result.warnings.length).toBeGreaterThan(0);
  });

  it("returns valid:true when strict=true and all gates have docs", () => {
    const projectRoot = makeTempDir();
    const conventionDir = join(projectRoot, CONVENTION_GATE_DOC_PATH);
    mkdirSync(conventionDir, { recursive: true });
    writeFileSync(join(conventionDir, "pre-merge.md"), "# Gate doc");

    const config = makeConfig({
      gates: ["pre-merge"],
      strict_instructions: true,
    });

    const result = validateAllGateInstructions(config, projectRoot, true);

    expect(result.valid).toBe(true);
    expect(result.warnings).toHaveLength(0);
  });

  it("returns valid:true when strict=false even if docs are missing", () => {
    const projectRoot = makeTempDir();
    const config = makeConfig({
      gates: ["pre-merge"],
      strict_instructions: false,
    });

    const result = validateAllGateInstructions(config, projectRoot, false);

    expect(result.valid).toBe(true);
  });
});

describe("resolveGateInstructions — skills", () => {
  it("returns configured skills array when gate has skills", () => {
    const projectRoot = makeTempDir();

    const config = makeConfig({
      gate_instructions: {
        "pre-merge": {
          doc: undefined,
          skills: ["web-testing", "review-work"],
          async: false,
          async_max_wait_seconds: 1800,
          async_poll_interval_seconds: 60,
          async_subagent_type: "quick",
          force: false,
        },
      },
    });

    const result = resolveGateInstructions(config, "pre-merge", projectRoot);

    expect(result.skills).toEqual(["web-testing", "review-work"]);
  });

  it("returns empty array when gate has no skills configured", () => {
    const projectRoot = makeTempDir();
    const config = makeConfig();

    const result = resolveGateInstructions(config, "pre-merge", projectRoot);

    expect(result.skills).toEqual([]);
  });

  it("returns empty array when gate_instructions entry has empty skills", () => {
    const projectRoot = makeTempDir();

    const config = makeConfig({
      gate_instructions: {
        "pre-merge": {
          doc: undefined,
          skills: [],
          async: false,
          async_max_wait_seconds: 1800,
          async_poll_interval_seconds: 60,
          async_subagent_type: "quick",
          force: false,
        },
      },
    });

    const result = resolveGateInstructions(config, "pre-merge", projectRoot);

    expect(result.skills).toEqual([]);
  });
});
