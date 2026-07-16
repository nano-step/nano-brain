## ADDED Requirements

### Requirement: `nano-brain init -- <agent>` shall install the requested agent bootstrap in the current workspace

`nano-brain init` SHALL accept a `--` separator followed by an agent target token. When the target is `opencode`, the command SHALL create or update a workspace-local `.opencode/commands/nano-brain.md` file in the current project and SHALL leave `~/.nano-brain/config.yml` untouched. Re-running the command with the same target SHALL be idempotent: the installed file content remains stable and unrelated workspace files are preserved.

#### Scenario: OpenCode install writes the workspace command file
- **WHEN** the user runs `nano-brain init -- opencode` in a workspace
- **THEN** `.opencode/commands/nano-brain.md` exists in that workspace
- **AND THEN** the generated file contains the nano-brain bootstrap instructions for OpenCode

#### Scenario: Reinstall is idempotent
- **WHEN** the user runs `nano-brain init -- opencode` twice in the same workspace
- **THEN** the second run does not create duplicate command files
- **AND THEN** unrelated files under `.opencode/commands/` remain unchanged

#### Scenario: Unsupported agent target is rejected
- **WHEN** the user runs `nano-brain init -- imaginary-agent`
- **THEN** the command exits with a non-zero status
- **AND THEN** it prints a clear message that the agent target is not supported

### Requirement: The installed OpenCode bootstrap shall target a workspace-local config file

The generated OpenCode command SHALL instruct the user or agent to create and use `.nanobrain/config.yml` in the current workspace, not `~/.nano-brain/config.yml`, and SHALL reference the home config only as an optional seed when it already exists. The installed command SHALL not write to the home config path as part of the install flow.

#### Scenario: Bootstrap mentions the workspace-local config path
- **WHEN** a user inspects `.opencode/commands/nano-brain.md` after installation
- **THEN** the instructions reference `.nanobrain/config.yml` in the current workspace

#### Scenario: Existing home config is treated as an optional seed
- **WHEN** `~/.nano-brain/config.yml` already exists
- **THEN** the bootstrap instructions mention it as the starting point for the workspace-local config
- **AND THEN** the instructions still direct the user or agent to write the active config under `.nanobrain/config.yml`

#### Scenario: Install does not mutate the home config
- **WHEN** the install flow completes successfully
- **THEN** `~/.nano-brain/config.yml` is unchanged
- **AND THEN** the workspace-local command file is the only new install artifact
