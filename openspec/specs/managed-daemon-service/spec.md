# managed-daemon-service Specification

## Purpose
TBD - created by archiving change cross-platform-daemon-auto-start. Update Purpose after archive.
## Requirements
### Requirement: Native service lifecycle commands

The CLI SHALL expose `nano-brain service install`, `uninstall`, `status`,
`restart`, and `update`. Version 1 SHALL support only the current user's macOS
LaunchAgent and Linux systemd user service. The CLI SHALL return an actionable
unsupported error without writing files when run on Windows, another GOOS, as
root, or inside a detected container. It SHALL NOT install a system-wide/root
service.

#### Scenario: macOS installation target
- **WHEN** a non-root user runs `nano-brain service install` on macOS
- **THEN** the CLI targets `~/Library/LaunchAgents/com.nano-step.nano-brain.plist`, uses label `com.nano-step.nano-brain`, and registers a per-user LaunchAgent

#### Scenario: Linux installation target
- **WHEN** a non-root user runs `nano-brain service install` on Linux with a usable `systemctl --user`
- **THEN** the CLI targets `~/.config/systemd/user/nano-brain.service` and registers a user unit

#### Scenario: Unsupported execution environment
- **WHEN** a user runs `nano-brain service install` on Windows, another GOOS, as root, or in a container
- **THEN** the CLI exits with code `3`, identifies the unsupported condition, and leaves no service definition or manager state

### Requirement: Native definitions SHALL own a foreground daemon

The generated service definition SHALL launch `serve` without `-d` or
`--daemon-child`. The macOS plist SHALL set `RunAtLoad=true`, `KeepAlive=true`,
`ThrottleInterval=5`, and explicit standard output/error paths under
`~/.nano-brain/logs/`. The Linux unit SHALL set `Restart=always`,
`RestartSec=2`, and `WantedBy=default.target`, and SHALL use journal-backed
standard output/error. The managed process SHALL NOT rely on the legacy PID file.

#### Scenario: Crash restart policy is encoded
- **WHEN** the generated macOS plist or Linux unit is inspected after installation
- **THEN** it contains the platform's keep-alive/restart policy and a foreground `serve` command, with no detach flag

#### Scenario: Service receives a normal stop
- **WHEN** the native manager stops the managed service
- **THEN** the foreground daemon receives the manager's termination signal, performs its existing graceful shutdown, and no legacy PID file is created as a side effect

### Requirement: Lifecycle transitions SHALL be idempotent and transactional

`install` SHALL create or replace the definition and start/enable it. Repeating
`install` SHALL converge to the current definition without duplicate manager
registrations. `update` SHALL regenerate the definition from current launcher
metadata and restart the service; if it is absent, `update` SHALL install and
start it. `restart` SHALL fail clearly when the service is not registered and
otherwise restart only the managed service. `uninstall` SHALL stop/disable or
boot out the service before removing only the fixed definition path; an absent
definition SHALL be a successful no-op. A manager failure SHALL return code `1`
with the failed operation and remediation, and a replaced definition SHALL be
restored when rollback is safe.

#### Scenario: Repeated install converges
- **WHEN** a user runs `service install` twice with the same config and launcher
- **THEN** the second invocation succeeds, leaves one definition and one manager registration, and starts the same service

#### Scenario: Update after an npm upgrade
- **WHEN** the current global npm wrapper resolves a new binary and the user runs `service update`
- **THEN** the definition is regenerated, the manager reloads it, the service restarts, and the subsequent status reports the new server version

#### Scenario: Uninstall is complete
- **WHEN** a user runs `service uninstall` for an installed service
- **THEN** the manager stops/disables or boots out the service, the fixed definition is removed, and later status reports unregistered without deleting config, logs, or data

#### Scenario: Manager operation fails
- **WHEN** definition replacement succeeds but registration or restart fails
- **THEN** the CLI reports the manager command failure, attempts to restore the prior definition, and does not claim installation succeeded

### Requirement: Service launcher and configuration paths SHALL be stable and safe

The service command SHALL store canonical absolute paths and argv-safe arguments.
It SHALL validate that a direct launcher is a regular executable. A global npm
launcher SHALL consist of validated absolute `node` and `npm/run.js` paths
provided by the wrapper; local, npx, and other ephemeral npm launchers SHALL be
rejected for persistent registration with a global-install remediation. The
service SHALL pin the resolved config path before `serve`, and SHALL NOT copy
shell environment variables, database URLs, tokens, API keys, or passwords into
the plist/unit. The existing `NANO_BRAIN_BIN` precedence SHALL remain honored by
installing its validated direct executable path.

