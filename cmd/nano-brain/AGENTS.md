# cmd/nano-brain — CLI Dispatcher + Server Entry Point

Custom dispatcher (no cobra). `main.go` switches on `os.Args[1]`; each command routes to a `runXxxCmd()` function in the same package.

## Files

| File | Purpose |
|---|---|
| `main.go` | Entry point: flag parsing, command dispatch switch, `startServer()` |
| `commands.go` | `runWriteCmd`, `runQueryCmd`, `runSearchCmd`, `runVSearchCmd`, `runReindexCmd`, `runHarvestCmd`, `runInitCmd`, shared `runStubCmd` |
| `ops.go` | Server-side ops: `runLogsCmd`, `runStatusCmd`, `runVersionCmd`, `runDockerCmd`, `runBenchCmd`, `runDBMigrateCmd`, `runResetEmbeddingsCmd` |
| `daemon.go` | `runServeCmd`, `runServeDaemon`, `runStopCmd`, `runRestartCmd`; PID-file lifecycle (`!windows`) |
| `guard.go` | Single-instance guard: container detection, `probeServer`, `guardBeforeStart` |
| `client.go` | HTTP client: `doRequest`, `sendRequest`, `getBaseURL`, `resolveHostPort` |
| `client_helpers.go` | `promptStartServer`, `isTTY`, `isNpxLaunched`, `suggestStartCommand` |
| `collection.go` | `runCollectionCmd` — add/remove/list collections via HTTP |
| `workspaces.go` | `runWorkspacesCmd` — list workspaces via HTTP |
| `init.go` | `runInteractiveInit` — guided wizard for workspace registration |
| `config_cmd.go` | `runConfigCmd` — show/validate config |
| `doctor.go` | `runDoctorCmd` — prerequisite checks (PG, pgvector, Ollama, model) |
| `detect.go` | `detectOpenCodeStorageDir`, `detectOpenCodeDBPath`, `detectClaudeCodeStorageDir` |
| `migrate.go` | `runDBMigrateCmd` — goose migrations + V1 SQLite import |
| `bench.go` | `runBenchCmd` — generate/run/compare/stress benchmark suite |
| `cmd_reset_embeddings.go` | `runResetEmbeddingsCmd` — clear embedding state |
| `commands_test.go` | Unit tests for HTTP client helpers and `getBaseURL` |
| `ops_test.go` | Tests for `runStatusCmd`, `runVersionCmd`, log helpers |
| `detect_test.go` | Tests for `detectOpenCodeStorageDir`, `detectOpenCodeDBPath` |
| `doctor_test.go` | Tests for doctor prerequisite checks |
| `workspaces_test.go` | Tests for `runWorkspacesCmd` |

## Key Patterns

**Command registration:** Add a `case "mycommand":` in `main.go`'s switch, call `runMycommandCmd(args[1:])`. No registration struct needed.

**Server startup flow:**
```
main() → startServer(configPath)
  → guardBeforeStart()           // block if already running
  → config.Load()
  → storage.NewPool()
  → storage.RunMigrations()
  → server.New(cfg, pool, ...)   // internal/server
  → errgroup: srv.Start() + fw.Run() + eq.Run() + hr.Run()
```

**Daemon mode:** `serve -d` spawns a child process with `--daemon-child`; writes PID to `~/.nano-brain/nano-brain.pid`. Child calls `startServer()` directly. Bypass duplicate-instance check with `NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1`.

**CLI → server:** All non-serve commands call `doRequest(method, getBaseURL()+"/api/v1/...", body)`. Base URL is `http://$NANO_BRAIN_HOST:$NANO_BRAIN_PORT` (default `localhost:3100`). Container environments auto-set `NANO_BRAIN_HOST=host.docker.internal`.

**`runStubCmd`:** Shared handler for query/search/vsearch. Parses `--workspace` + `--json`, POSTs to `/api/v1/<endpoint>`.

## Adding a New CLI Command

1. Add `case "mycommand": runMycommandCmd(args[1:]); return` in `main.go` switch.
2. Implement `func runMycommandCmd(args []string)` in `commands.go` (or a new file for large commands).
3. Call `doRequest` or `sendRequest` to hit the server API.
4. Add test cases in `commands_test.go` using `httptest.NewServer` for HTTP mocking.

## Testing

Tests use `httptest.NewServer` to mock the nano-brain HTTP API — no live server needed. Set `os.Setenv("NANO_BRAIN_HOST", ...)` + `NANO_BRAIN_PORT` to point `getBaseURL()` at the test server. See `commands_test.go:17` for the pattern.
