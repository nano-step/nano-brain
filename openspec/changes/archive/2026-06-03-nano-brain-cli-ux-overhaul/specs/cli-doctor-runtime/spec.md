## ADDED Requirements

### Requirement: `nano-brain doctor` SHALL include an offline "binary exists" check

The existing offline doctor checks SHALL be extended with a check that verifies the resolved nano-brain binary file exists on disk and is executable. This catches the silent-failure footgun where `npm/postinstall.js` exits 0 even when the binary download failed.

#### Scenario: Binary present and executable
- **WHEN** the user runs `nano-brain doctor` and the resolved binary at the npm-bin install path (or `NANO_BRAIN_BIN` override) is a regular file with at least one executable bit set
- **THEN** the report includes a row `Binary exists: <path> OK` and the overall exit code is unaffected by this check

#### Scenario: Binary missing
- **WHEN** the user runs `nano-brain doctor` and the resolved binary path does not exist (`os.Stat` returns `ErrNotExist`)
- **THEN** the report includes a row `Binary exists: <path> FAIL` with hint `Reinstall: npm install -g @nano-step/nano-brain@latest` and the overall exit code is non-zero

#### Scenario: Binary present but not executable
- **WHEN** the user runs `nano-brain doctor` and the resolved binary exists but has no executable bit (`mode & 0111 == 0`)
- **THEN** the report includes a row `Binary exists: <path> FAIL` with hint `chmod +x <path>` and the overall exit code is non-zero

### Requirement: `nano-brain doctor --online` SHALL perform runtime health checks against a running server

A new `--online` flag SHALL add runtime checks beyond the existing offline prerequisite checks. Without the flag, behaviour MUST remain unchanged. With the flag, the doctor SHALL connect to the resolved nano-brain server URL (via `getBaseURL()`) and report on server reachability, embed queue saturation, CLI ↔ server version skew, and MCP endpoint reachability.

#### Scenario: Server reachable, queue nominal, versions match, MCP responding
- **WHEN** the user runs `nano-brain doctor --online` and the server's `/api/status` returns HTTP 200 with `queue_pending / queue_capacity < 0.80` AND `version` matches the CLI's compile-time `Version` AND `/mcp` returns HTTP 200
- **THEN** the report includes rows `Server reachable: OK`, `Queue health: OK (N/M)`, `Version skew: OK (vX.Y.Z)`, `MCP endpoint: OK` and overall exit code is 0

#### Scenario: Server unreachable
- **WHEN** the user runs `nano-brain doctor --online` and the GET to `/api/status` fails with a connection error within 3 seconds
- **THEN** the report includes `Server reachable: <host:port> FAIL` with hint `Is the server running? Try: nano-brain serve -d` and subsequent online checks are reported as SKIP and the overall exit code is non-zero

#### Scenario: Queue saturation WARN at 80 %
- **WHEN** the user runs `nano-brain doctor --online` and the server returns `queue_pending = 8000` with `queue_capacity = 10000` (ratio 0.80)
- **THEN** the report includes `Queue health: 8000/10000 WARN` with hint `Embed queue is saturated; investigate slow embedding provider or backlog` and the overall exit code is non-zero

#### Scenario: Queue saturation FAIL at 95 %
- **WHEN** the user runs `nano-brain doctor --online` and the server returns `queue_pending = 9500` with `queue_capacity = 10000` (ratio 0.95)
- **THEN** the report includes `Queue health: 9500/10000 FAIL` with hint `Queue at critical capacity; new chunks are being dropped` and the overall exit code is non-zero

#### Scenario: CLI ↔ server version skew
- **WHEN** the user runs `nano-brain doctor --online` and the server returns `version = "v2026.6.5.1"` while the CLI's compile-time `Version` is `"v2026.5.1.1"`
- **THEN** the report includes `Version skew: cli=v2026.5.1.1 server=v2026.6.5.1 WARN` with hint `Reinstall CLI to match server: npm install -g @nano-step/nano-brain@latest` and overall exit code is non-zero

#### Scenario: MCP endpoint unreachable
- **WHEN** the user runs `nano-brain doctor --online` and the server responds on `/api/status` but the GET to `/mcp` fails with HTTP 404 or connection error
- **THEN** the report includes `MCP endpoint: <url> FAIL` with hint `MCP transport not wired; check server logs for /mcp route` and overall exit code is non-zero

### Requirement: `nano-brain doctor --online --json` SHALL emit machine-readable output

When both flags are present, the doctor SHALL emit a single JSON document on stdout containing all check results, suitable for CI consumption. The schema MUST be backward-compatible with the existing `nano-brain doctor --json` output (existing fields unchanged, new fields additive).

#### Scenario: JSON shape
- **WHEN** the user runs `nano-brain doctor --online --json`
- **THEN** stdout contains a JSON object with key `checks` (array of `{name, status, detail, hint}` objects) and key `all_passed` (boolean), AND online-mode checks appear AFTER offline-mode checks in the array, AND no human-readable headers or footers are emitted

#### Scenario: JSON exit code matches text exit code
- **WHEN** the user runs `nano-brain doctor --online --json` AND any check fails
- **THEN** stdout contains `"all_passed": false` AND the process exit code is non-zero
