# workspace-config-guard Specification

## Purpose
Prevent rogue database creation for unconfigured workspace paths. When `config.workspaces` is non-empty, the server bootstrap validates `effectiveWorkspaceRoot` against the configured set and silently redirects to the closest or first configured workspace before `createStore()` is called.

## Requirements

### Requirement: Validate workspace root against configured workspaces before DB creation

The server bootstrap SHALL validate `effectiveWorkspaceRoot` against `config.workspaces` before calling `createStore()`. If `config.workspaces` is non-empty and `effectiveWorkspaceRoot` is not a key in that map, the server SHALL NOT create a new database for that path. Instead, it SHALL fall back to the closest matching configured workspace (longest-prefix match, then first configured workspace) and emit a warning log.

#### Scenario: Root is in config.workspaces — no change

- **WHEN** the server starts with `--root /configured/path`
- **AND** `config.workspaces` contains `/configured/path`
- **THEN** `resolvedWorkspaceRoot` SHALL be `/configured/path`
- **THEN** a new database SHALL be created/opened for `/configured/path` as normal

#### Scenario: Root is not in config.workspaces — longest-prefix fallback

- **WHEN** the server starts with `--root /repo/sub-project`
- **AND** `config.workspaces` contains `/repo` but not `/repo/sub-project`
- **THEN** `resolvedWorkspaceRoot` SHALL be set to `/repo` (longest prefix match)
- **THEN** the server SHALL log a warning: `"Workspace /repo/sub-project is not in config.workspaces — falling back to /repo"`
- **THEN** NO new database SHALL be created for `/repo/sub-project`

#### Scenario: Root is not in config.workspaces — first-workspace fallback

- **WHEN** the server starts with `--root /unrelated/path`
- **AND** `config.workspaces` contains `/configured/a` and `/configured/b`
- **AND** neither is a prefix of `/unrelated/path`
- **THEN** `resolvedWorkspaceRoot` SHALL be set to `/configured/a` (first configured workspace)
- **THEN** the server SHALL log a warning: `"Workspace /unrelated/path is not in config.workspaces — falling back to /configured/a"`
- **THEN** NO new database SHALL be created for `/unrelated/path`

#### Scenario: config.workspaces is empty — no restriction (backward compatible)

- **WHEN** the server starts with `--root /any/path`
- **AND** `config.workspaces` is empty or undefined
- **THEN** `resolvedWorkspaceRoot` SHALL be `/any/path` (current behavior unchanged)
- **THEN** a database SHALL be created/opened for `/any/path` as before

#### Scenario: No --root provided, cwd not in config.workspaces

- **WHEN** the server starts without `--root` and `process.cwd()` is not in `config.workspaces`
- **AND** `config.workspaces` is non-empty
- **THEN** the server SHALL fall back to the closest matching or first configured workspace
- **THEN** the server SHALL log the fallback warning

### Requirement: resolveConfiguredWorkspace helper function

A `resolveConfiguredWorkspace(root: string, configuredWorkspaces: string[])` function SHALL be extracted in `bootstrap.ts` and used by both the daemon and non-daemon branches to produce the final `resolvedWorkspaceRoot`.

#### Scenario: Input matches a configured workspace exactly

- **WHEN** `resolveConfiguredWorkspace("/repo", ["/repo", "/other"])` is called
- **THEN** the function SHALL return `{ resolved: "/repo", fallback: false }`

#### Scenario: Input matches by longest prefix

- **WHEN** `resolveConfiguredWorkspace("/repo/sub", ["/repo", "/other"])` is called
- **THEN** the function SHALL return `{ resolved: "/repo", fallback: true }`

#### Scenario: No prefix match

- **WHEN** `resolveConfiguredWorkspace("/unrelated", ["/repo", "/other"])` is called
- **THEN** the function SHALL return `{ resolved: "/repo", fallback: true }` (first configured workspace)

#### Scenario: Empty configured list

- **WHEN** `resolveConfiguredWorkspace("/any/path", [])` is called
- **THEN** the function SHALL return `{ resolved: "/any/path", fallback: false }`
