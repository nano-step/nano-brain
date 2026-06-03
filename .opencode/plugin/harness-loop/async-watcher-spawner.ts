import type { HarnessConfig, HarnessLoopState, RunnerOutput } from "./types.js";
import { RunnerOutputSchema } from "./types.js";

export interface WatcherSpawnContext {
  spawnBackgroundTask(
    subagentType: string,
    prompt: string
  ): Promise<string>;
}

export function buildWatcherPrompt(
  gate: string,
  config: HarnessConfig,
  state: HarnessLoopState
): string {
  const gateConfig = config.gate_instructions[gate];
  const maxWait = gateConfig?.async_max_wait_seconds ?? 1800;
  const pollInterval = gateConfig?.async_poll_interval_seconds ?? 60;
  const runnerPath = config.runner_path;
  const featureId = state.feature_id ?? "";

  return `You are a harness loop background watcher for gate "${gate}".

TASK: Poll the runner until it returns a terminal status (PASS, FAIL, BLOCKED) or timeout.

RUNNER COMMAND:
${runnerPath} ${gate} --json${featureId ? ` --feature=${featureId}` : ""}

POLL LOOP:
1. Run the runner command, capture stdout.
2. Parse JSON. Extract \`status\` field.
3. If status in [PASS, FAIL, BLOCKED] → output the JSON verbatim as your final response and stop.
4. If status == WAITING → sleep ${pollInterval} seconds.
5. If total elapsed > ${maxWait} seconds → output a synthesized JSON:
   {"gate": "${gate}", "status": "FAIL", "instructions_for_agent": "Watcher timed out after ${maxWait}s; gate did not reach terminal status", "rule_ids_violated": ["watcher-timeout"]}
   and stop.
6. Otherwise repeat from step 1.

OUTPUT FORMAT: Exactly one JSON object matching the RunnerOutput contract. Nothing else. No prose, no explanations.

CONSTRAINTS:
- Do NOT modify any files.
- Do NOT spawn other subagents.
- Do NOT use any tool other than bash.
- Total wall-clock time MUST be bounded by ${maxWait} seconds.`;
}

export async function spawnWatcher(
  ctx: WatcherSpawnContext,
  gate: string,
  config: HarnessConfig,
  state: HarnessLoopState
): Promise<string> {
  const gateConfig = config.gate_instructions[gate];
  const subagentType = gateConfig?.async_subagent_type ?? "quick";
  const prompt = buildWatcherPrompt(gate, config, state);

  return ctx.spawnBackgroundTask(subagentType, prompt);
}

export function parseWatcherResult(
  result: string,
  gate: string
): RunnerOutput {
  const trimmed = result.trim();

  const jsonMatch = trimmed.match(/\{[\s\S]*\}/);
  if (!jsonMatch) {
    return {
      gate,
      status: "ERROR",
      checks: [],
      rule_ids_violated: ["watcher-parse-error"],
      instructions_for_agent: "Watcher did not return valid JSON",
    };
  }

  try {
    const parsed = JSON.parse(jsonMatch[0]);
    const validated = RunnerOutputSchema.safeParse(parsed);

    if (!validated.success) {
      return {
        gate,
        status: "ERROR",
        checks: [],
        rule_ids_violated: ["watcher-schema-error"],
        instructions_for_agent: `Watcher returned invalid schema: ${validated.error.message}`,
      };
    }

    return validated.data;
  } catch (e) {
    return {
      gate,
      status: "ERROR",
      checks: [],
      rule_ids_violated: ["watcher-parse-error"],
      instructions_for_agent: `Failed to parse watcher result: ${e instanceof Error ? e.message : String(e)}`,
    };
  }
}
