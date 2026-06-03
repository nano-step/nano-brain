import { parseWatcherResult } from "./async-watcher-spawner.js";
import type { PluginContext } from "./harness-loop-event-handler.js";
import type { HarnessLoopState, RunnerOutput } from "./types.js";

export async function collectWatcherResult(
  ctx: PluginContext,
  state: HarnessLoopState,
  watcherTaskId: string
): Promise<RunnerOutput | null> {
  if (!ctx.collectBackgroundTaskResult) {
    return null;
  }

  const result = await ctx.collectBackgroundTaskResult(watcherTaskId);

  if (result == null) {
    return null;
  }

  return parseWatcherResult(result, state.loop.current_gate);
}
