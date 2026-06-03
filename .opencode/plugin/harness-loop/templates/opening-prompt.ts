import { SYSTEM_DIRECTIVE_PREFIX } from "../constants.js";
import type { HarnessConfig } from "../types.js";

export function buildOpeningPrompt(
  config: HarnessConfig,
  featureId: string | null
): string {
  const gateList = config.gates.map((g, i) => `  ${i + 1}. ${g}`).join("\n");

  const parts = [
    `${SYSTEM_DIRECTIVE_PREFIX} — STARTING]`,
    "",
    "🚀 Harness loop activated.",
    "",
    "The loop will automatically drive you through these gates:",
    gateList,
    "",
    "After each agent idle, the harness will:",
    "1. Run the gate check",
    "2. If PASS → advance to next gate",
    "3. If FAIL → inject fix instructions and continue",
    "4. If BLOCKED → pause for human input",
    "",
    `Completion: Emit \`<promise>${config.completion_promise}</promise>\` when done, or the loop ends automatically when the final gate passes.`,
    "",
    'Override: Add "[HARNESS-OVERRIDE]: <reason>" to any reply to pause the loop for human approval.',
    "",
    "Cancel: Run /harness-off to stop the loop at any time.",
  ];

  if (featureId) {
    parts.push("", `Feature: ${featureId}`);
  }

  parts.push("", "Starting with gate: " + config.gates[0]);

  return parts.join("\n");
}
