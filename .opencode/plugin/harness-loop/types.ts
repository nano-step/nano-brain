import { z } from "zod";

// =============================================================================
// Runner Contract Types (from harness-runner-contract spec)
// =============================================================================

/**
 * Status values that a runner can return.
 */
export const RunnerStatusSchema = z.enum([
  "PASS",
  "FAIL",
  "SKIP",
  "WAITING",
  "BLOCKED",
  "ERROR",
]);
export type RunnerStatus = z.infer<typeof RunnerStatusSchema>;

/**
 * Individual check result within a gate.
 */
export const RunnerCheckSchema = z.object({
  id: z.string(),
  name: z.string(),
  status: z.enum(["PASS", "FAIL", "SKIP"]),
  rule_id: z.string().optional(),
  message: z.string().optional(),
});
export type RunnerCheck = z.infer<typeof RunnerCheckSchema>;

/**
 * Output from the runner script.
 * Strict schema: rejects unknown fields.
 */
export const RunnerOutputSchema = z
  .object({
    gate: z.string(),
    status: RunnerStatusSchema,
    checks: z.array(RunnerCheckSchema).default([]),
    next_gate: z.string().nullable().optional(),
    instructions_for_agent: z.string().optional(),
    wait_seconds: z.number().optional(),
    rule_ids_violated: z.array(z.string()).default([]),
  })
  .strict();
export type RunnerOutput = z.infer<typeof RunnerOutputSchema>;

// =============================================================================
// Configuration Types (from harness-loop-config spec)
// =============================================================================

/**
 * Per-gate instruction configuration.
 */
export const GateInstructionSchema = z.object({
  doc: z.string().optional(),
  skills: z.array(z.string()).default([]),
  async: z.boolean().default(false),
  async_max_wait_seconds: z.number().default(1800),
  async_poll_interval_seconds: z.number().default(60),
  async_subagent_type: z.string().default("quick"),
  force: z.boolean().default(false),
});
export type GateInstruction = z.infer<typeof GateInstructionSchema>;

/**
 * Phase hooks for before/after gate execution.
 */
export const PhaseHookSchema = z.object({
  before: z.string().optional(),
  after: z.string().optional(),
});
export type PhaseHook = z.infer<typeof PhaseHookSchema>;

/**
 * Fail policy for handling gate failures.
 */
export const FailPolicySchema = z.enum(["auto", "hybrid", "ask"]);
export type FailPolicy = z.infer<typeof FailPolicySchema>;

/**
 * Main harness configuration schema.
 */
export const HarnessConfigSchema = z
  .object({
    runner_path: z.string(),
    gates: z.array(z.string()).min(1),
    fail_policy: FailPolicySchema.default("hybrid"),
    rule_id_format: z.string().default("{id}"),
    max_total_iterations: z.number().default(100),
    max_iterations_per_gate: z.number().default(10),
    auto_fix_attempts: z.number().default(3),
    cache_ttl_minutes: z.number().default(30),
    runner_timeout_seconds: z.number().default(300),
    completion_promise: z.string().default("HARNESS-COMPLETE"),
    ultrawork_verify_gates: z.array(z.string()).default([]),
    state_file_path: z.string().default(".opencode/harness-loop.local.json"),
    gate_instructions: z.record(z.string(), GateInstructionSchema).default({}),
    phase_hooks: z.record(z.string(), PhaseHookSchema).default({}),
    strict_instructions: z.boolean().default(false),
    async_heartbeats: z.boolean().default(true),
  })
  .strict();
export type HarnessConfig = z.infer<typeof HarnessConfigSchema>;

/**
 * Snapshot of config stored in state for resume scenarios.
 */
export type ConfigSnapshot = HarnessConfig;

// =============================================================================
// State Types (from harness-loop-state spec)
// =============================================================================

/**
 * Checkpoint entry (capyhome-compat format).
 */
export interface CheckpointEntry {
  status: RunnerStatus;
  checked_at: string; // ISO 8601
  story?: string;
  issue_number?: number;
  checks?: Record<string, { status: string; message?: string }>;
}

