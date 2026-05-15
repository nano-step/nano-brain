## MODIFIED Requirements

### Requirement: Workspace detection from PWD

The MCP server SHALL compute a `projectHash` from the **resolved** workspace root (after `resolveConfiguredWorkspace()` guard is applied) at startup using `sha256(resolvedWorkspaceRoot).substring(0, 12)`. This hash SHALL be stored as `currentProjectHash` on the server context and used for all default search filtering. The resolved root SHALL always be a workspace declared in `config.workspaces` when workspaces are configured.

#### Scenario: Server starts in a workspace directory that is in config

- **WHEN** the MCP server starts with `--root /Users/alice/projects/my-app`
- **AND** `/Users/alice/projects/my-app` is in `config.workspaces`
- **THEN** `currentProjectHash` is set to the first 12 characters of `sha256("/Users/alice/projects/my-app")`
- **THEN** the hash is consistent across restarts in the same directory

#### Scenario: Server starts with --root not in config — hash uses fallback path

- **WHEN** the MCP server starts with `--root /unconfigured/path`
- **AND** `config.workspaces` is non-empty and does not contain `/unconfigured/path`
- **THEN** `resolvedWorkspaceRoot` is set to the fallback workspace
- **THEN** `currentProjectHash` is computed from the fallback workspace path (NOT from `/unconfigured/path`)
- **THEN** NO database is created or opened for `/unconfigured/path`

#### Scenario: Hash matches harvester convention

- **WHEN** the MCP server computes `currentProjectHash` for a workspace
- **THEN** the hash matches the directory name used by the harvester for that workspace's sessions (`sessions/{projectHash}/*.md`)
