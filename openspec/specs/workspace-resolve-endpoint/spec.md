# workspace-resolve-endpoint Specification

## Purpose
TBD - created by archiving change add-workspace-resolve-endpoint. Update Purpose after archive.
## Requirements
### Requirement: Workspace path-to-hash resolution endpoint
The server SHALL expose `POST /api/v1/workspaces/resolve` that accepts a filesystem path and returns the deterministic workspace hash plus registration status. The endpoint SHALL be public (no `workspaceMiddleware`) and read-only (no DB writes, no side effects).

#### Scenario: Resolve registered workspace
- **WHEN** `POST /api/v1/workspaces/resolve` is called with body `{"path": "/abs/path/to/registered/project"}`
- **AND** the workspace at that path is already registered via `POST /api/v1/init`
- **THEN** the response is HTTP 200 with `Content-Type: application/json`
- **AND** the body is `{"workspace_hash": "<64-char-hex>", "root_path": "/abs/path/to/registered/project", "name": "<workspaces.name>", "registered": true}`
- **AND** `workspace_hash` equals `sha256(absolute_path)` rendered as lowercase hex

#### Scenario: Resolve unregistered path
- **WHEN** `POST /api/v1/workspaces/resolve` is called with body `{"path": "/abs/path/never/registered"}`
- **THEN** the response is HTTP 200 with the same shape
- **AND** `workspace_hash` is computed from `filepath.Abs(path)` via `storage.WorkspaceHash`
- **AND** `name` is `filepath.Base(absolute_path)` (no DB lookup needed)
- **AND** `registered` is `false`
- **AND** no row is inserted into the `workspaces` table

#### Scenario: Resolve relative path
- **WHEN** the request body contains a relative path (e.g., `{"path": "."}` or `{"path": "../foo"}`)
- **THEN** the server normalizes it via `filepath.Abs` (relative to the SERVER'S working directory, not the client's)
- **AND** the response `root_path` field contains the absolute path

> Note: Clients SHOULD send absolute paths since the server's CWD is not the client's CWD. The CLI subcommand handles this by calling `os.Getwd()` client-side before posting.

#### Scenario: Missing path field
- **WHEN** `POST /api/v1/workspaces/resolve` is called with body `{}` or `{"path": ""}`
- **THEN** the response is HTTP 400 with error message indicating `path is required`

#### Scenario: Invalid JSON body
- **WHEN** the request body is malformed JSON
- **THEN** the response is HTTP 400 with error message indicating invalid request body

#### Scenario: Read-only — no DB mutation
- **WHEN** `POST /api/v1/workspaces/resolve` is called any number of times with any path (registered or not)
- **THEN** the count of rows in `workspaces`, `collections`, `documents`, and `chunks` tables is unchanged

### Requirement: CLI `workspaces current` subcommand
The CLI SHALL expose `nano-brain workspaces current` which resolves the current shell's working directory (or `--path=<p>`) to a workspace hash by calling `POST /api/v1/workspaces/resolve`.

#### Scenario: Default — bare hash to stdout
- **WHEN** `nano-brain workspaces current` is invoked in a registered project directory
- **THEN** stdout contains exactly the 64-character workspace hash followed by a newline
- **AND** stderr is empty
- **AND** the exit code is `0`

#### Scenario: `--export` flag
- **WHEN** `nano-brain workspaces current --export` is invoked
- **THEN** stdout contains exactly `export NANO_BRAIN_WORKSPACE=<hash>\n`
- **AND** the exit code is `0`
- **AND** the line is suitable for `eval "$(nano-brain workspaces current --export)"`

#### Scenario: `--json` flag
- **WHEN** `nano-brain workspaces current --json` is invoked
- **THEN** stdout contains the full server JSON response with newline
- **AND** the JSON parses to an object with keys `workspace_hash`, `root_path`, `name`, `registered`

#### Scenario: `--check` flag with registered workspace
- **WHEN** `nano-brain workspaces current --check` is invoked in a registered project
- **THEN** stdout contains the bare hash
- **AND** the exit code is `0`

#### Scenario: `--check` flag with unregistered path
- **WHEN** `nano-brain workspaces current --check` is invoked in a directory NOT registered
- **THEN** the exit code is `2` (distinct from 1=server error and 0=success)
- **AND** stderr contains a message indicating the workspace is not registered

#### Scenario: `--path` flag overrides CWD
- **WHEN** `nano-brain workspaces current --path=/some/other/path` is invoked
- **THEN** the server is called with that path (not `os.Getwd()`)
- **AND** the response reflects that path's hash and registration status

#### Scenario: Server unreachable
- **WHEN** the nano-brain server is not running
- **THEN** the exit code is `1`
- **AND** stderr contains a connection error message

#### Scenario: Combined flags
- **WHEN** `nano-brain workspaces current --export --check` is invoked in a registered project
- **THEN** stdout contains the `export NANO_BRAIN_WORKSPACE=<hash>` line
- **AND** the exit code is `0`

### Requirement: MCP tool `memory_workspaces_resolve`
The MCP server SHALL expose a tool named `memory_workspaces_resolve` with the same semantic surface as the HTTP endpoint.

#### Scenario: MCP resolve tool
- **WHEN** an MCP client calls tool `memory_workspaces_resolve` with arguments `{"path": "/abs/path"}`
- **THEN** the response contains a JSON object with keys `workspace_hash`, `root_path`, `name`, `registered`
- **AND** the values match the corresponding HTTP endpoint response for the same path

#### Scenario: MCP resolve with empty path
- **WHEN** an MCP client calls `memory_workspaces_resolve` with `{"path": ""}` or missing `path`
- **THEN** the tool returns an error indicating `path is required`

### Requirement: Skill documentation phase-based structure
The agent-facing documentation in `skills/nano-brain/SKILL.md` SHALL follow a 4-phase structure: DISCOVER → SELECT → EXECUTE → RECOVER. The bootstrap section SHALL document the one-liner `eval "$(npx nano-brain workspaces current --export)"`.

#### Scenario: SKILL.md contains all four phases
- **WHEN** an agent reads `skills/nano-brain/SKILL.md`
- **THEN** the file contains headers matching `## Phase 1 — DISCOVER`, `## Phase 2 — SELECT`, `## Phase 3 — EXECUTE`, `## Phase 4 — RECOVER`
- **AND** the DISCOVER section contains the bootstrap one-liner using `nano-brain workspaces current --export`
- **AND** the RECOVER section contains an error table with at least the entries: `workspace_required`, `workspace_not_registered`, `connection refused`

#### Scenario: AGENTS_SNIPPET.md bootstrap example
- **WHEN** an agent reads `skills/nano-brain/AGENTS_SNIPPET.md`
- **THEN** the file contains the literal bootstrap line `eval "$(npx nano-brain workspaces current --export)"`
- **AND** all curl examples use endpoint path prefix `/api/v1/` (not `/api/`)
- **AND** all curl POST examples include a `workspace` field in their request body OR reference `$NANO_BRAIN_WORKSPACE`

#### Scenario: Project AGENTS.md sync
- **WHEN** an agent reads the `<!-- OPENCODE-MEMORY:START -->` block in the project root `AGENTS.md`
- **THEN** the block matches the content of `skills/nano-brain/AGENTS_SNIPPET.md`
- **AND** no `curl` example uses the wrong path `/api/query`
- **AND** every API example shows the workspace handling

