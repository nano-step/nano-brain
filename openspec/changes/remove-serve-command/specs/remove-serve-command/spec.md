## REMOVED Requirements

### Requirement: serve command CLI entry point
The CLI SHALL NOT expose a `serve` command. The `case 'serve'` branch in the command dispatch switch and the `serve` entry in `showHelp()` output SHALL be removed.

**Reason**: The `serve` command is redundant — the server now runs inside a Docker container managed by `docker start/stop`.
**Migration**: Users SHALL use `docker start nano-brain` to start the server and `docker stop nano-brain` to stop it.

#### Scenario: User runs serve command
- **WHEN** a user executes `npx nano-brain serve` (or any subcommand like `serve start`, `serve stop`, `serve install`)
- **THEN** the CLI SHALL exit with an unknown-command error (no special handling)

#### Scenario: Help text does not list serve
- **WHEN** a user executes `npx nano-brain --help`
- **THEN** the output SHALL NOT contain the word `serve` as a command option

### Requirement: handleServe function removal
The `handleServe()` function (lines 434–691 in `src/index.ts`) and all associated PID-file management logic SHALL be deleted entirely.

**Reason**: All serve subcommands (start, stop, status, install, uninstall, --foreground) are superseded by Docker container management.
**Migration**: No code migration needed — the function is only called from the `serve` dispatch case which is also removed.

#### Scenario: No serve-related code in index.ts
- **WHEN** the codebase is searched for `handleServe`
- **THEN** zero matches SHALL be found

### Requirement: service-installer module removal
The file `src/service-installer.ts` SHALL be deleted entirely. The import `import { installService, uninstallService } from './service-installer.js'` in `src/index.ts` SHALL be removed.

**Reason**: The service-installer generates launchd plists and systemd unit files, which are no longer needed since the server runs in Docker.
**Migration**: No migration needed — the module is only used by `handleServe()` which is also removed.

#### Scenario: service-installer.ts does not exist
- **WHEN** the file system is checked for `src/service-installer.ts`
- **THEN** the file SHALL NOT exist

#### Scenario: No imports of service-installer
- **WHEN** the codebase is searched for `service-installer`
- **THEN** zero import statements SHALL match

## ADDED Requirements

### Requirement: Docker-based error messages for unreachable server
When the CLI detects that the nano-brain server is not reachable (in contexts where it previously said "Daemon not running"), it SHALL display the following error message:
```
Error: nano-brain server not reachable. Ensure the Docker container is running:
  docker start nano-brain
```
This applies to all 4 locations in `src/index.ts` (lines ~1414, ~1558, ~1725, ~2322) where the old "Daemon not running. Start it on the host: npx nano-brain serve install && launchctl load ..." message appeared.

#### Scenario: Server unreachable during query command
- **WHEN** a user runs a CLI command (e.g., `query`, `search`, `vsearch`, `write`) from inside a container and the server is not reachable
- **THEN** the CLI SHALL print `Error: nano-brain server not reachable. Ensure the Docker container is running:\n  docker start nano-brain` and exit with code 1

#### Scenario: Error message does not reference serve or launchctl
- **WHEN** any error message about server connectivity is displayed
- **THEN** the message SHALL NOT contain `serve install`, `launchctl`, or `LaunchAgents`

### Requirement: NANO_BRAIN_HOST environment variable overrides container detection
The `getHttpHost()` function SHALL check the `NANO_BRAIN_HOST` environment variable first. If set, it SHALL return the value of `NANO_BRAIN_HOST` directly, bypassing container detection and the `host.docker.internal` / `localhost` fallback logic.

**Fallback chain:** `NANO_BRAIN_HOST` env var → container detection (`host.docker.internal`) → `localhost`

#### Scenario: NANO_BRAIN_HOST is set
- **GIVEN** the environment variable `NANO_BRAIN_HOST` is set to `"10.0.0.5"`
- **WHEN** `getHttpHost()` is called
- **THEN** it SHALL return `"10.0.0.5"` regardless of whether the process is running inside a container

#### Scenario: NANO_BRAIN_HOST is not set, running in container
- **GIVEN** the environment variable `NANO_BRAIN_HOST` is not set
- **AND** the process is running inside a container
- **WHEN** `getHttpHost()` is called
- **THEN** it SHALL return `"host.docker.internal"` (existing behavior preserved)

#### Scenario: NANO_BRAIN_HOST is not set, running on host
- **GIVEN** the environment variable `NANO_BRAIN_HOST` is not set
- **AND** the process is NOT running inside a container
- **WHEN** `getHttpHost()` is called
- **THEN** it SHALL return `"localhost"` (existing behavior preserved)

### Requirement: NANO_BRAIN_PORT environment variable overrides default port
A `getHttpPort()` function (or equivalent logic) SHALL check the `NANO_BRAIN_PORT` environment variable first. If set, it SHALL parse the value as an integer and use it as the server port, overriding the default `DEFAULT_HTTP_PORT` (3100).

**Fallback chain:** `NANO_BRAIN_PORT` env var → `DEFAULT_HTTP_PORT` (3100)

#### Scenario: NANO_BRAIN_PORT is set
- **GIVEN** the environment variable `NANO_BRAIN_PORT` is set to `"4200"`
- **WHEN** the HTTP port is resolved
- **THEN** the port SHALL be `4200`

#### Scenario: NANO_BRAIN_PORT is not set
- **GIVEN** the environment variable `NANO_BRAIN_PORT` is not set
- **WHEN** the HTTP port is resolved
- **THEN** the port SHALL be `3100` (existing default preserved)

### Requirement: Server detection uses configurable host and port
The `detectRunningServer()` and `detectRunningServerContainer()` functions SHALL use the resolved host (from `getHttpHost()`) and port (from `getHttpPort()`) when checking server connectivity. Error messages SHALL display the configured `host:port` so users can diagnose misconfiguration.

#### Scenario: Error message shows configured address
- **GIVEN** `NANO_BRAIN_HOST` is set to `"10.0.0.5"` and `NANO_BRAIN_PORT` is set to `"4200"`
- **WHEN** the server is not reachable
- **THEN** the error message SHALL include `10.0.0.5:4200` in the output

### Requirement: mcp command preserved
The `mcp` command and `src/server.ts` SHALL remain completely unchanged. Docker uses `npx tsx src/index.ts mcp --http --port=3100 --host=0.0.0.0` as its entry point and this MUST continue to work.

#### Scenario: mcp command still works
- **WHEN** `npx nano-brain mcp --http --port=3100` is executed
- **THEN** the MCP server SHALL start and listen on the specified port

### Requirement: Test cleanup
All serve-related test cases in `test/cli.test.ts` SHALL be removed or updated. No test SHALL reference `serve`, `handleServe`, `service-installer`, or the old "Daemon not running" error message text.

#### Scenario: No serve references in tests
- **WHEN** the test file `test/cli.test.ts` is searched for `serve`
- **THEN** zero matches SHALL be found (excluding unrelated uses of the word in other contexts)

#### Scenario: Tests pass after removal
- **WHEN** the test suite is executed
- **THEN** all tests SHALL pass without errors related to missing `serve` command or `service-installer` module
