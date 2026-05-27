# harvester-per-project-scoping Specification

## Purpose
TBD - created by archiving change fix-harvester-per-project-workspace. Update Purpose after archive.
## Requirements
### Requirement: Session-to-workspace mapping via project.worktree JOIN

The OpenCode SQLite harvester SHALL derive workspace hash per-session from `project.worktree` via LEFT JOIN on `session.project_id`. It SHALL auto-register unknown workspaces via `UpsertWorkspace` and SHALL handle orphaned sessions with a fallback hash + WARN log.

#### Scenario: Session mapped to per-project workspace

- **WHEN** `HarvestAll()` processes a session with a valid `project.worktree`
- **THEN** the session is stored under `WorkspaceHash(project.worktree)`, not `WorkspaceHash(dbPath)`

---

