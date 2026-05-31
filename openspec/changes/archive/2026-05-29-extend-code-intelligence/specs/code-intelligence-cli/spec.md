# Code Intelligence CLI Commands

## ADDED Requirements

### Requirement: context command
The system SHALL provide a `nano-brain context <symbol>` CLI command that shows a 360° view of a symbol — its definition, callers, callees, containing file, and imports.

#### Scenario: Symbol context lookup
- **WHEN** running `nano-brain context processFile --workspace <hash>`
- **THEN** the output SHALL show the symbol's file, kind, language, outgoing calls, and incoming callers

#### Scenario: Symbol not found
- **WHEN** running `nano-brain context nonExistentSymbol --workspace <hash>`
- **THEN** the command SHALL print "symbol not found" and exit with code 1

#### Scenario: JSON output
- **WHEN** running `nano-brain context processFile --json`
- **THEN** the output SHALL be valid JSON with keys: symbol, file, kind, language, calls_out, called_by, imports

### Requirement: code-impact command
The system SHALL provide a `nano-brain code-impact <symbol>` CLI command that shows the blast radius of changing a symbol via BFS traversal of reverse dependencies. The command SHALL call `POST /api/v1/graph/impact`.

#### Scenario: Impact analysis with default depth
- **WHEN** running `nano-brain code-impact processFile --workspace <hash>`
- **THEN** the output SHALL show symbols that directly or transitively depend on processFile (default depth=2)

#### Scenario: Custom depth
- **WHEN** running `nano-brain code-impact processFile --depth 3`
- **THEN** the BFS traversal SHALL extend to 3 levels of reverse dependencies (max_depth clamped to [1,3] by server)

#### Scenario: No impact
- **WHEN** running `nano-brain code-impact` on a leaf symbol with no dependents
- **THEN** the output SHALL indicate "no upstream dependents found"

### Requirement: detect-changes command
The system SHALL provide a `nano-brain detect-changes` CLI command that maps git diff output to affected symbols and their impact radius. The command SHALL use `exec.CommandContext` with a 10s timeout to invoke `git diff`. `git` MUST be in PATH and CWD MUST be inside a git repo.

#### Scenario: Staged changes detection
- **WHEN** running `nano-brain detect-changes --staged --workspace <hash>`
- **THEN** the output SHALL list changed files, affected symbols at changed lines, and impact radius per symbol

#### Scenario: All changes detection
- **WHEN** running `nano-brain detect-changes --all --workspace <hash>`
- **THEN** the output SHALL include both staged and unstaged changes

#### Scenario: No changes
- **WHEN** running `nano-brain detect-changes` with a clean working tree
- **THEN** the output SHALL indicate "no changes detected"

#### Scenario: JSON output
- **WHEN** running `nano-brain detect-changes --json`
- **THEN** the output SHALL be valid JSON with keys: changed_files, affected_symbols, impact_radius, details

#### Scenario: Git not available
- **WHEN** running `nano-brain detect-changes` and `git` is not in PATH
- **THEN** the command SHALL print "git not found in PATH" and exit with code 1

### Requirement: CLI commands use existing API
All CLI commands SHALL call the existing REST API endpoints (`/api/v1/symbols`, `/api/v1/graph/query`, `/api/v1/graph/impact`). No new HTTP handler logic SHALL be added.

#### Scenario: API reuse
- **WHEN** `nano-brain context` is invoked
- **THEN** it SHALL make HTTP requests to `/api/v1/symbols` and `/api/v1/graph/query` on the configured server
