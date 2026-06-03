import { INSTRUCTIONS_MAX_LENGTH } from "./constants.js";
import type {
  HarnessConfig,
  HarnessLoopState,
  RunnerOutput,
} from "./types.js";
import {
  buildFailPrompt,
  buildBlockedPrompt,
  buildWaitingPrompt,
  buildPassTransitionPrompt,
  buildCompletionPrompt,
  type ContinuationPromptContext,
} from "./templates/continuation-prompt.js";
import { resolveGateInstructions } from "./gate-instructions-resolver.js";

function formatRuleId(ruleId: string, format: string): string {
  const alreadyFormatted =
    ruleId.startsWith("R") ||
    ruleId.startsWith("FP ") ||
    ruleId.includes("#") ||
    ruleId.includes("-");

  if (alreadyFormatted) {
    return ruleId;
  }

  return format.replace("{id}", ruleId);
}

function formatRuleIds(ruleIds: string[], format: string): string[] {
  return ruleIds.map((id) => formatRuleId(id, format));
}

function truncateInstructions(instructions: string): string {
  if (instructions.length <= INSTRUCTIONS_MAX_LENGTH) {
    return instructions;
  }

  return instructions.slice(0, INSTRUCTIONS_MAX_LENGTH - 15) + "...[truncated]";
}

export function buildContinuationPrompt(
  state: HarnessLoopState,
  runnerOutput: RunnerOutput,
  config: HarnessConfig,
  projectRoot: string
): string {
  const gateInstructions = resolveGateInstructions(
    config,
    runnerOutput.gate,
    projectRoot
  );

  const formattedRuleIds = formatRuleIds(
    runnerOutput.rule_ids_violated,
    config.rule_id_format
  );

  const outputWithFormattedIds: RunnerOutput = {
    ...runnerOutput,
    rule_ids_violated: formattedRuleIds,
    instructions_for_agent: runnerOutput.instructions_for_agent
      ? truncateInstructions(runnerOutput.instructions_for_agent)
      : undefined,
  };

  const ctx: ContinuationPromptContext = {
    gate: runnerOutput.gate,
    gateIteration: state.loop.gate_iteration,
    maxIterationsPerGate: state.loop.max_iterations_per_gate,
    totalIteration: state.loop.total_iteration,
    maxTotalIterations: state.loop.max_total_iterations,
    featureId: state.feature_id,
    runnerOutput: outputWithFormattedIds,
    config,
    gateInstructions,
  };

  switch (runnerOutput.status) {
    case "FAIL":
      return buildFailPrompt(ctx);

    case "BLOCKED":
      return buildBlockedPrompt(ctx);

    case "WAITING":
      return buildWaitingPrompt(ctx, runnerOutput.wait_seconds ?? 30);

    case "PASS": {
      const gates = config.gates;
      const currentIndex = gates.indexOf(runnerOutput.gate);
      const nextGate =
        runnerOutput.next_gate ?? gates[currentIndex + 1] ?? null;

      if (nextGate) {
        return buildPassTransitionPrompt(runnerOutput.gate, nextGate);
      }

      return buildCompletionPrompt("structural");
    }

    case "SKIP": {
      const gates = config.gates;
      const currentIndex = gates.indexOf(runnerOutput.gate);
      const nextGate = gates[currentIndex + 1] ?? null;

      if (nextGate) {
        return buildPassTransitionPrompt(
          `${runnerOutput.gate} (skipped)`,
          nextGate
        );
      }

      return buildCompletionPrompt("structural");
    }

    case "ERROR":
      return buildFailPrompt(ctx);

    default: {
      const _: never = runnerOutput.status;
      void _;
      return buildFailPrompt(ctx);
    }
  }
}
