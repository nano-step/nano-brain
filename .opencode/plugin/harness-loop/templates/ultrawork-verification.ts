import { SYSTEM_DIRECTIVE_PREFIX } from "../constants.js";

export function buildUltraworkVerificationPrompt(gate: string): string {
  return [
    `${SYSTEM_DIRECTIVE_PREFIX} — VERIFICATION REQUIRED]`,
    "",
    `Gate "${gate}" passed automated checks. Oracle verification requested.`,
    "",
    "Before transitioning to the next gate, verify:",
    "1. All acceptance criteria are fully met",
    "2. No edge cases were missed",
    "3. The implementation matches the intent",
    "4. No security or performance concerns",
    "",
    "If verification passes, emit VERIFIED and the loop will continue.",
    "If issues found, list them and the loop will re-run this gate.",
  ].join("\n");
}
