## Context

nano-brain originally ran as a host-native daemon managed by the `serve` command, which handled process lifecycle (start/stop via PID files), service installation (launchd plists on macOS, systemd units on Linux), and foreground execution. The server is now deployed inside a Docker container, making the entire `serve` subsystem redundant. The `mcp` command (used by Docker: `npx tsx src/index.ts mcp --http --port=3100 --host=0.0.0.0`) remains the correct entry point for the server.

Four error messages in the CLI still reference the old `serve install && launchctl load` workflow, confusing users who encounter connection failures.

**Current code layout:**
- `src/index.ts` line 13: `import { installService, uninstallService } from './service-installer.js'`
- `src/index.ts` lines 434–691: `handleServe()` function (~258 lines) — PID management, start/stop/status/install/uninstall subcommands
- `src/index.ts` line 4105: `case 'serve': return handleServe(globalOpts, commandArgs)`
- `src/index.ts` line 223: `serve` entry in `showHelp()` output
- `src/index.ts` lines ~1414, ~1558, ~1725, ~2322: four "Daemon not running" error messages
- `src/service-installer.ts`: entire file (~100 lines) — launchd/systemd service generation

## Goals / Non-Goals

**Goals:**
- Remove all `serve` command code and its service-installer dependency
- Update error messages to reference Docker-based server management
- Keep the `mcp` command and `server.ts` completely untouched
- Remove dead imports and any serve-related npm scripts
- Update tests to reflect the removal

**Non-Goals:**
- Changing how the `mcp` command works
- Modifying `server.ts` or any MCP tool implementations
- Adding new Docker management commands to the CLI
- Changing the Docker container setup or Dockerfile

## Decisions

### 1. Full removal vs deprecation warning

**Decision:** Full removal (no deprecation period).

**Rationale:** The `serve` command has been superseded by Docker for all users. There is no gradual migration path needed — Docker is already the only supported deployment method. A deprecation warning would add complexity for zero benefit since no one should be calling `serve` anymore.

**Alternative considered:** Add a deprecation warning that prints "use docker start nano-brain instead" — rejected because it keeps dead code around and the service-installer dependency alive.

### 2. Error message wording

**Decision:** Replace all four error messages with:
```
Error: nano-brain server not reachable. Ensure the Docker container is running:
  docker start nano-brain
```

**Rationale:** Simple, actionable, and matches the current deployment model. The old message referenced `serve install` and `launchctl load` which are macOS-specific and no longer applicable.

### 3. Surgical deletion approach

**Decision:** Delete in this order to avoid broken intermediate states:
1. Remove `import { installService, uninstallService }` from line 13
2. Delete `handleServe()` function (lines 434–691)
3. Remove `case 'serve'` from command dispatch (line 4105)
4. Remove `serve` from `showHelp()` (line 223)
5. Update the 4 error messages (lines ~1414, ~1558, ~1725, ~2322)
6. Delete `src/service-installer.ts` entirely
7. Clean up `package.json` (remove any serve-related scripts)
8. Update `test/cli.test.ts`

**Rationale:** This order ensures each step produces a compilable state. The import removal and function deletion happen first so the dispatch case removal doesn't reference a missing function.

### 4. Configurable server address via environment variables

**Decision:** Add `NANO_BRAIN_HOST` and `NANO_BRAIN_PORT` environment variable support to `getHttpHost()` and the default port logic.

**Rationale:** The current `getHttpHost()` returns `"host.docker.internal"` when inside a container and `"localhost"` otherwise. This is hardcoded and doesn't work in all Docker networking setups — for example, when nano-brain runs in a separate container and `host.docker.internal` doesn't route correctly. By checking `NANO_BRAIN_HOST` first, containers can explicitly configure the server address.

**Resolution chain:**
1. `NANO_BRAIN_HOST` env var → if set, use it directly
2. Container detection → `host.docker.internal`
3. Default → `localhost`

Similarly for port:
1. `NANO_BRAIN_PORT` env var → if set, parse as integer
2. Default → `3100` (`DEFAULT_HTTP_PORT`)

**Implementation:**
```typescript
function getHttpHost(): string {
  if (process.env.NANO_BRAIN_HOST) return process.env.NANO_BRAIN_HOST;
  return isRunningInContainer() ? 'host.docker.internal' : 'localhost';
}

function getHttpPort(): number {
  if (process.env.NANO_BRAIN_PORT) return parseInt(process.env.NANO_BRAIN_PORT, 10);
  return DEFAULT_HTTP_PORT; // 3100
}
```

The `detectRunningServer()` and `detectRunningServerContainer()` functions, as well as error messages, should use these helpers to show the configured host:port.

**Alternative considered:** A CLI flag like `--server-host` — rejected because the env var approach is simpler, works across all commands without repetition, and is the standard pattern for container configuration.

## Risks / Trade-offs

- **[Risk] Users on older setups still using `serve`** → Mitigation: The Docker migration happened before this change. Any remaining `serve` users will get a clear "unknown command" error and can check docs.
- **[Risk] Stale PID files left on disk** → Mitigation: Out of scope — leftover `~/.nano-brain/serve.pid` files are harmless and will be ignored.
- **[Risk] Other code referencing `service-installer.ts`** → Mitigation: Only one import exists (line 13 of `index.ts`). Verified via grep — no other files import it.
- **[Risk] Invalid `NANO_BRAIN_PORT` value** → Mitigation: `parseInt()` returns `NaN` for non-numeric strings; the connection will fail with a clear error. A validation check could be added but is low priority since this is an advanced configuration knob.
