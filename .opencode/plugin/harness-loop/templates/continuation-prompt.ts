import { SYSTEM_DIRECTIVE_PREFIX } from "../constants.js";
import type { RunnerOutput, HarnessConfig } from "../types.js";
import type { GateInstructionsResult } from "../types.js";

export interface ContinuationPromptContext {
  gate: string;
  gateIteration: number;
  maxIterationsPerGate: number;
  totalIteration: number;
  maxTotalIterations: number;
  featureId: string | null;
  runnerOutput: RunnerOutput;
  config: HarnessConfig;
  gateInstructions: GateInstructionsResult;
}

function buildInstructionsSection(
  gateInstructions: GateInstructionsResult
): string {
  const lines: string[] = [];

  if (gateInstructions.docPath) {
    lines.push(`📖 Read project's gate protocol FIRST (mandatory):`);
    lines.push(`   ${gateInstructions.docPath}`);
    lines.push("");
  } else if (gateInstructions.warning) {
    lines.push(`⚠️ ${gateInstructions.warning}`);
    lines.push("");
  }

  if (gateInstructions.skills.length > 0) {
    lines.push(`🔧 Load skills before attempting fix:`);
    for (const skill of gateInstructions.skills) {
      lines.push(`   - ${skill}`);
    }
    lines.push("");
  }

  return lines.join("\n");
}

export function buildFailPrompt(ctx: ContinuationPromptContext): string {
  const header = `${SYSTEM_DIRECTIVE_PREFIX} gate=${ctx.gate} iter=${ctx.gateIteration}/${ctx.maxIterationsPerGate} total=${ctx.totalIteration}/${ctx.maxTotalIterations}]`;

  const ruleIds =
    ctx.runnerOutput.rule_ids_violated.length > 0
      ? ctx.runnerOutput.rule_ids_violated.join(", ")
      : "none specified";

  const instructionsSection = buildInstructionsSection(ctx.gateInstructions);
  const runnerInstructions =
    ctx.runnerOutput.instructions_for_agent ?? "No specific instructions provided.";

  const parts = [
    header,
    "",
    `Gate "${ctx.gate}" failed. Rules violated: ${ruleIds}.`,
    "",
  ];

  if (instructionsSection) {
    parts.push(instructionsSection);
  }

  parts.push("Runner instructions:");
  parts.push(runnerInstructions);
  parts.push("");
  parts.push(
    `Fix the listed failures, then continue. The harness will re-check ${ctx.gate} automatically on your next idle.`
  );
  parts.push("");
  parts.push(
    'If you cannot fix and need human input, add "[HARNESS-OVERRIDE]: <reason>" to your reply and the loop will pause for user approval.'
  );

  if (ctx.featureId) {
    parts.push("");
    parts.push(`Original feature: ${ctx.featureId}`);
  }

  return parts.join("\n");
}

export function buildBlockedPrompt(ctx: ContinuationPromptContext): string {
  const header = `${SYSTEM_DIRECTIVE_PREFIX} gate=${ctx.gate} iter=${ctx.gateIteration}/${ctx.maxIterationsPerGate} total=${ctx.totalIteration}/${ctx.maxTotalIterations}]`;

  const instructionsSection = buildInstructionsSection(ctx.gateInstructions);
  const runnerInstructions =
    ctx.runnerOutput.instructions_for_agent ??
    "No specific instructions provided. Human intervention needed.";

  const parts = [
    header,
    "",
    `Gate "${ctx.gate}" is BLOCKED and requires human intervention.`,
    "",
  ];

  if (instructionsSection) {
    parts.push(instructionsSection);
  }

  parts.push("Runner instructions:");
  parts.push(runnerInstructions);
  parts.push("");
  parts.push(
    "This gate cannot proceed without user input. Please review the situation and provide guidance."
  );
  parts.push("");
  parts.push(
    "Options:\n" +
      "1. Provide instructions on how to proceed\n" +
      "2. Run /harness-off to cancel the loop\n" +
      '3. Add "[HARNESS-OVERRIDE]: <reason>" to skip this gate'
  );

  return parts.join("\n");
}

export function buildWaitingPrompt(
  ctx: ContinuationPromptContext,
  waitSeconds: number
): string {
  const header = `${SYSTEM_DIRECTIVE_PREFIX} gate=${ctx.gate} — WAITING]`;

  return [
    header,
    "",
    `Gate "${ctx.gate}" returned WAITING status.`,
    `The harness will re-check in ${waitSeconds} seconds.`,
    "",
    "Do not take any action — the loop will automatically retry.",
  ].join("\n");
}

export function buildPassTransitionPrompt(
  previousGate: string,
  nextGate: string
): string {
  return [
    `${SYSTEM_DIRECTIVE_PREFIX} — PASS]`,
    "",
    `✓ Gate "${previousGate}" passed.`,
    `Transitioning to gate "${nextGate}".`,
  ].join("\n");
}

export function buildCompletionPrompt(
  source: "promise_tag" | "structural"
): string {
  const reason =
    source === "promise_tag"
      ? "Completion promise tag detected."
      : "All gates passed and final gate reached.";

  return [
    `${SYSTEM_DIRECTIVE_PREFIX} — COMPLETE]`,
    "",
    `🎉 Harness loop complete!`,
    "",
    reason,
    "",
    "The loop has ended successfully.",
  ].join("\n");
}
