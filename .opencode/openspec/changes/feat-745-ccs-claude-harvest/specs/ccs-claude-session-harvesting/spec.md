## ADDED Requirements

### Requirement: CCS projects roots are auto-detected deterministically

When Claude Code harvesting is enabled and no explicit Claude session directory is configured, nano-brain SHALL derive Claude projects-parent patterns from platform Claude locations and known CCS roots. CCS root derivation SHALL treat `CCS_DIR` as the root, `$CCS_HOME/.ccs` as the legacy root, and `<user home>/.ccs` as the default root. Candidate patterns SHALL be deterministic, deduplicated, and scoped to `instances/*/projects`.

#### Scenario: Default CCS root
- **WHEN** `<home>/.ccs/instances/profile-a/projects` exists and no CCS env vars are set
- **THEN** auto-detection SHALL include the `<home>/.ccs/instances/*/projects` pattern.

#### Scenario: CCS_DIR root
- **WHEN** `CCS_DIR` names a custom root containing `instances/profile-a/projects`
- **THEN** auto-detection SHALL include `<custom root>/instances/*/projects` without requiring `~/.ccs` to exist.

#### Scenario: CCS_HOME legacy root
- **WHEN** `CCS_HOME` names a base directory
- **THEN** auto-detection SHALL use `<CCS_HOME>/.ccs/instances/*/projects`, not `<CCS_HOME>/instances/*/projects`.

#### Scenario: Multiple historical roots
- **WHEN** more than one candidate CCS root exists
- **THEN** auto-detection SHALL scan their union in deterministic precedence order and deduplicate equivalent entries.

#### Scenario: Invocation-scoped config dir
- **WHEN** a CCS profile was launched with a custom `--config-dir` not represented by env vars
- **THEN** nano-brain SHALL NOT attempt to infer it, and documentation SHALL require an explicit `session_dirs` entry.

### Requirement: Explicit session directory configuration remains restrictive and compatible

The existing `session_dir` and `session_dirs` keys SHALL retain their union and backward compatibility semantics. A non-empty explicit configuration SHALL act as the complete source list and disable startup auto-detection.

#### Scenario: Legacy singular directory
- **WHEN** only `session_dir` is configured
- **THEN** it SHALL continue to be harvested as before and SHALL be ordered before `session_dirs` entries.

#### Scenario: Glob list
- **WHEN** `session_dirs` contains a glob matching multiple CCS instance projects dirs
- **THEN** every matching projects parent SHALL be eligible for harvesting with deterministic deduplication.

#### Scenario: Manual custom root
- **WHEN** a user configures a custom CCS `--config-dir` projects glob in `session_dirs`
- **THEN** nano-brain SHALL harvest matching sessions from that root without requiring CCS env vars.

### Requirement: Claude harvest discovery is re-evaluated on each harvest cycle

The Claude harvester SHALL re-expand configured projects-parent patterns and re-read registered workspaces on every harvest cycle. Discovery SHALL NOT be limited to leaf directories that existed when the daemon started.

#### Scenario: First session after startup
- **GIVEN** a CCS projects pattern is configured and a workspace is registered
- **WHEN** the first matching session leaf is created after daemon startup and a harvest cycle runs
- **THEN** the session SHALL be discovered without restarting the daemon.

#### Scenario: Workspace registered after startup
- **GIVEN** the daemon started before a workspace was registered
- **WHEN** that workspace is registered and a matching CCS session exists
- **THEN** the next manual or scheduled harvest SHALL discover it without restarting the daemon.

#### Scenario: New CCS profile after startup
- **GIVEN** an instance glob was loaded at startup
- **WHEN** a new matching profile projects dir appears later
- **THEN** the next harvest cycle SHALL consider it without a daemon restart.

#### Scenario: Default storage root created after startup
- **GIVEN** zero-config Claude harvesting was enabled at startup
- **WHEN** the standard Claude projects dir or default `~/.ccs` instance root is first created later
- **THEN** the next harvest cycle SHALL consider the retained standard pattern without a daemon restart.

#### Scenario: Config edit still requires restart
- **WHEN** the YAML values of `session_dir` or `session_dirs` are changed while the daemon is running
- **THEN** the running daemon SHALL continue using the startup-loaded patterns until restart.

### Requirement: Transcript cwd guards workspace attribution

When a transcript contains an explicit `cwd`, the Claude harvester SHALL compare it with the registered workspace path before persistence and SHALL skip mismatches. Transcripts without `cwd` SHALL preserve legacy behavior.

#### Scenario: Matching cwd
- **WHEN** a discovered transcript's `cwd` matches the registered workspace path after path normalization
- **THEN** it SHALL be eligible for ingestion into that workspace.

#### Scenario: Mismatched cwd
- **WHEN** an encoded leaf collision causes a transcript whose `cwd` belongs to a different path to be scanned for a workspace
- **THEN** it SHALL NOT be ingested into that workspace.

#### Scenario: Missing cwd
- **WHEN** an older transcript has no `cwd`
- **THEN** the harvester SHALL retain the pre-existing encoded-path behavior rather than dropping the transcript solely for missing metadata.

### Requirement: Init enables harvesting without persisting stale profile paths

During a fresh config render, `nano-brain init` SHALL enable Claude Code harvesting with zero additional questions when detectable Claude or CCS storage exists. It SHALL leave `session_dir` and `session_dirs` empty so daemon auto-detection remains active, and it SHALL print a concise notice.

#### Scenario: Detectable CCS storage
- **WHEN** init renders a fresh config on a machine with CCS instance storage
- **THEN** the rendered config SHALL contain `harvester.claudecode.enabled: true` and no concrete per-profile CCS paths.

#### Scenario: No detectable storage
- **WHEN** init renders a fresh config with no Claude or CCS storage indicators
- **THEN** Claude harvesting SHALL remain disabled by default.

#### Scenario: Existing config kept
- **WHEN** interactive init is re-run and the user chooses to keep an existing config
- **THEN** nano-brain SHALL NOT rewrite harvester settings implicitly.

### Requirement: CCS setup behavior is documented accurately

User-facing documentation SHALL describe the CCS layout, glob-based `session_dirs` configuration, custom-root behavior, auto-detection replacement semantics, restart/reload boundary, privacy implications, and project-local `.mcp.json` Claude MCP wiring.

#### Scenario: CCS MCP wiring
- **WHEN** a Claude Code user runs under CCS profile isolation
- **THEN** documentation SHALL direct them to project-local `.mcp.json` rather than relying on user-scope `~/.claude.json` propagation.

#### Scenario: Explicit custom root documentation
- **WHEN** a user reads the configuration reference
- **THEN** it SHALL show a complete explicit `session_dirs` example including standard Claude and CCS projects-parent patterns.

## MODIFIED Requirements

### Requirement: Claude harvester lifecycle no longer depends on startup leaf existence

Claude harvester registration SHALL represent projects-root discovery capability rather than a fixed list of leaf directories present at startup.

#### Scenario: Empty roots at startup
- **WHEN** configured glob patterns currently match no leaf session dirs
- **THEN** the harvester SHALL remain registered and able to discover future matches while the daemon continues running.
