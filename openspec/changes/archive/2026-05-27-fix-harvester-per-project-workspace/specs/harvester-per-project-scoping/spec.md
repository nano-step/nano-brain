# harvester-per-project-scoping Delta Specification

## ADDED Requirements

### Requirement: Session-to-workspace mapping via project.worktree JOIN

The OpenCode SQLite harvester SHALL derive workspace hash per-session from `project.worktree` via LEFT JOIN on `session.project_id`. It SHALL auto-register unknown workspaces via `UpsertWorkspace` and SHALL handle orphaned sessions with a fallback hash + WARN log.

#### Scenario: Session mapped to per-project workspace

- **WHEN** `HarvestAll()` processes a session with a valid `project.worktree`
- **THEN** the session is stored under `WorkspaceHash(project.worktree)`, not `WorkspaceHash(dbPath)`

---

## Purpose

Defines how the OpenCode SQLite harvester maps sessions to nano-brain workspaces. Each OpenCode project (identified by `project.worktree`) SHALL map to exactly one nano-brain workspace, keyed by `WorkspaceHash(project.worktree)`.

## Requirements

### Requirement: Session-to-workspace mapping via project.worktree

The OpenCode SQLite harvester SHALL determine the workspace hash for each session by reading `project.worktree` from OpenCode's `project` table (joined via `session.project_id`) and computing `WorkspaceHash(project.worktree)`. The workspace hash SHALL NOT be derived from the OpenCode DB file path.

#### Scenario: Session belongs to a known project

- **WHEN** `HarvestAll()` processes a session with `project_id = "abc"` and `project.worktree = "/Users/alice/projects/my-app"`
- **THEN** the session document is stored under `workspace_hash = SHA256("/Users/alice/projects/my-app")`
- **THEN** the hash matches what `WorkspaceHash("/Users/alice/projects/my-app")` returns

#### Scenario: Two sessions from the same project share one workspace

- **WHEN** two sessions both have `project_id = "abc"` (`worktree = "/Users/alice/projects/my-app"`)
- **THEN** both session documents are stored under the same `workspace_hash`
- **THEN** only one workspace row exists in the `workspaces` table for that worktree

#### Scenario: Sessions from different projects go to different workspaces

- **WHEN** session S1 has `project.worktree = "/Users/alice/projects/app-a"` and session S2 has `project.worktree = "/Users/alice/projects/app-b"`
- **THEN** S1 is stored under `WorkspaceHash("/Users/alice/projects/app-a")`
- **THEN** S2 is stored under `WorkspaceHash("/Users/alice/projects/app-b")`
- **THEN** querying workspace A never returns S2, and querying workspace B never returns S1

### Requirement: Workspace auto-registration on first harvest

The harvester SHALL automatically register a workspace in nano-brain PostgreSQL (via `UpsertWorkspace`) the first time it encounters a session belonging to that project, without requiring the user to run `nano-brain init --root`.

#### Scenario: First harvest for a new project

- **WHEN** `HarvestAll()` encounters a session with a `project.worktree` not yet in the `workspaces` table
- **THEN** the workspace is inserted via `UpsertWorkspace` with `hash = WorkspaceHash(worktree)` and `root = worktree`
- **THEN** the session document is successfully stored under that workspace hash

#### Scenario: Subsequent harvest for existing project

- **WHEN** `HarvestAll()` encounters a session for a project already registered
- **THEN** `UpsertWorkspace` is called (idempotent) without error
- **THEN** the existing workspace row is not duplicated

### Requirement: Graceful handling of orphaned sessions

Sessions whose `project` row has been deleted (orphaned by OpenCode internals) SHALL be harvested under a fallback workspace derived from the DB file path, with a warning log entry.

#### Scenario: Session has no matching project row

- **WHEN** a session's `project_id` does not match any row in the `project` table (LEFT JOIN returns NULL worktree)
- **THEN** the session is stored under `WorkspaceHash(dbPath)` as fallback
- **THEN** a WARN log is emitted: `"session has no project row, using fallback workspace"` with `session_id` field

### Requirement: Constructor does not accept pre-computed workspace

`NewOpenCodeSQLiteHarvester` SHALL NOT accept a `workspace string` parameter. The harvester SHALL derive workspace hash internally during `HarvestAll()`.

#### Scenario: Harvester constructed without workspace parameter

- **WHEN** `NewOpenCodeSQLiteHarvester(pgDB, logger, dbPath)` is called
- **THEN** the harvester is created successfully
- **THEN** no workspace hash is stored in the struct at construction time
