# Design: Container Server Guard

## Architecture

Single new file `cmd/nano-brain/guard.go` with three functions:

```
guardBeforeStart(configPort int) error
  ├── isContainer() bool
  ├── autoConfigContainer()           // sets NANO_BRAIN_HOST if in container
  └── checkExistingServer(port int)   // probes localhost + host.docker.internal
```

Called from two places:
1. `startServer()` in main.go — before config load completes, using default port (3100) or env override
2. `--daemon-child` path in main.go — same guard before server starts

## Flow

```
guardBeforeStart(port)
  │
  ├─ isContainer()?
  │   ├─ YES → autoConfigContainer() → set NANO_BRAIN_HOST=host.docker.internal
  │   │         print warning to stderr
  │   └─ NO → continue
  │
  ├─ NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1?
  │   └─ YES → return nil (skip port check)
  │
  ├─ checkExistingServer(port)
  │   ├─ probe localhost:{port}/api/status (timeout 2s)
  │   ├─ if NANO_BRAIN_HOST not set → also probe host.docker.internal:{port}/api/status
  │   ├─ if any responds → return error("server already running at {addr}")
  │   └─ none responds → return nil
  │
  └─ return nil (safe to start)
```

## Key Decisions

1. **Guard placement**: Before config load — we need to check early, and port can come from env var or default. Config file is loaded later.
2. **Probe timeout**: 2 seconds — fast enough for local check, generous for Docker networking.
3. **host.docker.internal check**: Only when NANO_BRAIN_HOST is NOT explicitly set. If user set it, they know what they're doing.
4. **Container detection**: `/.dockerenv` (Docker) + `KUBERNETES_SERVICE_HOST` (K8s). Simple and reliable.
5. **No build tag**: Unlike daemon.go, guard.go works on all platforms (no Unix-specific syscalls).
6. **stderr for warnings**: Guard warnings go to stderr so they don't interfere with JSON output from CLI commands.
