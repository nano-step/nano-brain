## ADDED Requirements

### Requirement: Discovery of per-project OpenCode SQLite databases under `db_root`

When `harvester.opencode.db_root` is non-empty (set via config, env var, or auto-detection), the daemon SHALL scan that directory at startup for per-project OpenCode SQLite databases matching the glob `<db_root>/*/opencode.db`. For each candidate, the daemon SHALL open the SQLite file read-only (`?mode=ro`), query `SELECT id, worktree FROM project LIMIT 1`, and treat the returned `worktree` value as the project's absolute path.

#### Scenario: Multiple per-project DBs under root, all registered

- **GIVEN** `db_root` is `/Users/u/.ai-sandbox/opencode-dbs`
- **AND** that directory contains `proj-a-xxx/opencode.db` (worktree `/u/proj-a`), `proj-b-yyy/opencode.db` (worktree `/u/proj-b`)
- **AND** both `/u/proj-a` and `/u/proj-b` are registered nano-brain workspaces
- **WHEN** the daemon starts
- **THEN** exactly two `OpenCodeSQLiteHarvester` instances are registered with the `Runner`, one per DB
- **AND** each instance's `dbPath` points at its respective `opencode.db`

#### Scenario: Some per-project DBs match, some do not

- **GIVEN** three candidate DBs with worktrees `/u/proj-a`, `/u/proj-b`, `/u/unknown`
- **AND** only `/u/proj-a` is a registered nano-brain workspace
- **WHEN** the daemon starts
- **THEN** exactly one harvester (for `/u/proj-a`) is registered
- **AND** unmatched DBs are logged at Debug level with a `reason="worktree_not_registered"` field
- **AND** no error or warning is emitted for unmatched DBs

#### Scenario: DB with empty or root worktree skipped

- **GIVEN** a candidate DB whose `project.worktree` value is `""` or `"/"`
- **WHEN** the daemon scans it during discovery
- **THEN** the DB is excluded from harvester registration
- **AND** a Debug log is emitted with `reason="global_or_empty_worktree"`

#### Scenario: Unreadable or corrupt candidate DB

- **GIVEN** a candidate path under `db_root` that is unreadable, contains no `project` table, or fails `SELECT worktree FROM project`
- **WHEN** the daemon scans it
- **THEN** the candidate is skipped
- **AND** a Debug log is emitted with `reason` and `error` fields
- **AND** daemon startup continues without aborting

#### Scenario: db_root directory exists but is empty

- **GIVEN** `db_root` resolves to a directory containing zero subdirectories with an `opencode.db` file
- **WHEN** discovery runs
- **THEN** `filepath.Glob` returns zero candidates
- **AND** no harvesters are instantiated from `db_root` mode
- **AND** the daemon falls through to `db_path` mode (if set) or `session_dir` mode (if set) or disabled
- **AND** if `db_root` was explicitly configured (config/env), a Warn log is emitted; if auto-detected, an Info log is emitted

#### Scenario: Same worktree appears in multiple DBs under db_root

- **GIVEN** two distinct DB files `<db_root>/foo-aaa/opencode.db` and `<db_root>/foo-bbb/opencode.db` both have `project.worktree = /u/foo`
- **AND** `/u/foo` is a registered nano-brain workspace
- **WHEN** discovery runs
- **THEN** two `OpenCodeSQLiteHarvester` instances are registered, one per DB file
- **AND** content-hash deduplication at the document upsert layer prevents duplicate session-summary documents in PG
- **AND** an Info log is emitted noting the duplicate worktree mapping

#### Scenario: Per-project DB contains multiple project rows

