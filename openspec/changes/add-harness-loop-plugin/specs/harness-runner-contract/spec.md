## ADDED Requirements

### Requirement: Runner SHALL be invoked as a subprocess with structured arguments

The plugin SHALL invoke the project-configured runner as `<runner_path> <gate-name> [--feature=<id>] [--force] [--json]` using `child_process.spawn` with no shell interpolation, capturing stdout, stderr, and exit code.

#### Scenario: Standard runner invocation
- **WHEN** the plugin needs to execute gate "pre-work" for feature "feat-NNN-foo"
- **THEN** the plugin SHALL spawn `<runner_path>` with argv `["pre-work", "--feature=feat-NNN-foo", "--json"]`, env inherited, cwd set to the project root, and a hard timeout from config (default 300 seconds)

#### Scenario: Runner invocation with --force flag
- **WHEN** the loop was started via `/harness-on --force` or a specific gate has `force: true` policy
- **THEN** the plugin SHALL include `--force` in the runner argv

#### Scenario: Runner timeout exceeded
- **WHEN** the runner subprocess does not exit within the configured timeout
- **THEN** the plugin SHALL send SIGTERM, wait 5 seconds, then SIGKILL, and treat the result as `{status: "ERROR", instructions_for_agent: "Runner timed out after <N>s"}`

### Requirement: Runner SHALL emit a single JSON object on stdout

The runner's stdout SHALL be exactly one JSON object matching the contract schema. Any extra output before or after the JSON object SHALL be treated as parse error.

#### Scenario: Valid JSON output
- **WHEN** the runner exits successfully and stdout contains exactly one JSON object
- **THEN** the plugin SHALL parse it with strict schema validation (no unknown fields treated as error)

#### Scenario: stdout contains multiple JSON objects
- **WHEN** the runner emits two JSON objects separated by whitespace
- **THEN** the plugin SHALL treat this as a contract violation: status forced to ERROR, message "Runner emitted multiple JSON objects; contract requires exactly one"

#### Scenario: stdout contains non-JSON prefix/suffix
- **WHEN** the runner emits "Running checks...\n{json}\nDone.\n"
- **THEN** the plugin SHALL treat the non-JSON content as ERROR; runners MUST emit JSON-only on stdout (logs go to stderr)

#### Scenario: stderr is captured but does not affect parsing
- **WHEN** the runner emits log lines on stderr alongside the JSON on stdout
- **THEN** the plugin SHALL capture stderr for diagnostics (surface in toast on ERROR) but SHALL NOT parse stderr as part of the contract

### Requirement: Runner output schema SHALL match a strict structure

The JSON object SHALL match this Zod-equivalent schema:

```jsonc
{
  "gate": "string (required, must equal the gate name passed in argv)",
  "status": "PASS | FAIL | SKIP | WAITING | BLOCKED | ERROR (required)",
  "checks": [
    {
      "id": "string",
      "name": "string",
      "status": "PASS | FAIL | SKIP",
      "rule_id": "string (optional, e.g., R29 or FP #37)",
      "message": "string (optional)"
    }
  ],
  "next_gate": "string | null (optional, hint for plugin)",
  "instructions_for_agent": "string (required when status in [FAIL, BLOCKED])",
  "wait_seconds": "number (required when status == WAITING)",
  "rule_ids_violated": ["string"]
}
```

#### Scenario: Missing required field
- **WHEN** runner returns `{"gate": "pre-work", "status": "FAIL"}` without `instructions_for_agent`
- **THEN** the plugin SHALL treat as contract violation, log the error, and inject a synthetic instructions message "Runner returned FAIL without instructions; check runner implementation"

#### Scenario: Status WAITING without wait_seconds
- **WHEN** runner returns `{"status": "WAITING"}` without `wait_seconds`
- **THEN** the plugin SHALL default to 30 seconds and emit a warning toast

#### Scenario: Gate name mismatch
- **WHEN** the plugin invoked the runner with gate="pre-work" but the response has `gate: "in-progress"`
- **THEN** the plugin SHALL treat as ERROR and refuse to act on the response

#### Scenario: Unknown status value
- **WHEN** runner returns `status: "MAYBE"`
- **THEN** the plugin SHALL treat as ERROR, never as PASS

### Requirement: Exit code SHALL match the status field

The runner's exit code SHALL semantically match the status field per this table.

#### Scenario: Exit code mapping
- **WHEN** runner emits status=PASS
- **THEN** runner SHALL exit 0; plugin SHALL warn if exit code disagrees with stated status

#### Scenario: Exit code table enforcement
- **WHEN** runner status is PASS, FAIL, SKIP, WAITING, BLOCKED, or ERROR
- **THEN** exit code SHALL be 0, 1, 2, 3, 4, or 5 respectively; plugin SHALL log a warning if mismatch detected but SHALL trust the JSON `status` field as authoritative

### Requirement: instructions_for_agent SHALL be safe to inject verbatim

The `instructions_for_agent` string SHALL be plain text suitable for direct embedding into the agent's message stream — no escape sequences, no executable code, no prompt injection vectors.

#### Scenario: Instructions contain newlines
- **WHEN** the runner emits `instructions_for_agent: "Step 1: do X\nStep 2: do Y"`
- **THEN** the plugin SHALL preserve newlines when embedding in the continuation prompt

#### Scenario: Instructions exceed length limit
- **WHEN** `instructions_for_agent` exceeds 8000 characters
- **THEN** the plugin SHALL truncate to 8000 characters with a "...[truncated]" suffix and emit a warning

#### Scenario: Instructions contain backtick code blocks
- **WHEN** runner emits instructions with markdown code blocks (e.g., suggested fix commands)
- **THEN** the plugin SHALL embed them unchanged — markdown is part of the expected format

### Requirement: Runner contract SHALL be language-agnostic

The plugin SHALL impose no restriction on the runner's implementation language, runtime, or dependencies, beyond emitting valid JSON on stdout and using the documented exit codes.

#### Scenario: Bash runner
- **WHEN** the configured runner is a bash script that uses `jq` to emit JSON
- **THEN** the plugin SHALL invoke it and parse output identically to any other runner

#### Scenario: Python runner
- **WHEN** the configured runner is a Python script (e.g., capyhome's `harness-state.py` extended into a full runner)
- **THEN** the plugin SHALL invoke and parse identically

#### Scenario: Compiled binary runner
- **WHEN** the configured runner is a compiled Go/Rust binary
- **THEN** the plugin SHALL invoke and parse identically

### Requirement: Runner SHALL be discoverable and validatable before loop start

When `/harness-on` is invoked, the plugin SHALL verify the runner exists and is executable before starting the loop.

#### Scenario: Runner path does not exist
- **WHEN** `/harness-on` invoked and `config.runner_path` points to a file that does not exist
- **THEN** the plugin SHALL refuse to start the loop, print "Runner not found: <path>", and NOT modify state

#### Scenario: Runner exists but is not executable
- **WHEN** the runner file exists but lacks execute permissions
- **THEN** the plugin SHALL refuse to start, print "Runner not executable: <path> — try `chmod +x <path>`"

#### Scenario: Runner dry-run on plugin install
- **WHEN** the plugin loads for the first time in a session (or after config change)
- **THEN** the plugin SHALL optionally run `<runner> --validate` (if supported) and warn the user if validation fails — non-blocking
