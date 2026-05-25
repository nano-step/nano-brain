# Spec: Server Guard

## ADDED Requirements

### Requirement: Port check before server start
The system MUST probe for an existing nano-brain server before starting a new instance.

#### Scenario: No existing server
- Given no nano-brain server is running
- When `startServer()` is called
- Then server starts normally

#### Scenario: Server already running on localhost
- Given a nano-brain server is running on localhost:3100
- When `startServer()` is called
- Then it prints "nano-brain server already running at localhost:3100" to stderr
- And exits with code 1

#### Scenario: Server running on host.docker.internal (from container)
- Given NANO_BRAIN_HOST is NOT set
- And a nano-brain server is running on host.docker.internal:3100
- When `startServer()` is called from inside a container
- Then it prints "nano-brain server already running at host.docker.internal:3100" to stderr
- And exits with code 1

#### Scenario: Override allows duplicate
- Given NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1
- And a server is already running
- When `startServer()` is called
- Then server starts normally (port check skipped)

### Requirement: Container detection
The system MUST detect Docker and Kubernetes container environments.

#### Scenario: Docker container
- Given `/.dockerenv` file exists
- When `isContainer()` is called
- Then it returns true

#### Scenario: Kubernetes pod
- Given `KUBERNETES_SERVICE_HOST` env var is set
- When `isContainer()` is called
- Then it returns true

#### Scenario: Host machine
- Given no container indicators present
- When `isContainer()` is called
- Then it returns false

### Requirement: Auto-configure NANO_BRAIN_HOST in container
The system MUST auto-set NANO_BRAIN_HOST when running in a container and the variable is not explicitly set.

#### Scenario: Container without explicit host
- Given running inside a container
- And NANO_BRAIN_HOST is NOT set
- When guard runs
- Then NANO_BRAIN_HOST is set to "host.docker.internal"
- And warning is printed to stderr

#### Scenario: Container with explicit host
- Given running inside a container
- And NANO_BRAIN_HOST is set to "my-host"
- When guard runs
- Then NANO_BRAIN_HOST remains "my-host"
- And no auto-config warning is printed

### Requirement: Guard applies to all server start paths
The guard MUST run before the server starts, regardless of entry point.

#### Scenario: No-args start
- When `nano-brain` is run with no args
- Then guard runs before server starts

#### Scenario: serve command
- When `nano-brain serve` is run
- Then guard runs before server starts

#### Scenario: serve -d command
- When `nano-brain serve -d` is run
- Then guard runs before daemon forks

#### Scenario: daemon-child
- When `nano-brain --daemon-child` runs
- Then guard runs before server starts
