import { createLoopStateController } from "../loop-state-controller.js";
import { readState, getStatePath } from "../storage.js";

export interface HarnessOffContext {
  projectRoot: string;
  showToast(message: string, variant: "info" | "warning" | "error"): void;
  cancelBackgroundTask?(taskId: string): Promise<void>;
}

export async function handleHarnessOff(ctx: HarnessOffContext): Promise<void> {
  const statePath = getStatePath(ctx.projectRoot);
  const state = readState(statePath);

  if (!state || !state.loop.active) {
    ctx.showToast("ℹ️ No active harness loop to cancel", "info");
    return;
  }

  const watcherTaskId = state.loop.watcher_task_id;

  if (watcherTaskId && ctx.cancelBackgroundTask) {
    try {
      await ctx.cancelBackgroundTask(watcherTaskId);
      ctx.showToast("🛑 Cancelled active watcher subagent", "info");
    } catch (e) {
      ctx.showToast(
        `⚠️ Failed to cancel watcher: ${e instanceof Error ? e.message : String(e)}`,
        "warning"
      );
    }
  }

  const controller = createLoopStateController(ctx.projectRoot);
  controller.cancelLoop();

  ctx.showToast(
    `🛑 Harness loop cancelled at gate "${state.loop.current_gate}" (iteration ${state.loop.gate_iteration})`,
    "info"
  );
}
