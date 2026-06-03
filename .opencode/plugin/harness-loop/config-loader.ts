import { readFileSync, existsSync, unlinkSync } from "node:fs";
import { join } from "node:path";
import {
  HarnessConfigSchema,
  HarnessConfigError,
  type HarnessConfig,
  type ConfigOverrides,
  type ConfigLoadResult,
} from "./types.js";
import {
  DEFAULT_CONFIG_FILE_PATH,
  OVERRIDE_CONFIG_FILE_PATH,
} from "./constants.js";

function readJsonFile(filePath: string): unknown {
  const content = readFileSync(filePath, "utf-8");
  return JSON.parse(content);
}

function mergeConfigs(
  base: Partial<HarnessConfig>,
  override: Partial<HarnessConfig>
): Partial<HarnessConfig> {
  const merged = { ...base };

  for (const [key, value] of Object.entries(override)) {
    if (value === undefined) continue;

    const k = key as keyof HarnessConfig;

    if (k === "gate_instructions" || k === "phase_hooks") {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (merged as any)[k] = {
        ...(merged[k] as Record<string, unknown>),
        ...(value as Record<string, unknown>),
      };
    } else if (k === "gates" || k === "ultrawork_verify_gates") {
      merged[k] = value as string[];
    } else {
      (merged as Record<string, unknown>)[k] = value;
    }
  }

  return merged;
}

function applyCliOverrides(
  config: Partial<HarnessConfig>,
  cliArgs: ConfigOverrides
): Partial<HarnessConfig> {
  const result = { ...config };

  if (cliArgs.maxIter !== undefined) {
    result.max_total_iterations = cliArgs.maxIter;
  }

  if (cliArgs.skipGate !== undefined && cliArgs.skipGate.length > 0) {
    const currentGates = result.gates ?? [];
    result.gates = currentGates.filter((g) => !cliArgs.skipGate?.includes(g));
  }

  return result;
}

export function loadConfig(
  projectRoot: string,
  cliArgs: ConfigOverrides = {}
): ConfigLoadResult {
  let merged: Partial<HarnessConfig> = {};
  let overrideConsumed = false;

  const configPath =
    cliArgs.configPath ?? join(projectRoot, DEFAULT_CONFIG_FILE_PATH);
  const overridePath = join(projectRoot, OVERRIDE_CONFIG_FILE_PATH);

  if (!existsSync(configPath)) {
    throw new HarnessConfigError(
      `Config file not found: ${configPath}. Create .opencode/harness.config.json with at least runner_path and gates.`
    );
  }

  try {
    const projectConfig = readJsonFile(configPath);
    merged = mergeConfigs(merged, projectConfig as Partial<HarnessConfig>);
  } catch (error) {
    if (error instanceof SyntaxError) {
      throw new HarnessConfigError(
        `Invalid JSON in config file: ${configPath}`,
        undefined
      );
    }
    throw error;
  }

  if (existsSync(overridePath)) {
    try {
      const overrideConfig = readJsonFile(overridePath);
      merged = mergeConfigs(merged, overrideConfig as Partial<HarnessConfig>);
      overrideConsumed = true;

      unlinkSync(overridePath);
    } catch (error) {
      if (error instanceof SyntaxError) {
        throw new HarnessConfigError(
          `Invalid JSON in override file: ${overridePath}`,
          undefined
        );
      }
      throw error;
    }
  }

  merged = applyCliOverrides(merged, cliArgs);

  const parseResult = HarnessConfigSchema.safeParse(merged);

  if (!parseResult.success) {
    const fieldErrors = parseResult.error.errors
      .map((e) => `  - ${e.path.join(".")}: ${e.message}`)
      .join("\n");

    throw new HarnessConfigError(
      `Invalid harness config:\n${fieldErrors}`,
      parseResult.error
    );
  }

  return {
    config: parseResult.data,
    overrideConsumed,
  };
}

export function getDefaultConfig(): HarnessConfig {
  return HarnessConfigSchema.parse({
    runner_path: "./scripts/harness-check.sh",
    gates: ["default"],
  });
}