/**
 * Loop metadata (plugin-private).
 */
export interface LoopMeta {
  active: boolean;
  current_gate: string;
  gate_iteration: number;
  total_iteration: number;
  max_iterations_per_gate: number;
  max_total_iterations: number;
  started_at: string; // ISO 8601
  session_id: string;
  config_snapshot: ConfigSnapshot;
  last_runner_output: RunnerOutput | null;
  no_progress_count: number;
  override_active: boolean;
  same_error_history: Record<string, string[][]>;
  verification_pending: boolean;
  watcher_task_id: string | null;
  message_count_at_start: number;
}

/**
 * Full harness loop state file schema.
 */
export interface HarnessLoopState {
  feature_id: string | null;
  issue_number: number | null;
  story: string | null;
  updated_at: string; // ISO 8601
  checkpoints: Record<string, CheckpointEntry>;
  loop: LoopMeta;
}

/**
 * Zod schema for state validation on read/write.
 */
export const LoopMetaSchema = z.object({
  active: z.boolean(),
  current_gate: z.string(),
  gate_iteration: z.number(),
  total_iteration: z.number(),
  max_iterations_per_gate: z.number(),
  max_total_iterations: z.number(),
  started_at: z.string(),
  session_id: z.string(),
  config_snapshot: HarnessConfigSchema,
  last_runner_output: RunnerOutputSchema.nullable(),
  no_progress_count: z.number(),
  override_active: z.boolean(),
  same_error_history: z.record(z.string(), z.array(z.array(z.string()))),
  verification_pending: z.boolean(),
  watcher_task_id: z.string().nullable(),
  message_count_at_start: z.number(),
});

export const CheckpointEntrySchema = z.object({
  status: RunnerStatusSchema,
  checked_at: z.string(),
  story: z.string().optional(),
  issue_number: z.number().optional(),
  checks: z
    .record(
      z.string(),
      z.object({
        status: z.string(),
        message: z.string().optional(),
      })
    )
    .optional(),
});

export const HarnessLoopStateSchema = z.object({
  feature_id: z.string().nullable(),
  issue_number: z.number().nullable(),
  story: z.string().nullable(),
  updated_at: z.string(),
  checkpoints: z.record(z.string(), CheckpointEntrySchema),
  loop: LoopMetaSchema,
});

// =============================================================================
// CLI Override Types
// =============================================================================

/**
 * CLI argument overrides for /harness-on command.
 */
export interface ConfigOverrides {
  force?: boolean;
  maxIter?: number;
  skipGate?: string[];
  configPath?: string;
  startFromGate?: string;
}

// =============================================================================
// Error Types
// =============================================================================

/**
 * Error thrown when harness config is invalid.
 */
export class HarnessConfigError extends Error {
  constructor(
    message: string,
    public readonly details?: z.ZodError
  ) {
    super(message);
    this.name = "HarnessConfigError";
  }
}

/**
 * Error thrown when state file is corrupted.
 */
export class StateCorruptionError extends Error {
  constructor(
    message: string,
    public readonly filePath: string
  ) {
    super(message);
    this.name = "StateCorruptionError";
  }
}

/**
 * Error thrown when runner contract is violated.
 */
export class RunnerContractError extends Error {
  constructor(
    message: string,
    public readonly details?: z.ZodError
  ) {
    super(message);
    this.name = "RunnerContractError";
  }
}

// =============================================================================
// Result Types
// =============================================================================

/**
 * Result of gate instructions resolution.
 */
export interface GateInstructionsResult {
  docPath: string | null;
  skills: string[];
  warning: string | null;
  isAsync: boolean;
  asyncConfig: {
    maxWaitSeconds: number;
    pollIntervalSeconds: number;
    subagentType: string;
  } | null;
}

/**
 * Completion detection result.
 */
export type CompletionSource = "promise_tag" | "structural" | null;

/**
 * Config load result.
 */
export interface ConfigLoadResult {
  config: HarnessConfig;
  overrideConsumed: boolean;
}
