import { existsSync } from "node:fs";
import { join } from "node:path";
import { CONVENTION_GATE_DOC_PATH } from "./constants.js";
import type { HarnessConfig, GateInstructionsResult } from "./types.js";

export function resolveGateInstructions(
  config: HarnessConfig,
  gateName: string,
  projectRoot: string
): GateInstructionsResult {
  const gateConfig = config.gate_instructions[gateName];

  let docPath: string | null = null;
  let warning: string | null = null;

  if (gateConfig?.doc) {
    const explicitPath = join(projectRoot, gateConfig.doc);
    if (existsSync(explicitPath)) {
      docPath = gateConfig.doc;
    } else {
      warning = `Gate instruction doc not found: ${gateConfig.doc}`;
    }
  } else {
    const conventionPath = join(
      projectRoot,
      CONVENTION_GATE_DOC_PATH,
      `${gateName}.md`
    );
    if (existsSync(conventionPath)) {
      docPath = join(CONVENTION_GATE_DOC_PATH, `${gateName}.md`);
    }
  }

  if (docPath === null && gateConfig?.skills && gateConfig.skills.length > 0) {
    warning =
      warning ??
      `No instruction doc found for gate "${gateName}". Using general best practices.`;
  }

  if (
    docPath === null &&
    (!gateConfig?.skills || gateConfig.skills.length === 0)
  ) {
    warning =
      warning ??
      `No instruction doc or skills configured for gate "${gateName}". Using general best practices.`;
  }

  const skills = gateConfig?.skills ?? [];

  const isAsync = gateConfig?.async ?? false;
  const asyncConfig = isAsync
    ? {
        maxWaitSeconds: gateConfig?.async_max_wait_seconds ?? 1800,
        pollIntervalSeconds: gateConfig?.async_poll_interval_seconds ?? 60,
        subagentType: gateConfig?.async_subagent_type ?? "quick",
      }
    : null;

  return {
    docPath,
    skills,
    warning,
    isAsync,
    asyncConfig,
  };
}

export function validateAllGateInstructions(
  config: HarnessConfig,
  projectRoot: string,
  strict: boolean
): { valid: boolean; warnings: string[] } {
  const warnings: string[] = [];

  for (const gate of config.gates) {
    const result = resolveGateInstructions(config, gate, projectRoot);
    if (result.warning && !result.docPath) {
      warnings.push(result.warning);
    }
  }

  if (strict && warnings.length > 0) {
    return { valid: false, warnings };
  }

  return { valid: true, warnings };
}
