# workspace-scoping Delta Specification

## MODIFIED Requirements

### Requirement: Workspace hash derived from project directory path

The workspace hash SHALL be `SHA256(abs(project_directory_path))` where `project_directory_path` is:
- For `nano-brain init --root <dir>`: the `<dir>` argument (resolved to absolute path)
- For the OpenCode SQLite harvester: `project.worktree` from OpenCode's `project` table
- For the session-dir harvester (`OpenCodeHarvester`): the configured `harvester.opencode.session_dir`
- For the Claude Code harvester: the configured `harvester.claude_code.session_dir`

The workspace hash SHALL NOT be derived from the OpenCode DB file path.

#### Scenario: init --root and harvester produce matching hashes for same project

- **WHEN** user runs `nano-brain init --root /Users/alice/projects/my-app`
- **AND** OpenCode has a project with `worktree = "/Users/alice/projects/my-app"`
- **THEN** `WorkspaceHash("/Users/alice/projects/my-app")` is identical in both cases
- **THEN** sessions harvested for that project are immediately visible when querying that workspace
