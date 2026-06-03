import {
  IDLE_SETTLE_MS,
  USER_MESSAGE_IN_PROGRESS_WINDOW_MS,
} from "./constants.js";
import {
  createLoopStateController,
  type LoopStateController,
} from "./loop-state-controller.js";
import { getStatePath, readState } from "./storage.js";
import { invokeRunner } from "./runner-invoker.js";
import { buildContinuationPrompt } from "./continuation-prompt-builder.js";
import {
  detectCompletion,
  detectOverrideToken,
} from "./completion-detector.js";
import { latestAssistantTurnMadeNoProgress } from "./no-progress-detector.js";
import { hasRepeatedSameError } from "./same-error-detector.js";
import { resolveGateInstructions } from "./gate-instructions-resolver.js";
import { buildCompletionPrompt } from "./templates/continuation-prompt.js";
import { buildUltraworkVerificationPrompt } from "./templates/ultrawork-verification.js";
import { collectWatcherResult } from "./async-watcher-result-handler.js";
import type { HarnessConfig, HarnessLoopState, RunnerOutput } from "./types.js";

export interface PluginContext {
  sessionId: string;
  projectRoot: string;
  getMessages(): Array<{ role: string; content: string }>;
  injectMessage(text: string): Promise<void>;
  showToast(message: string, variant: "info" | "warning" | "error"): void;
  hasActiveBackgroundTasks(): boolean;
  latestUserMessageTimestamp(): number;
  spawnWatcher?(
    gate: string,
    config: unknown,
    state: HarnessLoopState
  ): Promise<string>;
  cancelBackgroundTask?(taskId: string): Promise<void>;
  collectBackgroundTaskResult?(taskId: string): Promise<string | null>;
}

const inFlightSessions = new Set<string>();
const runtimeRetried = new Map<string, number>();

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function isCacheFresh(
  state: HarnessLoopState,
  gate: string,
  cacheTtlMinutes: number
): boolean {
  const checkpoint = state.checkpoints[gate];
  if (!checkpoint) return false;
  if (checkpoint.status !== "PASS") return false;

  const checkedAt = new Date(checkpoint.checked_at).getTime();
  const now = Date.now();
  const ttlMs = cacheTtlMinutes * 60 * 1000;

  return now - checkedAt < ttlMs;
}

function getNextGate(
  config: { gates: string[] },
  currentGate: string,
  runnerOutput: RunnerOutput | null
): string | null {
  if (runnerOutput?.next_gate) {
    return runnerOutput.next_gate;
  }

  const currentIndex = config.gates.indexOf(currentGate);
  if (currentIndex === -1 || currentIndex >= config.gates.length - 1) {
    return null;
  }

  return config.gates[currentIndex + 1] ?? null;
}

export async function handleSessionIdle(ctx: PluginContext): Promise<void> {
  const statePath = getStatePath(ctx.projectRoot);
  let state = readState(statePath);

  if (!state || !state.loop.active) {
    return;
  }

  if (state.loop.session_id !== ctx.sessionId) {
    return;
  }

  if (inFlightSessions.has(ctx.sessionId)) {
    return;
  }

  const now = Date.now();
  const lastUserMessage = ctx.latestUserMessageTimestamp();
  if (now - lastUserMessage < USER_MESSAGE_IN_PROGRESS_WINDOW_MS) {
    return;
  }

  if (ctx.hasActiveBackgroundTasks()) {
    return;
  }

  await sleep(IDLE_SETTLE_MS);

  state = readState(statePath);
  if (!state || !state.loop.active) {
    return;
  }
  if (state.loop.session_id !== ctx.sessionId) {
    return;
  }

  inFlightSessions.add(ctx.sessionId);

  try {
    await processLoopIteration(ctx, state);
  } finally {
    inFlightSessions.delete(ctx.sessionId);
  }
}

