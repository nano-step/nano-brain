import { OVERRIDE_TOKEN_REGEX } from "./constants.js";
import type {
  HarnessConfig,
  HarnessLoopState,
  CompletionSource,
  RunnerOutput,
} from "./types.js";

export interface SessionContext {
  getMessages(): Array<{ role: string; content: string }>;
}

export function detectPromiseTag(
  messages: Array<{ role: string; content: string }>,
  completionPromise: string,
  messageCountAtStart: number
): boolean {
  const promisePattern = `<promise>${completionPromise}</promise>`;

  const recentMessages = messages.slice(messageCountAtStart);

  for (const msg of recentMessages) {
    if (msg.role === "assistant" && msg.content.includes(promisePattern)) {
      return true;
    }
  }

  return false;
}

export function detectStructuralCompletion(
  state: HarnessLoopState,
  runnerOutput: RunnerOutput | null,
  config: HarnessConfig
): boolean {
  if (runnerOutput === null) {
    return false;
  }

  if (runnerOutput.status !== "PASS") {
    return false;
  }

  if (runnerOutput.next_gate !== null && runnerOutput.next_gate !== undefined) {
    return false;
  }

  const gates = config.gates;
  const currentGate = state.loop.current_gate;
  const lastGate = gates[gates.length - 1];

  return currentGate === lastGate;
}

export function detectCompletion(
  messages: Array<{ role: string; content: string }>,
  state: HarnessLoopState,
  runnerOutput: RunnerOutput | null,
  config: HarnessConfig
): CompletionSource {
  if (
    detectPromiseTag(
      messages,
      config.completion_promise,
      state.loop.message_count_at_start
    )
  ) {
    return "promise_tag";
  }

  if (detectStructuralCompletion(state, runnerOutput, config)) {
    return "structural";
  }

  return null;
}

export function detectOverrideToken(
  messages: Array<{ role: string; content: string }>
): { found: boolean; reason: string | null } {
  for (let i = messages.length - 1; i >= 0; i--) {
    const msg = messages[i];
    if (msg?.role === "assistant") {
      const match = OVERRIDE_TOKEN_REGEX.exec(msg.content);
      if (match) {
        return { found: true, reason: match[1] ?? null };
      }
      break;
    }
  }

  return { found: false, reason: null };
}
