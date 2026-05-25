# Container Server Guard

## Problem

AI agents running inside containers can ignore AGENTS.md instructions and start a nano-brain server inside the container, causing duplicate watcher/harvester/embed work. Additionally, two server instances on the same host (different ports or same port in different network namespaces) waste resources.

## Solution

Two complementary guards applied before any server start (`startServer()`, `serve`, `serve -d`):

### A. Port Check
Before starting, probe for an existing nano-brain server:
1. `GET /api/status` on `localhost:{port}` (configured port)
2. If `NANO_BRAIN_HOST` is NOT explicitly set, also check `host.docker.internal:{port}`
3. If any responds → refuse to start with message showing which address has a running server

### B. Container Detection
Detect container environment and auto-configure:
1. Check `/.dockerenv` file OR `KUBERNETES_SERVICE_HOST` env var
2. If container detected and `NANO_BRAIN_HOST` not set → auto-set to `host.docker.internal`
3. Print warning: "Detected container environment. CLI commands will use host.docker.internal:{port}."

### Override
`NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1` bypasses the port check (not container detection).

## Scope
- **In scope**: Guard logic in `cmd/nano-brain/`, applied to `startServer()` entry point
- **Out of scope**: Windows support (daemon.go already Unix-only), config file changes

## Files Changed
- `cmd/nano-brain/guard.go` (NEW) — `checkExistingServer()`, `isContainer()`, `autoConfigContainer()`
- `cmd/nano-brain/main.go` — call guard before `startServer()`
- `cmd/nano-brain/daemon.go` — call guard before daemon child starts

## References
- Issue: #137
