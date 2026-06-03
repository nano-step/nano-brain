import {
  readState,
  writeState,
  clearLoopBlock,
  createInitialState,
  getStatePath,
} from "./storage.js";
import {
  SAME_ERROR_HISTORY_WINDOW,
} from "./constants.js";
import type {
  HarnessConfig,
  HarnessLoopState,
  LoopMeta,
  RunnerOutput,
} from "./types.js";

export class LoopAlreadyActiveError extends Error {
  constructor(
    public readonly existingSessionId: string,
    public readonly existingGate: string
  ) {
    super(
      `Loop already active in session ${existingSessionId} at gate ${existingGate}`
    );
    this.name = "LoopAlreadyActiveError";
  }
}

export interface LoopStateController {
  getState(): HarnessLoopState | null;
  isActive(): boolean;
  startLoop(
    sessionId: string,
    config: HarnessConfig,
    featureId?: string,
    issueNumber?: number,
    story?: string,
    messageCountAtStart?: number
  ): HarnessLoopState;
  cancelLoop(): void;
  incrementGateIteration(): void;
  incrementTotalIteration(): void;
  transitionToGate(nextGate: string): void;
  recordRunnerOutput(output: RunnerOutput): void;
  incrementNoProgress(): void;
  resetNoProgress(): void;
  recordSameErrorHistory(gate: string, ruleIds: string[]): void;
  setVerificationPending(pending: boolean): void;
  setWatcherTaskId(taskId: string | null): void;
  clearWatcherTaskId(): void;
  setOverrideActive(active: boolean): void;
  rebindSession(newSessionId: string): void;
}

export function createLoopStateController(
  projectRoot: string,
  customStatePath?: string
): LoopStateController {
  const statePath = getStatePath(projectRoot, customStatePath);

  function getState(): HarnessLoopState | null {
    return readState(statePath);
  }

  function isActive(): boolean {
    const state = getState();
    return state?.loop.active === true;
  }

  function startLoop(
    sessionId: string,
    config: HarnessConfig,
    featureId?: string,
    issueNumber?: number,
    story?: string,
    messageCountAtStart = 0
  ): HarnessLoopState {
    let state = getState();

    if (state?.loop.active) {
      throw new LoopAlreadyActiveError(
        state.loop.session_id,
        state.loop.current_gate
      );
    }

    if (state === null) {
      state = createInitialState(
        featureId ?? null,
        issueNumber ?? null,
        story ?? null
      );
    } else {
      state.feature_id = featureId ?? state.feature_id;
      state.issue_number = issueNumber ?? state.issue_number;
      state.story = story ?? state.story;
    }

    const firstGate = config.gates[0];
    if (firstGate === undefined) {
      throw new Error("Config must have at least one gate");
    }

    const loopMeta: LoopMeta = {
      active: true,
      current_gate: firstGate,
      gate_iteration: 1,
      total_iteration: 1,
      max_iterations_per_gate: config.max_iterations_per_gate,
      max_total_iterations: config.max_total_iterations,
      started_at: new Date().toISOString(),
      session_id: sessionId,
      config_snapshot: config,
      last_runner_output: null,
      no_progress_count: 0,
      override_active: false,
      same_error_history: {},
      verification_pending: false,
      watcher_task_id: null,
      message_count_at_start: messageCountAtStart,
    };

    state.loop = loopMeta;
    writeState(statePath, state);

    return state;
  }

  function cancelLoop(): void {
    clearLoopBlock(statePath);
  }

  function updateLoop(updater: (loop: LoopMeta) => void): void {
    const state = getState();
    if (state === null || !state.loop.active) {
      return;
    }

    updater(state.loop);
    writeState(statePath, state);
  }

  function incrementGateIteration(): void {
    updateLoop((loop) => {
      loop.gate_iteration += 1;
    });
  }

  function incrementTotalIteration(): void {
    updateLoop((loop) => {
      loop.total_iteration += 1;
    });
  }

  function transitionToGate(nextGate: string): void {
    updateLoop((loop) => {
      const previousGate = loop.current_gate;

      loop.current_gate = nextGate;
      loop.gate_iteration = 1;
      loop.total_iteration += 1;

      if (
        previousGate &&
        loop.same_error_history[previousGate] !== undefined
      ) {
        delete loop.same_error_history[previousGate];
      }

      loop.verification_pending = false;
    });
  }

  function recordRunnerOutput(output: RunnerOutput): void {
    const state = getState();
    if (state === null || !state.loop.active) {
      return;
    }

    state.loop.last_runner_output = output;

    const gate = output.gate;
    if (!state.checkpoints[gate]) {
      state.checkpoints[gate] = {
        status: output.status,
        checked_at: new Date().toISOString(),
        checks: {},
      };
    } else {
      state.checkpoints[gate].status = output.status;
      state.checkpoints[gate].checked_at = new Date().toISOString();
    }

    writeState(statePath, state);
  }

  function incrementNoProgress(): void {
    updateLoop((loop) => {
      loop.no_progress_count += 1;
    });
  }

  function resetNoProgress(): void {
    updateLoop((loop) => {
      loop.no_progress_count = 0;
    });
  }

  function recordSameErrorHistory(gate: string, ruleIds: string[]): void {
    if (ruleIds.length === 0) {
      return;
    }

    updateLoop((loop) => {
      if (!loop.same_error_history[gate]) {
        loop.same_error_history[gate] = [];
      }

      loop.same_error_history[gate].push(ruleIds);

      if (loop.same_error_history[gate].length > SAME_ERROR_HISTORY_WINDOW) {
        loop.same_error_history[gate] = loop.same_error_history[gate].slice(
          -SAME_ERROR_HISTORY_WINDOW
        );
      }
    });
  }

  function setVerificationPending(pending: boolean): void {
    updateLoop((loop) => {
      loop.verification_pending = pending;
    });
  }

  function setWatcherTaskId(taskId: string | null): void {
    updateLoop((loop) => {
      loop.watcher_task_id = taskId;
    });
  }

  function clearWatcherTaskId(): void {
    updateLoop((loop) => {
      loop.watcher_task_id = null;
    });
  }

  function setOverrideActive(active: boolean): void {
    updateLoop((loop) => {
      loop.override_active = active;
    });
  }

  function rebindSession(newSessionId: string): void {
    updateLoop((loop) => {
      loop.session_id = newSessionId;
    });
  }

  return {
    getState,
    isActive,
    startLoop,
    cancelLoop,
    incrementGateIteration,
    incrementTotalIteration,
    transitionToGate,
    recordRunnerOutput,
    incrementNoProgress,
    resetNoProgress,
    recordSameErrorHistory,
    setVerificationPending,
    setWatcherTaskId,
    clearWatcherTaskId,
    setOverrideActive,
    rebindSession,
  };
}
