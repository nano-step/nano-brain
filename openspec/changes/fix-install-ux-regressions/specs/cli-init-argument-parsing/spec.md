# cli-init-argument-parsing Specification

## Purpose

`nano-brain init` is the documented way to register a workspace from a script, a runbook, or an agent. Its arguments must parse on the first pass, in the flag forms the documentation already advertises, and a root that can never be indexed must be rejected rather than silently registered.

## ADDED Requirements

### Requirement: `init` SHALL parse its full flag set in a single pass

`runInitCmd` SHALL parse `--`, `--yes`, `--root`, `--workspace`, `--json`, and `--force` in one pass. No flag SHALL be consumed by a pre-scan that leaves its value to be rejected as an unexpected argument.

#### Scenario: Space form is accepted

- **WHEN** the user runs `nano-brain init --root /path/to/project`
- **THEN** the command registers that path
- **AND** it does not print `unexpected argument`

#### Scenario: Equals form is accepted

- **WHEN** the user runs `nano-brain init --root=/path/to/project`
- **THEN** the command registers that path
- **AND** it does not print `unknown flag`

#### Scenario: Value flags combine with other flags

- **WHEN** the user runs `nano-brain init --root /path/to/project --json`
- **THEN** the command registers the path and emits JSON output

#### Scenario: Workspace flag parses

- **WHEN** the user runs `nano-brain init --root /path/to/project --workspace my-workspace`
- **THEN** both values are parsed and neither is reported as an unknown flag or unexpected argument

#### Scenario: Force flag reaches its implementation

- **WHEN** the user runs `nano-brain init --root /path/to/project --force`
- **THEN** the reset-workspace flow is reached rather than the command exiting during argument parsing

#### Scenario: Agent-target form keeps working

- **WHEN** the user runs `nano-brain init -- opencode`
- **THEN** the agent install path runs exactly as before this change

#### Scenario: Yes flag with a root reports its own guidance

- **WHEN** the user runs `nano-brain init --yes --root /path/to/project`
- **THEN** the command reaches the note explaining that `--yes` only writes config and that `--root` is ignored
- **AND** it does not exit with `unexpected argument`

### Requirement: The parser SHALL be testable without process exit

Argument parsing SHALL be available as a pure function returning parsed options and an error, so that every invocation form can be asserted in a unit test without spawning a subprocess.

#### Scenario: Invalid input returns an error rather than exiting

- **WHEN** the parser is called directly with an unrecognized flag
- **THEN** it returns an error describing the flag
- **AND** it does not terminate the process

### Requirement: Workspace registration SHALL reject a root that cannot be indexed

`POST /api/v1/init` SHALL reject a `root_path` that does not exist, is not a directory, or cannot be read, with a distinct message for each case. Validation SHALL occur at the handler so that the CLI, the interactive wizard, MCP tools, and direct API callers are all covered.

#### Scenario: Nonexistent path is rejected

- **WHEN** a client registers a `root_path` that does not exist on the daemon's filesystem
- **THEN** the request fails with a client error naming the path and stating that it does not exist
- **AND** no workspace and no collections are created

#### Scenario: Regular file is rejected

- **WHEN** a client registers a `root_path` that resolves to a regular file
- **THEN** the request fails with a client error stating that the path is not a directory

#### Scenario: Unreadable directory is rejected

- **WHEN** a client registers a `root_path` that is a directory the daemon cannot read
- **THEN** the request fails with a client error stating that the directory cannot be read

#### Scenario: Relative path is stored absolute

- **WHEN** a client registers `--root=.`
- **THEN** the registered workspace root is the absolute resolved path

#### Scenario: Remote daemon rejects a local-only path

- **WHEN** a developer runs `init --root <path>` against a remote daemon and the path exists only on the developer's machine
- **THEN** the request is rejected with the nonexistent-path error rather than registering a workspace the daemon can never index
