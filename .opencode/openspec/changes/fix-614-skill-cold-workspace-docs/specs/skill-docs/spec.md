# Skill Docs

## ADDED Requirements

### Requirement: Cold workspace diagnostics SHALL reflect current server contracts
All distributed `nano-brain` skill references (`.opencode/skills/nano-brain/SKILL.md`, `skills/nano-brain/SKILL.md`, and their `references/http-api.md` and `references/code-intelligence.md` companions) SHALL accurately document cold-workspace diagnostics and recovery paths matching current server behavior, and MUST NOT claim that unregistered workspaces return silent empty results on HTTP/write paths or that `chunks_enqueued: 0` means a workspace cannot be fixed via API.

#### Scenario: Unregistered workspace error and registration command
- **WHEN** an agent consults `SKILL.md` for an unregistered workspace
- **THEN** the docs state that unregistered workspaces return explicit errors (`workspace_not_found` HTTP 404 / `workspace_not_registered` HTTP 400 or MCP `isError` `"workspace %q is not registered; use POST /api/v1/init to register it first"`, plus `registered: false` from `memory_workspaces_resolve`) and direct recovery to `POST /api/v1/init` or `nano-brain init --root <path>` (or `covered_by.use` when an ancestor workspace covers the path)

#### Scenario: Freshly registered workspace and asynchronous incremental reindex
- **WHEN** an agent consults `SKILL.md` or `references/http-api.md` after `POST /api/v1/init` or incremental `POST /api/v1/reindex`
- **THEN** the docs explain that `init` registers default collections (`code`, `memory`, `sessions`) and attaches the async watcher, that incremental `reindex` returns `files_pending_ingest` and `watcher_triggered: true` with `embedded: 0` / `chunks_enqueued: 0` immediately because watcher ingestion is asynchronous, and that progress is verified via `GET /api/v1/stats?workspace=<hash>` and `memory_status` (`queue_pending`)

#### Scenario: Stale symbols, graph edges, and flowcharts after upgrade
- **WHEN** an agent consults `SKILL.md` for stale symbols, graph edges, or flowcharts on already-indexed files
- **THEN** the docs direct calling `POST /api/v1/update` (`{"workspace":"<hash>"}`) to re-extract symbols, graph edges, and flowcharts asynchronously without wiping `memory`/`sessions` or re-embedding chunks

#### Scenario: Empty impact array disambiguation via node_found
- **WHEN** an agent consults `SKILL.md`, `references/http-api.md`, or `references/code-intelligence.md` for `memory_impact` / `POST /api/v1/graph/impact` returning `impacted: []`
- **THEN** the docs state that `impacted` is always `[]` (never `null`) and `node_found: true|false` distinguishes a known leaf node with zero callers (`node_found: true`) from an unknown or unindexed node (`node_found: false`)
