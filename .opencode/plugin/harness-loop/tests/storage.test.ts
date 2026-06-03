import { describe, it, expect, afterEach } from "vitest";
import { writeFileSync, mkdirSync, rmSync, existsSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { randomUUID } from "node:crypto";
import {
  readState,
  writeState,
  clearLoopBlock,
  createInitialState,
} from "../storage.js";
import { StateCorruptionError } from "../types.js";
import { DEFAULT_STATE_FILE_PATH } from "../constants.js";

function makeStateFile(): { dir: string; statePath: string } {
  const dir = join(tmpdir(), `harness-storage-test-${randomUUID()}`);
  mkdirSync(join(dir, ".opencode"), { recursive: true });
  const statePath = join(dir, DEFAULT_STATE_FILE_PATH);
  return { dir, statePath };
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

describe("readState", () => {
  it("returns null when state file does not exist", () => {
    const { dir, statePath } = makeStateFile();
    dirs.push(dir);

    expect(readState(statePath)).toBeNull();
  });

  it("throws StateCorruptionError when file contains invalid JSON", () => {
    const { dir, statePath } = makeStateFile();
    dirs.push(dir);
    writeFileSync(statePath, "{ not valid json }", "utf-8");

    expect(() => readState(statePath)).toThrow(StateCorruptionError);
  });

  it("throws StateCorruptionError when file fails schema validation", () => {
    const { dir, statePath } = makeStateFile();
    dirs.push(dir);
    writeFileSync(statePath, JSON.stringify({ random: "garbage" }), "utf-8");

    expect(() => readState(statePath)).toThrow(StateCorruptionError);
  });
});

describe("writeState + readState round-trip", () => {
  it("reads back the exact same state that was written", () => {
    const { dir, statePath } = makeStateFile();
    dirs.push(dir);

    const initial = createInitialState("feat-123", 42, "my-story");
    initial.loop.config_snapshot.gates = ["gate-a"];
    initial.loop.config_snapshot.runner_path = "./run.sh";
    writeState(statePath, initial);

    const loaded = readState(statePath);

    expect(loaded).not.toBeNull();
    expect(loaded!.feature_id).toBe("feat-123");
    expect(loaded!.issue_number).toBe(42);
    expect(loaded!.story).toBe("my-story");
    expect(loaded!.loop.active).toBe(false);
    expect(loaded!.checkpoints).toEqual({});
  });
});

describe("clearLoopBlock", () => {
  it("is a no-op when state file does not exist", () => {
    const { dir, statePath } = makeStateFile();
    dirs.push(dir);

    expect(() => clearLoopBlock(statePath)).not.toThrow();
    expect(existsSync(statePath)).toBe(false);
  });

  it("throws StateCorruptionError when clearing a state with gates (schema enforcement)", () => {
    const { dir, statePath } = makeStateFile();
    dirs.push(dir);

    const state = createInitialState(null, null, null);
    state.checkpoints["pre-work"] = {
      status: "PASS",
      checked_at: new Date().toISOString(),
    };
    state.loop.active = true;
    state.loop.current_gate = "pre-work";
    state.loop.session_id = "sess-abc";
    state.loop.config_snapshot.gates = ["pre-work"];
    state.loop.config_snapshot.runner_path = "./run.sh";
    writeState(statePath, state);

    expect(() => clearLoopBlock(statePath)).toThrow(StateCorruptionError);
  });
});

describe("createInitialState", () => {
  it("creates state with loop.active false and empty checkpoints", () => {
    const state = createInitialState("feat-1", 7, "story.md");

    expect(state.feature_id).toBe("feat-1");
    expect(state.issue_number).toBe(7);
    expect(state.story).toBe("story.md");
    expect(state.loop.active).toBe(false);
    expect(state.checkpoints).toEqual({});
  });

  it("accepts null values for all parameters", () => {
    const state = createInitialState(null, null, null);

    expect(state.feature_id).toBeNull();
    expect(state.issue_number).toBeNull();
    expect(state.story).toBeNull();
  });
});
