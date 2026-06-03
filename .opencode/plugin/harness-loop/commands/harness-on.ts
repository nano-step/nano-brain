import { loadConfig } from "../config-loader.js";
import {
  createLoopStateController,
  LoopAlreadyActiveError,
} from "../loop-state-controller.js";
import { validateAllGateInstructions } from "../gate-instructions-resolver.js";
import { buildOpeningPrompt } from "../templates/opening-prompt.js";
import type { ConfigOverrides } from "../types.js";
import { existsSync, accessSync, constants } from "node:fs";
import { join } from "node:path";

export interface HarnessOnContext {
  projectRoot: string;
  sessionId: string;
  getMessageCount(): number;
  injectMessage(text: string): Promise<void>;
  showToast(message: string, variant: "info" | "warning" | "error"): void;
  askQuestion(options: {
    question: string;
    choices: string[];
  }): Promise<string>;
}

export interface HarnessOnOptions {
  force?: boolean;
  maxIter?: number;
  skipGate?: string[];
  configPath?: string;
  featureId?: string;
  issueNumber?: number;
  story?: string;
}

function parseCliArgs(args: string[]): HarnessOnOptions {
  const options: HarnessOnOptions = {};

  for (const arg of args) {
    if (arg === "--force") {
      options.force = true;
    } else if (arg.startsWith("--max-iter=")) {
      options.maxIter = parseInt(arg.slice(11), 10);
    } else if (arg.startsWith("--skip-gate=")) {
      const gates = arg.slice(12).split(",");
      options.skipGate = (options.skipGate ?? []).concat(gates);
    } else if (arg.startsWith("--config=")) {
      options.configPath = arg.slice(9);
    } else if (arg.startsWith("--feature=")) {
      options.featureId = arg.slice(10);
    } else if (arg.startsWith("--issue=")) {
      options.issueNumber = parseInt(arg.slice(8), 10);
    } else if (arg.startsWith("--story=")) {
      options.story = arg.slice(8);
    }
  }

  return options;
}

export async function handleHarnessOn(
  ctx: HarnessOnContext,
  args: string[] = []
): Promise<void> {
  const options = parseCliArgs(args);

  const cliOverrides: ConfigOverrides = {
    ...(options.force !== undefined && { force: options.force }),
    ...(options.maxIter !== undefined && { maxIter: options.maxIter }),
    ...(options.skipGate !== undefined && { skipGate: options.skipGate }),
    ...(options.configPath !== undefined && { configPath: options.configPath }),
  };

  let configResult;
  try {
    configResult = loadConfig(ctx.projectRoot, cliOverrides);
  } catch (e) {
    const message = e instanceof Error ? e.message : String(e);
    ctx.showToast(`❌ ${message}`, "error");
    return;
  }

  const { config, overrideConsumed } = configResult;

  if (overrideConsumed) {
    ctx.showToast(
      "ℹ️ Applied and removed .opencode/harness.override.json",
      "info"
    );
  }

  const runnerPath = join(ctx.projectRoot, config.runner_path);
  if (!existsSync(runnerPath)) {
    ctx.showToast(`❌ Runner not found: ${config.runner_path}`, "error");
    return;
  }

  try {
    accessSync(runnerPath, constants.X_OK);
  } catch {
    ctx.showToast(
      `❌ Runner not executable: ${config.runner_path} — try \`chmod +x ${config.runner_path}\``,
      "error"
    );
    return;
  }

  const instructionValidation = validateAllGateInstructions(
    config,
    ctx.projectRoot,
    config.strict_instructions
  );

  if (!instructionValidation.valid) {
    ctx.showToast(
      `❌ Missing gate instruction docs:\n${instructionValidation.warnings.join("\n")}`,
      "error"
    );
    return;
  }

  if (instructionValidation.warnings.length > 0) {
    ctx.showToast(
      `⚠️ Some gates have no instruction docs:\n${instructionValidation.warnings.join("\n")}`,
      "warning"
    );
  }

  const controller = createLoopStateController(ctx.projectRoot);

  try {
    const messageCount = ctx.getMessageCount();

    controller.startLoop(
      ctx.sessionId,
      config,
      options.featureId,
      options.issueNumber,
      options.story,
      messageCount
    );

    ctx.showToast(
      `🚀 Harness loop started with ${config.gates.length} gates`,
      "info"
    );

    const openingPrompt = buildOpeningPrompt(config, options.featureId ?? null);
    await ctx.injectMessage(openingPrompt);
  } catch (e) {
    if (e instanceof LoopAlreadyActiveError) {
      const answer = await ctx.askQuestion({
        question: `Loop already active in session ${e.existingSessionId} at gate "${e.existingGate}". What would you like to do?`,
        choices: ["Resume", "Cancel and restart", "Abort"],
      });

      if (answer === "Resume") {
        controller.rebindSession(ctx.sessionId);
        ctx.showToast(
          `🔄 Resuming loop at gate "${e.existingGate}"`,
          "info"
        );
      } else if (answer === "Cancel and restart") {
        controller.cancelLoop();

        const messageCount = ctx.getMessageCount();
        controller.startLoop(
          ctx.sessionId,
          config,
          options.featureId,
          options.issueNumber,
          options.story,
          messageCount
        );

        ctx.showToast(
          `🚀 Harness loop restarted with ${config.gates.length} gates`,
          "info"
        );

        const openingPrompt = buildOpeningPrompt(
          config,
          options.featureId ?? null
        );
        await ctx.injectMessage(openingPrompt);
      } else {
        ctx.showToast("❌ Loop start aborted", "warning");
      }
    } else {
      throw e;
    }
  }
}
