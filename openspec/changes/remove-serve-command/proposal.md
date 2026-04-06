## Why

The `serve` command (daemon lifecycle manager with `install/uninstall/start/stop/status` subcommands and launchd/systemd integration) is now redundant because nano-brain's server runs inside a Docker container managed by `docker start/stop`. Additionally, 4 error messages in the CLI still tell users to run `serve install && launchctl load ...`, which is the old host-native approach and confuses users who are on the Docker-based setup.

## What Changes

- **BREAKING** — Remove the entire `serve` command (`serve start`, `serve stop`, `serve status`, `serve install`, `serve uninstall`, `serve --foreground`) and all associated PID-file management from `src/index.ts`
- Add `NANO_BRAIN_HOST` and `NANO_BRAIN_PORT` environment variable support so containers can explicitly configure the nano-brain server address instead of relying on the hardcoded `host.docker.internal` fallback
- **BREAKING** — Delete `src/service-installer.ts` (launchd plist / systemd unit generation)
- Remove `serve` case from the command dispatch switch in `src/index.ts`
- Remove `serve` entry from `showHelp()` output in `src/index.ts`
- Update 4 error messages (lines ~1414, ~1558, ~1725, ~2322 in `src/index.ts`) from the old "daemon not running … serve install" text to Docker-based guidance: `docker start nano-brain`
- Remove any serve-related npm scripts from `package.json`
- Update or remove serve-related tests in `test/cli.test.ts`

## Capabilities

### New Capabilities

- `remove-serve-command`: Covers the removal of the `serve` CLI command, deletion of `service-installer.ts`, cleanup of command dispatch and help text, and updating error messages to reference Docker
- `configurable-server-address`: The `getHttpHost()` function checks `NANO_BRAIN_HOST` env var first, then falls back to container detection (`host.docker.internal`), then `localhost`. Similarly, `NANO_BRAIN_PORT` overrides the default port 3100. This allows containers in non-standard Docker networking setups (e.g., nano-brain running in a separate container where `host.docker.internal` doesn't route correctly) to explicitly configure the server address.

### Modified Capabilities

_(none — no existing spec-level requirements are changing; the `mcp-server` spec is unaffected since the `mcp` command and `server.ts` are preserved)_

## Impact

- **CLI surface**: The `serve` command and all its subcommands will no longer be available. Users who relied on `npx nano-brain serve install` must use `docker start nano-brain` instead.
- **Files affected**:
  - `src/index.ts` — remove `handleServe()`, update error messages, update help text, remove `serve` dispatch case
  - `src/service-installer.ts` — delete entirely
  - `package.json` — remove serve-related scripts (if any)
  - `test/cli.test.ts` — remove/update serve-related test cases
- **No impact on**: `mcp` command, `server.ts`, all other CLI commands (`query`, `search`, `vsearch`, `write`, `update`, `status`, etc.), Docker-based deployment
