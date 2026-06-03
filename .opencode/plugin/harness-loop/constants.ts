export const DEFAULT_MAX_TOTAL_ITERATIONS = 100;
export const DEFAULT_MAX_PER_GATE = 10;
export const DEFAULT_AUTO_FIX_ATTEMPTS = 3;
export const DEFAULT_CACHE_TTL_MINUTES = 30;
export const DEFAULT_COMPLETION_PROMISE = "HARNESS-COMPLETE";
export const DEFAULT_RUNNER_TIMEOUT_SECONDS = 300;
export const USER_MESSAGE_IN_PROGRESS_WINDOW_MS = 2000;
export const IDLE_SETTLE_MS = 150;
export const OVERRIDE_TOKEN_REGEX = /^\[HARNESS-OVERRIDE\]:\s*(.+)$/m;
export const RUNNER_SIGKILL_GRACE_MS = 5000;
export const HARNESS_OFF_RUNNER_WAIT_MS = 30000;
export const INSTRUCTIONS_MAX_LENGTH = 8000;
export const SAME_ERROR_HISTORY_WINDOW = 5;
export const SAME_ERROR_TRIP_THRESHOLD = 3;
export const MAX_HEARTBEATS_PER_GATE = 3;
export const OUTER_GRACE_BUFFER_SECONDS = 30;

export const EXIT_CODE_MAP: Record<string, number> = {
  PASS: 0,
  FAIL: 1,
  SKIP: 2,
  WAITING: 3,
  BLOCKED: 4,
  ERROR: 5,
};

export const DEFAULT_STATE_FILE_PATH = ".opencode/harness-loop.local.json";
export const DEFAULT_CONFIG_FILE_PATH = ".opencode/harness.config.json";
export const OVERRIDE_CONFIG_FILE_PATH = ".opencode/harness.override.json";
export const CONVENTION_GATE_DOC_PATH = "docs/harness/gates";

export const SYSTEM_DIRECTIVE_PREFIX =
  "[SYSTEM_DIRECTIVE — HARNESS LOOP" as const;
