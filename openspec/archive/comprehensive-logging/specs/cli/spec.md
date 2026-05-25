# Spec: CLI Logging

## ADDED Requirements

### Requirement: verbose flag
The CLI MUST support a `-v` flag (integer count) to override the logging level.

#### Scenario: Default — no flag
- Given no `-v` flag is passed
- When any CLI command runs
- Then the effective log level is `cfg.Logging.Level` from config (default: `info`)

#### Scenario: Single -v flag
- When `nano-brain -v <command>` is invoked
- Then the effective log level is `debug`

#### Scenario: -vv or -v=2
- When `nano-brain -v -v <command>` or equivalent is invoked
- Then the effective log level is `trace`

#### Scenario: --verbose synonym
- When `nano-brain --verbose <command>` is invoked
- Then the effective log level is `debug` (same as single -v)

### Requirement: TTY-aware console output
When stdout is an interactive terminal, the CLI MUST use human-readable console log format.

#### Scenario: Stdout is a TTY
- Given stdout is a character device
- When the server or CLI runs
- Then the stdout log stream uses ConsoleWriter (human-readable, colored unless NO_COLOR is set)
- And the file log stream remains JSON (machine-parseable)

#### Scenario: Stdout is not a TTY (pipe, redirect, CI)
- Given stdout is not a character device
- When the server or CLI runs
- Then stdout log stream is JSON (same as file stream)

### Requirement: CLI command lifecycle logs
Every CLI command that communicates with the server MUST emit at least one INFO log at command start and one at completion (success or failure).

#### Scenario: init --root success
- Given the server is reachable and the workspace is registered
- When `nano-brain init --root /path` is run with debug logging enabled
- Then the log file contains an entry with `"message":"registering workspace"` and `"root_path":"/path"`
- And the log file contains an entry with `"message":"workspace registered"` and a `workspace_hash` field

#### Scenario: CLI command error
- Given a CLI command fails (server error, network error)
- When the error is handled
- Then the log file contains an entry at `error` level with an `error` field containing the error message
- And the user still sees the error on stderr (printf not removed)

### Requirement: trace level support
The log level `"trace"` MUST be accepted in `logging.level` config and via `-v -v`.

#### Scenario: trace level in config
- Given `logging.level: trace` in config.yml
- When `parseLogLevel("trace")` is called
- Then it returns `zerolog.TraceLevel` without error