#### Scenario: Global npm launcher is accepted
- **WHEN** `service install` is invoked through a globally installed npm wrapper that supplies stable `node` and `run.js` paths
- **THEN** the generated definition launches that pair with an absolute config path and `serve`, and contains no credential-bearing environment values

#### Scenario: Ephemeral npm launcher is rejected
- **WHEN** `service install` is invoked through an npx cache or project-local wrapper
- **THEN** the CLI exits with code `1`, explains that persistent registration requires a global npm install or direct binary, and writes no active definition

#### Scenario: Custom binary override remains direct
- **WHEN** `NANO_BRAIN_BIN` names an existing executable and the user runs `service install`
- **THEN** the service definition uses that validated absolute executable path and does not serialize the `NANO_BRAIN_BIN` variable

### Requirement: Generated definitions SHALL be private, atomic, and argv-safe

The CLI SHALL create parent directories with user-only access, write a temporary
definition in the target directory, restrict the file to user read/write
permissions, and atomically rename it into place. It SHALL escape XML values for
launchd and systemd values without invoking a shell or allowing control
characters to alter arguments. It SHALL reject unsafe or non-canonical launcher
paths and SHALL remove/replace only the fixed service definition owned by this
feature.

#### Scenario: Path containing spaces is rendered safely
- **WHEN** the launcher or config path contains spaces or XML-significant characters
- **THEN** the manager receives the exact path as one argument and the generated definition remains parseable without shell expansion

#### Scenario: Definition write is interrupted
- **WHEN** a definition write fails before the atomic rename
- **THEN** the existing definition remains unchanged and no partial temporary definition is treated as registered

#### Scenario: Definition permissions are inspected
- **WHEN** a service definition and its parent directory are created
- **THEN** they are not group/world writable and contain no secret values

### Requirement: `service status` SHALL expose registration, supervisor, and readiness separately

`service status` SHALL support `--json` and emit one object with the fields
`platform`, `registered`, `supervisor_state`, `health_reachable`, `ready`,
`endpoint`, `version`, and `error`. The endpoint SHALL be derived from the
installed config without invoking the generic HTTP client's auto-start recovery.
The command SHALL probe `/health` directly with a bounded timeout. It SHALL
use exit code `0` only when registered, active, reachable, and ready; `1` when
registered but inactive, failed, unreachable, unauthorized, degraded, or not
ready; `2` when unregistered; and `3` for an unsupported platform or unavailable
user manager. False/empty fields SHALL remain present in partial JSON states.

#### Scenario: Healthy managed service
- **WHEN** the definition exists, the manager reports active, and `/health` returns `ready=true` with a version
- **THEN** human and JSON status report registered/active/reachable/ready and the command exits `0`

#### Scenario: PostgreSQL is still unavailable
- **WHEN** the manager reports active but `/health` is unreachable or reports `ready=false`
- **THEN** status reports registered and active, sets `health_reachable`/`ready` accordingly, explains startup degradation, and exits `1` without launching a legacy daemon

#### Scenario: Health endpoint requires auth
- **WHEN** the manager reports active but the direct `/health` probe returns `401`
- **THEN** status reports the service as reachable with readiness false/unknown, does not guess or persist credentials, and exits `1`

#### Scenario: Service is not registered
- **WHEN** the fixed definition is absent and no manager registration exists
- **THEN** status reports `registered=false`, `supervisor_state="inactive"`, includes the install remediation, and exits `2`

### Requirement: Existing CLI recovery SHALL prefer a registered service

The recovery flow SHALL prefer the registered managed service. When the fixed
managed-service definition exists, connection-refused recovery
and the interactive init server step SHALL start or restart that service through
its native manager and wait for direct health readiness. They SHALL NOT call the
legacy `serve -d` path in that state. When no managed definition exists, the
existing recovery and initialization behavior SHALL remain unchanged. Installing
a service while a live legacy PID daemon owns the configured endpoint SHALL fail
with a stop/migration instruction instead of starting a second process.

#### Scenario: Recovery during delayed database startup
- **WHEN** a client cannot reach HTTP while the registered service is waiting for PostgreSQL
- **THEN** recovery uses the service manager and does not spawn a second PID-file daemon

#### Scenario: Legacy behavior without registration
- **WHEN** a client cannot reach HTTP and no managed definition exists
- **THEN** the existing prompt and `serve -d` recovery path remain available

#### Scenario: Install detects a legacy process
- **WHEN** `service install` finds a live legacy PID daemon on the configured endpoint
- **THEN** it exits with code `1`, leaves the existing service state unchanged, and tells the user to stop or migrate the legacy daemon

