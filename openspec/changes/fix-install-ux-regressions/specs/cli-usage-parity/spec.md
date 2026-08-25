# cli-usage-parity Specification

## Purpose

The command dispatcher, `nano-brain help`, and the agent-facing command table must never disagree about which commands exist. Drift in either direction is a silent failure: an undocumented command is undiscoverable, and a documented-but-absent command makes an agent fail with `Unknown command`.

## ADDED Requirements

### Requirement: `printUsage()` SHALL document every dispatched command

Every `case` label in the command-dispatch switch SHALL appear as a documented command in `printUsage()` output.

#### Scenario: The service command is documented

- **WHEN** the user runs `nano-brain help`
- **THEN** the output contains a `service` command line describing the managed-service subcommands

#### Scenario: Parity is enforced from the dispatcher, not a maintained list

- **WHEN** the usage-parity test runs
- **THEN** its expected command set is derived by parsing the dispatch switch in `cmd/nano-brain/main.go`
- **AND** adding a new `case` to that switch without adding a usage line causes the test to fail

#### Scenario: Matching is anchored to the command column

- **WHEN** the usage-parity test checks for a command named `get`
- **THEN** it matches only a usage line whose command column is exactly `get`
- **AND** it does not pass merely because `multi-get` appears in the output

### Requirement: `printUsage()` SHALL NOT document commands that are not dispatched

Every command documented in `printUsage()` SHALL have a corresponding `case` label in the dispatch switch.

#### Scenario: A removed command cannot stay documented

- **WHEN** a command is deleted from the dispatch switch but left in `printUsage()`
- **THEN** the usage-parity test fails

### Requirement: The agent-facing command table SHALL only name dispatched commands

The command table in root `SKILL.md` SHALL NOT list a command that the dispatcher does not accept.

#### Scenario: Phantom commands are removed

- **WHEN** the skill command table is checked against the dispatch switch
- **THEN** `update`, `embed`, `focus`, `graph-stats`, `symbols`, and `impact` are absent from the table, because no dispatch case exists for any of them

#### Scenario: An agent following the skill does not hit an unknown command

- **WHEN** an agent executes every command listed in the skill's command table
- **THEN** none of them exits with `Unknown command`

### Requirement: The install path SHALL recommend the supervised service, not only reference it

On a platform where the managed-service backend is usable, the setup wizard and the start-command suggestion SHALL surface `nano-brain service install` rather than presenting the unsupervised `serve -d` as the only option.

#### Scenario: Wizard offers the supervised option

- **WHEN** the interactive wizard reaches its serve step on a platform whose service backend reports usable
- **THEN** it offers or points at `nano-brain service install`
- **AND** its closing summary tells the user whether the daemon will survive a reboot

#### Scenario: Unsupported platform keeps the current behavior

- **WHEN** the service backend reports not usable
- **THEN** the wizard suggests `serve -d` exactly as before, with no mention of the service subcommand
