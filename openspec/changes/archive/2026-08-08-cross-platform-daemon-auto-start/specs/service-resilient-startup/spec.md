## ADDED Requirements

### Requirement: Supervisor-compatible startup and launcher resolution

The server startup path used by foreground `serve` SHALL remain in the
foreground and SHALL use a cancellation-aware PostgreSQL retry helper without
changing the fail-fast behavior of the existing single-attempt `NewPool` API.
The retry helper SHALL close each failed pool attempt, wait with exponential
backoff of 1s, 2s, 4s and so on capped at 30s, and stop promptly when the
startup context is cancelled. Invalid DSNs and configuration validation errors
SHALL fail immediately. Each retry SHALL emit structured warning context with
the attempt and next delay without logging credentials.

#### Scenario: PostgreSQL becomes available after launch
- **WHEN** `serve` starts while PostgreSQL refuses connections and PostgreSQL becomes reachable before the context is cancelled
- **THEN** startup retries with the defined capped backoff, eventually creates the server, and exposes `/health` with `ready=true` without a supervisor restart being required

#### Scenario: Startup is cancelled during backoff
- **WHEN** the foreground process receives SIGTERM while the retry helper is waiting
- **THEN** the wait exits promptly, the failed pool is closed, no retry goroutine or pool remains, and the process performs normal shutdown

#### Scenario: Invalid database configuration
- **WHEN** the configured database URL cannot be parsed or the server configuration is invalid
- **THEN** startup returns a descriptive error immediately and does not retry the permanent configuration failure

#### Scenario: One-shot pool caller remains fail-fast
- **WHEN** a migration, command, or test calls the existing `storage.NewPool` API
- **THEN** it retains single-attempt behavior and is not changed to wait indefinitely for PostgreSQL

### Requirement: The npm wrapper SHALL preserve foreground process ownership

`npm/run.js` SHALL continue to honor a valid `NANO_BRAIN_BIN` override and its
existing missing-binary fallback, but when it launches the Go child it SHALL
wait for that child, forward SIGTERM and SIGINT, and exit with the child's exit
status or signal. For service installation, it SHALL provide absolute `run.js`
and Node paths plus a marker identifying a global npm installation; the marker
SHALL not be asserted for local or npx/ephemeral package paths.

#### Scenario: Managed npm service receives SIGTERM
- **WHEN** a service manager sends SIGTERM to the foreground `node npm/run.js serve` process
- **THEN** the wrapper forwards SIGTERM to the Go child, waits for child cleanup, and exits without leaving the Go server orphaned

#### Scenario: Go child crashes
- **WHEN** the Go child exits unexpectedly while launched through `run.js`
- **THEN** the wrapper exits with the child's non-zero result so launchd/systemd can apply its configured restart policy

#### Scenario: Global launcher metadata is captured
- **WHEN** `service install` is invoked through a global npm package
- **THEN** the Go CLI receives absolute wrapper/Node metadata and can render a persistent launcher without resolving an ephemeral npx path

#### Scenario: Existing binary override is honored
- **WHEN** `NANO_BRAIN_BIN` points to an existing executable
- **THEN** `run.js` executes that path for ordinary commands and service installation resolves it as the direct validated launcher

### Requirement: Service startup SHALL use the installed configuration path

The generated manager command SHALL pass the canonical absolute config path before
the `serve` subcommand. Foreground startup under a manager SHALL therefore use
the same file regardless of the manager's working directory or the interactive
shell's transient `NANO_BRAIN_CONFIG` value. Service status SHALL derive its
endpoint from that installed file and SHALL not call the generic CLI request
recovery path.

#### Scenario: Manager starts from a different working directory
- **WHEN** launchd or systemd starts the installed unit outside the user's project directory
- **THEN** `serve` loads the pinned config path and binds the configured host/port

#### Scenario: Interactive environment changes after install
- **WHEN** the user's shell has a different `NANO_BRAIN_CONFIG` after installation
- **THEN** the managed service and `service status` continue to use the pinned config path, while ordinary non-service CLI invocations retain normal config precedence
