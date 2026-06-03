import { spawn, type ChildProcess } from "node:child_process";
import { existsSync, accessSync, constants } from "node:fs";
import { join } from "node:path";
import {
  RunnerOutputSchema,
  RunnerContractError,
  type RunnerOutput,
  type HarnessConfig,
} from "./types.js";
import {
  RUNNER_SIGKILL_GRACE_MS,
  EXIT_CODE_MAP,
  DEFAULT_RUNNER_TIMEOUT_SECONDS,
} from "./constants.js";

export interface RunnerInvokeOptions {
  featureId?: string;
  force?: boolean;
}

function createSyntheticError(
  gate: string,
  message: string
): RunnerOutput {
  return {
    gate,
    status: "ERROR",
    checks: [],
    rule_ids_violated: [],
    instructions_for_agent: message,
  };
}

function isExecutable(filePath: string): boolean {
  try {
    accessSync(filePath, constants.X_OK);
    return true;
  } catch {
    return false;
  }
}

function parseRunnerOutput(
  stdout: string,
  gate: string
): RunnerOutput {
  const trimmed = stdout.trim();

  if (trimmed.length === 0) {
    throw new RunnerContractError("Runner emitted no output on stdout");
  }

  const jsonMatches = trimmed.match(/^\s*\{[\s\S]*\}\s*$/);
  if (!jsonMatches) {
    throw new RunnerContractError(
      "Runner stdout must be exactly one JSON object. Found non-JSON or malformed content."
    );
  }

  const jsonCount = (trimmed.match(/^\s*\{/gm) || []).length;
  if (jsonCount > 1) {
    throw new RunnerContractError(
      "Runner emitted multiple JSON objects; contract requires exactly one"
    );
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch (e) {
    throw new RunnerContractError(
      `Runner output is not valid JSON: ${e instanceof Error ? e.message : String(e)}`
    );
  }

  const result = RunnerOutputSchema.safeParse(parsed);
  if (!result.success) {
    const fieldErrors = result.error.errors
      .map((e) => `${e.path.join(".")}: ${e.message}`)
      .join("; ");
    throw new RunnerContractError(
      `Runner contract violation: ${fieldErrors}`,
      result.error
    );
  }

  const output = result.data;

  if (output.gate !== gate) {
    throw new RunnerContractError(
      `Gate mismatch: invoked "${gate}" but runner returned "${output.gate}"`
    );
  }

  if (
    (output.status === "FAIL" || output.status === "BLOCKED") &&
    !output.instructions_for_agent
  ) {
    output.instructions_for_agent =
      "Runner returned " +
      output.status +
      " without instructions; check runner implementation";
  }

  if (output.status === "WAITING" && output.wait_seconds === undefined) {
    output.wait_seconds = 30;
  }

  return output;
}

export async function invokeRunner(
  config: HarnessConfig,
  gateName: string,
  projectRoot: string,
  options: RunnerInvokeOptions = {}
): Promise<RunnerOutput> {
  const runnerPath = join(projectRoot, config.runner_path);

  if (!existsSync(runnerPath)) {
    return createSyntheticError(
      gateName,
      `Runner not found: ${runnerPath}`
    );
  }

  if (!isExecutable(runnerPath)) {
    return createSyntheticError(
      gateName,
      `Runner not executable: ${runnerPath} — try \`chmod +x ${runnerPath}\``
    );
  }

  const args: string[] = [gateName, "--json"];
  if (options.featureId) {
    args.push(`--feature=${options.featureId}`);
  }
  if (options.force) {
    args.push("--force");
  }

  const timeoutMs =
    (config.runner_timeout_seconds ?? DEFAULT_RUNNER_TIMEOUT_SECONDS) * 1000;

  return new Promise<RunnerOutput>((resolve) => {
    let stdout = "";
    let stderr = "";
    let killed = false;
    let resolved = false;

    const proc: ChildProcess = spawn(runnerPath, args, {
      cwd: projectRoot,
      env: process.env,
      stdio: ["ignore", "pipe", "pipe"],
    });

    const timeout = setTimeout(() => {
      if (resolved) return;
      killed = true;
      proc.kill("SIGTERM");

      setTimeout(() => {
        if (!resolved) {
          proc.kill("SIGKILL");
        }
      }, RUNNER_SIGKILL_GRACE_MS);
    }, timeoutMs);

    proc.stdout?.on("data", (chunk: Buffer) => {
      stdout += chunk.toString();
    });

    proc.stderr?.on("data", (chunk: Buffer) => {
      stderr += chunk.toString();
    });

    proc.on("close", (exitCode) => {
      if (resolved) return;
      resolved = true;
      clearTimeout(timeout);

      if (killed) {
        resolve(
          createSyntheticError(
            gateName,
            `Runner timed out after ${timeoutMs / 1000}s`
          )
        );
        return;
      }

      try {
        const output = parseRunnerOutput(stdout, gateName);

        const expectedExitCode = EXIT_CODE_MAP[output.status];
        if (exitCode !== expectedExitCode && exitCode !== null) {
          console.warn(
            `[harness-loop] Exit code mismatch: runner exited ${exitCode} but status is ${output.status} (expected exit ${expectedExitCode})`
          );
        }

        resolve(output);
      } catch (e) {
        if (e instanceof RunnerContractError) {
          resolve(
            createSyntheticError(
              gateName,
              e.message + (stderr ? `\n\nRunner stderr:\n${stderr}` : "")
            )
          );
        } else {
          resolve(
            createSyntheticError(
              gateName,
              `Unexpected error parsing runner output: ${e instanceof Error ? e.message : String(e)}`
            )
          );
        }
      }
    });

    proc.on("error", (err) => {
      if (resolved) return;
      resolved = true;
      clearTimeout(timeout);
      resolve(
        createSyntheticError(
          gateName,
          `Failed to spawn runner: ${err.message}`
        )
      );
    });
  });
}