- **GIVEN** a per-project SQLite contains more than one row in the `project` table (corruption case — per-project DBs are designed to have exactly one project row)
- **WHEN** discovery runs `SELECT id, worktree FROM project LIMIT 1`
- **THEN** one project row is selected arbitrarily (SQLite's natural order)
- **AND** discovery proceeds with that single worktree; other project rows are ignored
- **AND** no error is emitted (graceful handling of corruption)

### Requirement: Three-mode priority chain for OpenCode harvester source

The daemon SHALL select exactly one OpenCode harvest source mode per startup, evaluating in priority order: (1) `db_root` if it produces ≥1 matched harvester; (2) `db_path` if non-empty; (3) `session_dir` if non-empty; (4) disabled. Modes after the selected one SHALL NOT be evaluated or instantiated.

#### Scenario: All three modes configured, `db_root` wins

- **GIVEN** config has `db_root=/x`, `db_path=/y/opencode.db`, `session_dir=/z`
- **AND** `/x` contains at least one matched per-project DB
- **WHEN** the daemon starts
- **THEN** only the `db_root` discovery harvesters are registered
- **AND** no harvester is created from `/y/opencode.db` or `/z`

#### Scenario: `db_root` set but produces zero matches — fall through

- **GIVEN** config has `db_root=/x` (no matched DBs) and `db_path=/y/opencode.db`
- **WHEN** the daemon starts
- **THEN** the daemon falls through to `db_path` mode
- **AND** an Info log is emitted noting `db_root` produced zero matches before fall-through

#### Scenario: Nothing configured, all auto-detect probes fail

- **WHEN** the daemon starts with no `db_root`, `db_path`, or `session_dir` set
- **AND** every auto-detect probe returns empty (no env vars, no platform default paths exist)
- **THEN** no OpenCode harvester is registered
- **AND** an Info log: `"opencode harvester disabled (no db_root, db_path, or session_dir configured)"`

### Requirement: Configuration field `harvester.opencode.db_root`

The configuration schema SHALL include `harvester.opencode.db_root` as an optional string field. It SHALL be settable via YAML config, the `NANO_BRAIN_HARVESTER_OPENCODE_DB_ROOT` env var, and the alias `OPENCODE_DB_ROOT`. The default value SHALL be empty string; auto-detection SHALL fill it at startup when the platform default directory exists.

#### Scenario: Setting via env var override

- **GIVEN** YAML config does not set `db_root`
- **AND** env var `OPENCODE_DB_ROOT=/custom/path` is exported
- **WHEN** the daemon starts
- **THEN** `cfg.Harvester.OpenCode.DBRoot` equals `/custom/path`
- **AND** discovery scans `/custom/path/*/opencode.db`

### Requirement: Auto-detection of platform default `db_root`

When `harvester.opencode.db_root` is unset after config + env var resolution, the daemon SHALL probe the platform default location `$HOME/.ai-sandbox/opencode-dbs` on macOS and Linux. If that path exists and is a directory, it SHALL be used as `db_root`. On Windows the probe SHALL be a no-op (no default).

#### Scenario: Platform default exists

- **GIVEN** `$HOME/.ai-sandbox/opencode-dbs` exists and is a directory
- **AND** no `db_root` is set in config or env
- **WHEN** `detectOpenCodeDBRoot()` runs
- **THEN** it returns `$HOME/.ai-sandbox/opencode-dbs`

#### Scenario: Platform default missing

- **GIVEN** `$HOME/.ai-sandbox/opencode-dbs` does not exist
- **WHEN** `detectOpenCodeDBRoot()` runs
- **THEN** it returns empty string

### Requirement: Status endpoint exposes harvester mode and DB count

The `GET /api/status` response field `harvester_status.opencode` SHALL include the fields `mode` (one of `"db_root"`, `"db_path"`, `"session_dir"`, `"disabled"`), `db_count` (integer, count of registered OpenCode SQLite harvesters; always 0 or 1 for non-`db_root` modes), and `db_root` (string, the resolved root path when `mode == "db_root"`, else empty). Existing fields `enabled`, `session_dir`, `db_path` SHALL be retained for backward compatibility.

#### Scenario: db_root mode with two harvesters

- **WHEN** the daemon ran discovery and registered 2 per-project harvesters
- **AND** `GET /api/status` is called
- **THEN** the response contains `harvester_status.opencode.mode == "db_root"`
- **AND** `harvester_status.opencode.db_count == 2`
- **AND** `harvester_status.opencode.db_root` equals the resolved root path
- **AND** `harvester_status.opencode.enabled == true`

#### Scenario: Disabled mode

- **WHEN** no OpenCode harvester is registered
- **AND** `GET /api/status` is called
- **THEN** the response contains `harvester_status.opencode.mode == "disabled"`
- **AND** `harvester_status.opencode.db_count == 0`
- **AND** `harvester_status.opencode.enabled == false`

### Requirement: Harvester registration via `Runner.AddHarvester`

The `Runner` SHALL support 1..N OpenCode harvester instances registered via `AddHarvester(h Harvester)`. Per-tick fan-out via `RunOnce` SHALL invoke each registered harvester's `HarvestAll` exactly once and aggregate `harvested`, `skipped`, `errCount` counters across all of them. The summarizer set via `Runner.WithSummarizer` SHALL be propagated to every harvester registered after the summarizer is set, not just the first.

#### Scenario: Multiple OpenCode SQLite harvesters fan out

- **GIVEN** the `Runner` has 3 `OpenCodeSQLiteHarvester` instances registered (each on a different per-project DB)
- **WHEN** `Runner.RunOnce(ctx)` fires
- **THEN** `HarvestAll` is called on each of the 3 instances
- **AND** the aggregate counters returned reflect the sum across all 3