async function processLoopIteration(
  ctx: PluginContext,
  state: HarnessLoopState
): Promise<void> {
  const controller = createLoopStateController(ctx.projectRoot);
  const config = state.loop.config_snapshot;
  const currentGate = state.loop.current_gate;
  const messages = ctx.getMessages();

  const completionSource = detectCompletion(
    messages,
    state,
    state.loop.last_runner_output,
    config
  );

  if (completionSource) {
    controller.cancelLoop();
    ctx.showToast(
      `🎉 Harness loop complete! (${completionSource})`,
      "info"
    );
    await ctx.injectMessage(buildCompletionPrompt(completionSource));
    return;
  }

  const override = detectOverrideToken(messages);
  if (override.found) {
    controller.setOverrideActive(true);
    controller.cancelLoop();
    ctx.showToast(
      `⏸️ Loop paused by override: ${override.reason ?? "user requested"}`,
      "warning"
    );
    return;
  }

  if (latestAssistantTurnMadeNoProgress(messages)) {
    controller.incrementNoProgress();
    state = controller.getState()!;

    if (state.loop.no_progress_count >= 3) {
      controller.cancelLoop();
      ctx.showToast(
        "❌ Loop stopped: No progress detected for 3 consecutive turns",
        "error"
      );
      return;
    }
  } else {
    controller.resetNoProgress();
  }

  if (state.loop.total_iteration >= state.loop.max_total_iterations) {
    controller.cancelLoop();
    ctx.showToast(
      `❌ Loop stopped: Max total iterations (${state.loop.max_total_iterations}) reached`,
      "error"
    );
    return;
  }

  if (state.loop.gate_iteration >= state.loop.max_iterations_per_gate) {
    const failPolicy = config.fail_policy;

    if (failPolicy === "ask" || failPolicy === "hybrid") {
      controller.cancelLoop();
      ctx.showToast(
        `⏸️ Loop paused: Gate "${currentGate}" exceeded max iterations (${state.loop.max_iterations_per_gate})`,
        "warning"
      );
      return;
    }
  }

  if (hasRepeatedSameError(state, currentGate)) {
    controller.cancelLoop();
    ctx.showToast(
      `❌ Loop stopped: Same error repeated 3 times on gate "${currentGate}"`,
      "error"
    );
    return;
  }

  if (state.loop.watcher_task_id != null) {
    const watcherResult = await collectWatcherResult(
      ctx,
      state,
      state.loop.watcher_task_id
    );

    if (watcherResult === null) {
      return;
    }

    controller.clearWatcherTaskId();
    controller.recordRunnerOutput(watcherResult);

    if (watcherResult.rule_ids_violated.length > 0) {
      controller.recordSameErrorHistory(
        currentGate,
        watcherResult.rule_ids_violated
      );
    }

    await handleRunnerOutput(ctx, controller, state, config, currentGate, watcherResult);
    return;
  }

  const gateInstructions = resolveGateInstructions(
    config,
    currentGate,
    ctx.projectRoot
  );

  if (gateInstructions.isAsync && gateInstructions.asyncConfig) {
    if (ctx.spawnWatcher) {
      const taskId = await ctx.spawnWatcher(currentGate, config, state);
      controller.setWatcherTaskId(taskId);
      controller.incrementGateIteration();
      ctx.showToast(
        `🕐 Watching gate "${currentGate}" via background subagent`,
        "info"
      );

      const maxWaitSeconds = gateInstructions.asyncConfig.maxWaitSeconds;
      const heartbeatsEnabled = config.async_heartbeats;

      if (heartbeatsEnabled) {
        const intervalSeconds = Math.max(
          60,
          Math.floor(maxWaitSeconds / 3)
        );
        let heartbeatCount = 0;
        const maxHeartbeats = 3;

        const heartbeatInterval = setInterval(() => {
          if (heartbeatCount >= maxHeartbeats) {
            clearInterval(heartbeatInterval);
            return;
          }
          heartbeatCount += 1;
          const elapsed = heartbeatCount * intervalSeconds;
          ctx.showToast(
            `⏳ Still waiting for gate "${currentGate}" (watcher active ~${elapsed}s)`,
            "info"
          );
        }, intervalSeconds * 1000);

        const graceTimeout = setTimeout(async () => {
          clearInterval(heartbeatInterval);

          if (ctx.cancelBackgroundTask) {
            await ctx.cancelBackgroundTask(taskId);
          }

          const refreshedState = controller.getState();
          if (
            refreshedState == null ||
            !refreshedState.loop.active ||
            refreshedState.loop.watcher_task_id !== taskId
          ) {
            return;
          }

          const timeoutOutput: RunnerOutput = {
            gate: currentGate,
            status: "FAIL",
            checks: [],
            rule_ids_violated: ["watcher-timeout"],
            instructions_for_agent: `Async watcher timed out after ${maxWaitSeconds}s`,
          };

          controller.clearWatcherTaskId();
          controller.recordRunnerOutput(timeoutOutput);

          const prompt = buildContinuationPrompt(
            refreshedState,
            timeoutOutput,
            config,
            ctx.projectRoot
          );
          await ctx.injectMessage(prompt);
          ctx.showToast(
            `⏰ Gate "${currentGate}" watcher timed out after ${maxWaitSeconds}s`,
            "warning"
          );
        }, (maxWaitSeconds + 30) * 1000);

        graceTimeout.unref?.();
      }

      return;
    }
  }

  if (
    isCacheFresh(
      state,
      currentGate,
      config.cache_ttl_minutes
    )
  ) {
    const nextGate = getNextGate(config, currentGate, null);

    if (nextGate) {
      controller.transitionToGate(nextGate);
      ctx.showToast(`✓ Gate "${currentGate}" cached PASS → ${nextGate}`, "info");
    } else {
      controller.cancelLoop();
      ctx.showToast("🎉 Harness loop complete! (all gates cached PASS)", "info");
      await ctx.injectMessage(buildCompletionPrompt("structural"));
    }
    return;
  }

  const runnerOutput = await invokeRunner(
    config,
    currentGate,
    ctx.projectRoot,
    state.feature_id != null ? { featureId: state.feature_id } : {}
  );

  controller.recordRunnerOutput(runnerOutput);

  if (runnerOutput.rule_ids_violated.length > 0) {
    controller.recordSameErrorHistory(currentGate, runnerOutput.rule_ids_violated);
  }

  await handleRunnerOutput(ctx, controller, state, config, currentGate, runnerOutput);
}

