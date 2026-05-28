# Spec: CLI Connect-Error UX

## ADDED Requirements

### Requirement: Connect-error message format
When the CLI cannot reach the nano-brain server, the error output MUST contain three lines: a header naming the unreachable address, a hint that the server is not running, and a concrete next-action command.

#### Scenario: Server unreachable, error message structure
- Given the nano-brain server is not running on `localhost:3100`
- When any CLI command that calls the server is invoked (e.g. `nano-brain init --root /tmp/x`)
- Then stderr contains a line matching `Error: cannot connect to nano-brain server at localhost:3100`
- And stderr contains a line indicating the server is not running
- And stderr contains a line starting with `Run this to start it:` followed by an executable command
- And the exit code is non-zero

#### Scenario: Custom host/port in error
- Given `NANO_BRAIN_HOST=my-host` and `NANO_BRAIN_PORT=9999` are set
- And no server is running at `my-host:9999`
- When any CLI command that calls the server is invoked
- Then the error message names `my-host:9999` (not the default)

### Requirement: Start-command suggestion auto-detection
The suggested start command MUST reflect how the CLI was launched.

#### Scenario: Launched via npx
- Given `npm_execpath` or `npm_package_name` environment variables are set
- When the connect-error path is taken
- Then the suggestion line contains `npx @nano-step/nano-brain@beta serve -d`

#### Scenario: Launched as binary
- Given neither `npm_execpath` nor `npm_package_name` is set
- When the connect-error path is taken
- Then the suggestion line contains `nano-brain serve -d`

### Requirement: Interactive auto-start prompt
When stdin AND stderr are both TTYs, the CLI MUST offer to start the server before exiting.

#### Scenario: Both stdin and stderr are TTY, user accepts
- Given stdin and stderr are TTYs
- And `NANO_BRAIN_NO_AUTO_START` is not set
- And the server is not running
- When the connect-error path is taken
- Then the user is prompted with text matching `Start server now? [Y/n]:`
- And on `Y` or empty input, `runServeDaemon` is invoked
- And after the daemon reports healthy (HTTP 200 on `/api/status`), the original request is retried exactly once
- And on retry success, the original CLI command succeeds as if the server had been up the whole time

#### Scenario: User declines the prompt
- Given the interactive prompt is shown
- When the user types `n` (or any non-empty answer other than `Y`/`y`)
- Then no daemon is started
- And the original error is restored to stderr
- And the process exits non-zero

### Requirement: Non-TTY suppression
The CLI MUST NOT prompt or auto-start when stdin OR stderr is non-TTY.

#### Scenario: Stdin piped
- Given stdin is a pipe (not a TTY)
- And stderr is a TTY
- When the connect-error path is taken
- Then no prompt is shown
- And the error + suggestion are still printed
- And the process exits non-zero

#### Scenario: Stderr redirected to file
- Given stdin is a TTY
- And stderr is redirected to a file
- When the connect-error path is taken
- Then no prompt is shown

#### Scenario: Both non-TTY (CI / agent harness)
- Given neither stdin nor stderr is a TTY
- When the connect-error path is taken
- Then no prompt is shown

### Requirement: NANO_BRAIN_NO_AUTO_START override
The `NANO_BRAIN_NO_AUTO_START=1` environment variable MUST disable the interactive prompt even when both stdin and stderr are TTYs.

#### Scenario: Override set in TTY session
- Given `NANO_BRAIN_NO_AUTO_START=1`
- And stdin and stderr are TTYs
- And the server is not running
- When the connect-error path is taken
- Then no prompt is shown
- And the error + suggestion are still printed
- And the process exits non-zero

### Requirement: Health-check before retry
After auto-starting the daemon, the CLI MUST wait for the server to report healthy before retrying.

#### Scenario: Daemon becomes healthy quickly
- Given the daemon was just started via auto-start
- When the CLI polls `GET /api/status`
- Then it returns HTTP 200 within 10 seconds
- And the original request is retried

#### Scenario: Daemon fails to become healthy
- Given the daemon was started but does not respond to `/api/status` within 10 seconds
- When the polling deadline expires
- Then the CLI prints an error pointing at `~/.nano-brain/logs/nano-brain.log`
- And exits non-zero
- And does NOT retry the original request

### Requirement: Single retry only
The auto-start flow MUST retry the original request at most once.

#### Scenario: Retry succeeds
- Given the daemon is now healthy
- When the CLI retries the original request
- And the retry returns a successful response
- Then the command completes normally

#### Scenario: Retry fails
- Given the daemon is now healthy
- When the CLI retries the original request
- And the retry returns an error (5xx, timeout, etc.)
- Then the CLI prints the retry error
- And does NOT retry again
- And exits non-zero
