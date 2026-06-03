import {
  readFileSync,
  writeFileSync,
  renameSync,
  existsSync,
  mkdirSync,
} from "node:fs";
import { dirname, join } from "node:path";
import { openSync, closeSync, fsyncSync } from "node:fs";
import {
  HarnessLoopStateSchema,
  StateCorruptionError,
  type HarnessLoopState,
  type LoopMeta,
} from "./types.js";
import { DEFAULT_STATE_FILE_PATH } from "./constants.js";

function ensureDir(filePath: string): void {
  const dir = dirname(filePath);
  if (!existsSync(dir)) {
    mkdirSync(dir, { recursive: true });
  }
}

function atomicWriteFile(filePath: string, content: string): void {
  ensureDir(filePath);

  const tmpPath = `${filePath}.tmp.${process.pid}`;

  const fd = openSync(tmpPath, "w", 0o644);
  try {
    writeFileSync(fd, content, "utf-8");
    fsyncSync(fd);
  } finally {
    closeSync(fd);
  }

  renameSync(tmpPath, filePath);
}

export function getStatePath(projectRoot: string, customPath?: string): string {
  return join(projectRoot, customPath ?? DEFAULT_STATE_FILE_PATH);
}

export function stateExists(statePath: string): boolean {
  return existsSync(statePath);
}

export function readState(statePath: string): HarnessLoopState | null {
  if (!existsSync(statePath)) {
    return null;
  }

  let content: string;
  try {
    content = readFileSync(statePath, "utf-8");
  } catch {
    return null;
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(content);
  } catch {
    throw new StateCorruptionError(
      `State file contains invalid JSON: ${statePath}`,
      statePath
    );
  }

  const result = HarnessLoopStateSchema.safeParse(parsed);
  if (!result.success) {
    throw new StateCorruptionError(
      `State file schema validation failed: ${result.error.message}`,
      statePath
    );
  }

  // Zod validates the shape; cast needed because exactOptionalPropertyTypes
  // creates a mismatch between optional Zod fields and the interface.
  return result.data as unknown as HarnessLoopState;
}

export function writeState(statePath: string, state: HarnessLoopState): void {
  const result = HarnessLoopStateSchema.safeParse(state);
  if (!result.success) {
    throw new StateCorruptionError(
      `Attempted to write invalid state: ${result.error.message}`,
      statePath
    );
  }

  state.updated_at = new Date().toISOString();

  const content = JSON.stringify(state, null, 2);
  atomicWriteFile(statePath, content);
}

export function clearLoopBlock(statePath: string): void {
  const state = readState(statePath);
  if (state === null) {
    return;
  }

  const clearedLoop: LoopMeta = {
    active: false,
    current_gate: "",
    gate_iteration: 0,
    total_iteration: 0,
    max_iterations_per_gate: 0,
    max_total_iterations: 0,
    started_at: "",
    session_id: "",
    config_snapshot: {
      runner_path: "",
      gates: [],
      fail_policy: "hybrid",
      rule_id_format: "{id}",
      max_total_iterations: 100,
      max_iterations_per_gate: 10,
      auto_fix_attempts: 3,
      cache_ttl_minutes: 30,
      runner_timeout_seconds: 300,
      completion_promise: "HARNESS-COMPLETE",
      ultrawork_verify_gates: [],
      state_file_path: DEFAULT_STATE_FILE_PATH,
      gate_instructions: {},
      phase_hooks: {},
      strict_instructions: false,
      async_heartbeats: true,
    },
    last_runner_output: null,
    no_progress_count: 0,
    override_active: false,
    same_error_history: {},
    verification_pending: false,
    watcher_task_id: null,
    message_count_at_start: 0,
  };

  const newState: HarnessLoopState = {
    ...state,
    loop: clearedLoop,
    updated_at: new Date().toISOString(),
  };

  writeState(statePath, newState);
}

export function createInitialState(
  featureId: string | null,
  issueNumber: number | null,
  story: string | null
): HarnessLoopState {
  return {
    feature_id: featureId,
    issue_number: issueNumber,
    story,
    updated_at: new Date().toISOString(),
    checkpoints: {},
    loop: {
      active: false,
      current_gate: "",
      gate_iteration: 0,
      total_iteration: 0,
      max_iterations_per_gate: 0,
      max_total_iterations: 0,
      started_at: "",
      session_id: "",
      config_snapshot: {
        runner_path: "",
        gates: [],
        fail_policy: "hybrid",
        rule_id_format: "{id}",
        max_total_iterations: 100,
        max_iterations_per_gate: 10,
        auto_fix_attempts: 3,
        cache_ttl_minutes: 30,
        runner_timeout_seconds: 300,
        completion_promise: "HARNESS-COMPLETE",
        ultrawork_verify_gates: [],
        state_file_path: DEFAULT_STATE_FILE_PATH,
        gate_instructions: {},
        phase_hooks: {},
        strict_instructions: false,
        async_heartbeats: true,
      },
      last_runner_output: null,
      no_progress_count: 0,
      override_active: false,
      same_error_history: {},
      verification_pending: false,
      watcher_task_id: null,
      message_count_at_start: 0,
    },
  };
}