async function handleRunnerOutput(
  ctx: PluginContext,
  controller: LoopStateController,
  state: HarnessLoopState,
  config: HarnessConfig,
  currentGate: string,
  runnerOutput: RunnerOutput
): Promise<void> {
  switch (runnerOutput.status) {
    case "PASS": {
      if (config.ultrawork_verify_gates.includes(currentGate)) {
        controller.setVerificationPending(true);
        await ctx.injectMessage(buildUltraworkVerificationPrompt(currentGate));
        ctx.showToast(
          `🔍 Gate "${currentGate}" PASS — Oracle verification requested`,
          "info"
        );
        return;
      }

      const nextGate = getNextGate(config, currentGate, runnerOutput);

      if (nextGate) {
        controller.transitionToGate(nextGate);
        ctx.showToast(`✓ Gate "${currentGate}" PASS → ${nextGate}`, "info");
      } else {
        controller.cancelLoop();
        ctx.showToast("🎉 Harness loop complete!", "info");
        await ctx.injectMessage(buildCompletionPrompt("structural"));
      }
      break;
    }

    case "FAIL":
    case "ERROR": {
      controller.incrementGateIteration();
      const failPrompt = buildContinuationPrompt(
        state,
        runnerOutput,
        config,
        ctx.projectRoot
      );
      await ctx.injectMessage(failPrompt);
      ctx.showToast(
        `❌ Gate "${currentGate}" ${runnerOutput.status} — fix instructions injected`,
        "warning"
      );
      break;
    }

    case "BLOCKED": {
      controller.cancelLoop();
      const blockedPrompt = buildContinuationPrompt(
        state,
        runnerOutput,
        config,
        ctx.projectRoot
      );
      await ctx.injectMessage(blockedPrompt);
      ctx.showToast(
        `⏸️ Gate "${currentGate}" BLOCKED — human input needed`,
        "warning"
      );
      break;
    }

    case "WAITING": {
      const waitSeconds = runnerOutput.wait_seconds ?? 30;
      ctx.showToast(
        `⏳ Gate "${currentGate}" WAITING — retry in ${waitSeconds}s`,
        "info"
      );
      await sleep(waitSeconds * 1000);
      break;
    }

    case "SKIP": {
      const nextGate = getNextGate(config, currentGate, runnerOutput);

      if (nextGate) {
        controller.transitionToGate(nextGate);
        ctx.showToast(`⏭️ Gate "${currentGate}" SKIP → ${nextGate}`, "info");
      } else {
        controller.cancelLoop();
        ctx.showToast("🎉 Harness loop complete!", "info");
        await ctx.injectMessage(buildCompletionPrompt("structural"));
      }
      break;
    }
  }
}

export async function handleSessionError(
  ctx: PluginContext,
  error: Error
): Promise<void> {
  const statePath = getStatePath(ctx.projectRoot);
  const state = readState(statePath);

  if (!state || !state.loop.active) {
    return;
  }

  if (state.loop.session_id !== ctx.sessionId) {
    return;
  }

  const retryCount = runtimeRetried.get(ctx.sessionId) ?? 0;

  if (retryCount >= 3) {
    const controller = createLoopStateController(ctx.projectRoot);
    controller.cancelLoop();
    ctx.showToast(`❌ Loop stopped: Too many errors (${error.message})`, "error");
    runtimeRetried.delete(ctx.sessionId);
    return;
  }

  runtimeRetried.set(ctx.sessionId, retryCount + 1);
  ctx.showToast(
    `⚠️ Session error (retry ${retryCount + 1}/3): ${error.message}`,
    "warning"
  );
}

export async function handleSessionDeleted(
  ctx: PluginContext,
  deletedSessionId: string
): Promise<void> {
  const statePath = getStatePath(ctx.projectRoot);
  const state = readState(statePath);

  if (!state || !state.loop.active) {
    return;
  }

  if (state.loop.session_id === deletedSessionId) {
    const controller = createLoopStateController(ctx.projectRoot);
    controller.cancelLoop();
  }
}
