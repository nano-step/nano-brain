# Phase 13: Interactive Init Wizard - Research

**Researched:** 2026-07-02
**Domain:** CLI wizard orchestration — Docker CLI shell-out, Postgres connect-poll, Ollama-compatible HTTP, cross-platform daemon guard, koanf config semantics
**Confidence:** HIGH

## Summary

This phase upgrades `runInteractiveInit` (currently `cmd/nano-brain/init.go:26-253`) from a static "ask everything" prompt sequence into a progressive wizard that detects state and only asks what it can't resolve automatically. The codebase already has every load-bearing primitive the wizard needs — modular doctor checks, a config writer, `waitForServerHealthy`, and Phase 10's MCP client prompt flow — so this phase is almost entirely **orchestration and control-flow work**, not new infrastructure.

Two genuinely new capabilities are required: (1) shelling out to the `docker` CLI via `os/exec` to detect, provision, and recover a Postgres container, and (2) a `pgx.Connect`-based readiness poll loop. Both were verified empirically against a real local Docker daemon in this research session (not just read from docs): `docker run -d` with a name collision exits **125** with stderr containing `is already in use by container`; a port collision exits **125** with stderr containing `port is already allocated` **and leaves a stray `Created`-state container behind that must be `docker rm`'d before retrying on 5433**; `docker start <name>` on a stopped container exits 0. The pgx connect-poll loop mechanism was verified to correctly distinguish "connection refused" (container not listening yet) from "ready" (ping succeeds) — the actual settle time on a warm image was under 20ms in this environment, but published community guidance for cold-pull-plus-init scenarios documents Postgres self-restarting once during first-time initialization, which is why the poll must survive at least one connection-refused-then-recovers cycle, not just a single retry.

The Windows finding is more significant than the phase context anticipated: `cmd/nano-brain/daemon.go` is the **only** source of `runServeCmd`, `runServeDaemon`, `pidFilePath`, `runStopCmd`, and `runRestartCmd`, and it carries `//go:build !windows` with **no counterpart file**. A `GOOS=windows go build` of the whole CLI fails today (verified via cross-compile in this session) — this is a pre-existing gap unrelated to this phase, not something this phase should fix, but it means the wizard's serve step must runtime-guard on `runtime.GOOS == "windows"` (not just skip a Docker-shaped feature) and print the manual `nano-brain serve` instruction, exactly as D-specifics already anticipates.

The koanf empty-string question (D-11) is resolved: a config file value of `provider: ""` **does** override the struct default `"ollama"` after `Load()`'s defaults-then-file merge — verified with a standalone reproduction using the exact koanf provider chain nano-brain uses. No `provider: none` sentinel is needed.

**Primary recommendation:** Decompose `runInteractiveInit` into named step functions (config-exists gate → DB step → embedding step → advanced gate → write+doctor → serve step → register step → MCP step), each independently testable via the existing `promptReader`/`isTTYFn` injection pattern, each returning early/skip based on auto-detected state per D-01 through D-17. Add one new file `cmd/nano-brain/docker_provision.go` (or similar) for the `os/exec` Docker logic, kept fully separate from prompt/UI code so it can be table-driven tested with a fake `execRunner` seam.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Postgres detection/provisioning | CLI (client) | — | Wizard runs before any server process exists; must use `pgx.Connect` and `os/exec` directly, not an API call |
| Embedding provider selection | CLI (client) | Config file | Wizard writes `embedding.*` into config.yml; actual embedding calls happen server-side at runtime |
| Doctor health checks | CLI (client), calling `internal/health/doctor` package | — | Already a pure-function package, callable both from `doctor` command and wizard |
| Server start/health-poll | CLI (client) orchestrates; API/Backend (`/api/status`) is the signal | — | `waitForServerHealthy` polls the backend's own health endpoint; CLI owns the polling loop |
| Workspace registration | CLI (client) → API/Backend (`/api/v1/init`) | Database (workspaces table) | Existing `runInitCmd --root` HTTP path; wizard reuses it as an extracted helper |
| MCP client config | CLI (client), writes to filesystem (`.mcp.json`, `opencode.json`, `~/.codex/config.toml`) | — | Phase 10 code, reused verbatim — no tier change |
| Config persistence | CLI (client) writes; Config file is read by API/Backend at startup | — | Wizard is the sole writer; `internal/config.Load()` is the sole reader, shared by both `doctor` and `serve` |

## User Constraints (from CONTEXT.md)

<user_constraints>

### Locked Decisions

**Wizard flow & question budget**
- D-01: Core flow asks ≤6 questions total: (1) overwrite/keep existing config, (2) database (only if not auto-resolvable), (3) enable embeddings y/n (+ URL/model if yes and not auto-detected), (4) start server now y/n, (5) register current directory y/n, (6) per-MCP-client y/n (existing Phase 10 prompts). Everything else — harvester (auto-detect, no prompt), summarization (off), search/watcher/logging (defaults) — uses defaults silently.
- D-02: Add an "Advanced settings? [y/N]" gate at the point where today's detailed prompts begin. Default N skips straight to config preview. Y preserves the existing detailed prompt sequence (harvester dirs, summarization, search tuning, watcher, logging) unchanged — no functionality removed, just gated.
- D-03: If config already exists: offer "[k]eep / [o]verwrite" (default keep). Keep → skip all config questions and jump directly to the service steps (doctor → serve → register → MCP). This makes `nano-brain init` safely re-runnable as a "resume setup" command.
- D-04: Non-TTY behavior unchanged: interactive path requires a TTY (reuse `isTTYFn`); non-interactive callers keep using `init --root --json` / hand-written config. No new non-interactive contract in this phase.

**Database: detect → Docker provision → remote URL fallback**
- D-05: Detection order: (1) try connecting to the configured/default Postgres URL (`pgx.Connect`, 3s timeout — reuse `doctor.CheckPostgreSQL` shape). Reachable → use it, zero DB questions. (2) Not reachable → check `docker info` (CLI, 3s timeout). (3) Docker available → ask "PostgreSQL not found. Start one via Docker with default settings? [Y/n]". (4) No Docker or user declines → prompt for a PostgreSQL URL (remote or self-managed), with install guidance printed (`https://docs.docker.com/get-docker/`, pgvector requirement).
- D-06: Docker provisioning shells out to the `docker` CLI (`os/exec`) — no Docker SDK dependency (single-binary constraint, and the CLI is the only thing guaranteed present if `docker info` succeeded). Container: `docker run -d --name nanobrain-pg --restart unless-stopped -p 5432:5432 -e POSTGRES_USER=nanobrain -e POSTGRES_PASSWORD=nanobrain -e POSTGRES_DB=nanobrain_dev pgvector/pgvector:pg17` — identical to the documented manual command in SETUP_AGENT.md Step 4.
- D-07: Port 5432 conflict handling: if connect to :5432 failed but the port is occupied (container run fails with port-bind error), offer port 5433 and adjust the generated `database.url` accordingly. If a container named `nanobrain-pg` already exists but is stopped, `docker start nanobrain-pg` instead of `docker run`.
- D-08: After provisioning, poll readiness by attempting `pgx.Connect` in a loop (up to ~30s, 500ms interval) — not `docker exec pg_isready` (connect-poll validates the actual URL the config will use, including auth).
- D-09: Any user-entered Postgres URL is validated with a live connect before writing config. On failure: show the error, re-prompt (allow "save anyway" escape hatch with a warning so an intentionally-offline setup isn't blocked).
- D-10: Migrations stay auto-run at server start (existing `RunMigrations` in `startServer`) — the wizard does NOT run migrations itself. No new migration path.

**Embeddings: optional, any Ollama-compatible URL**
- D-11: New first question: "Enable semantic embeddings? [Y/n]". No → write `embedding:\n  provider: ""` — the server already degrades gracefully to BM25-only (`main.go:490-504` guards on `Provider != ""`); print a one-line note ("BM25 keyword search only — re-run `nano-brain init` to enable embeddings later").
- D-12: Yes → auto-detect local Ollama (`detectOllama`, existing). Found → confirm defaults (URL + `nomic-embed-text`). Not found → prompt for provider: `ollama` (any Ollama-compatible URL, local or cloud — the existing `NewOllamaEmbedder(url, model, dim)` already takes an arbitrary URL) or `voyage`. Do NOT build a new OpenAI-compatible embedding provider in this phase — that is REQ-INFRA-01 / issue #412, separate work. "Any URL" here means any Ollama-API-compatible endpoint.
- D-13: `doctor` must stop treating missing embeddings as failure when disabled: `CheckEmbeddingProvider`/`CheckEmbeddingModel` return `skip` (detail: "disabled — BM25-only") when `cfg.Embedding.Provider == ""` instead of defaulting to ollama and FAILing. This is the only doctor behavior change; when a provider IS configured, checks behave exactly as today.

**Chain to running state (config → doctor → serve → register → MCP)**
- D-14: After config write + doctor: ask "Start nano-brain server now? [Y/n]" → reuse `runServeDaemon(configPath)` + `waitForServerHealthy` (existing, `client_helpers.go:88`). If a server is already running (PID check / health OK), skip with a "already running" note. If doctor reported a FAIL on PostgreSQL, do not attempt server start — print what to fix and exit non-zero.
- D-15: After server healthy: ask "Register this directory as a workspace? [<cwd>]" (Enter = cwd, or type another path, empty-ish "n" to skip). Registration reuses the existing `--root` code path logic (POST `/api/v1/init`, `triggerInitBackground`) — refactor `runInitCmd`'s registration body into a callable helper rather than duplicating it.
- D-16: MCP client step reuses Phase 10's `promptMCPClientConfig` verbatim (Claude Code / OpenCode / Codex CLI, per-client Y/N, idempotent merge writers). "Show the list of supported tools and let the user choose" is exactly what it does. No new clients added in this phase.
- D-17: Final output: a short summary block — server URL, workspace name/hash, which MCP clients were configured, and the single next action ("restart your AI client"). This replaces today's dead-end ("start the server and run init --root ...").

**Docs**
- D-18: Rewrite `docs/SETUP_AGENT.md` around the new flow: prerequisites check (Node/Docker/optionally Ollama) → `npm install -g` → `nano-brain init` → verify. The per-step manual instructions move to a "Manual setup / troubleshooting" appendix (they remain valid for VPS/team path). Update README's Start section to `npm install -g @nano-step/nano-brain && nano-brain init`.

### Claude's Discretion
- Exact prompt wording, section headers, and summary formatting.
- Internal decomposition of `runInteractiveInit` (it will need splitting into testable steps — follow existing `promptWithDefault`/`promptConsequential` + injected `promptReader`/`isTTYFn` test-hook patterns).
- Whether Docker detection uses `docker info` or `docker version` (whichever is cheaper/more reliable).
- Timeout values within the ranges noted above.

### Deferred Ideas (OUT OF SCOPE)
- Embedded/zero-Docker database (PGlite/SQLite-vec) — architectural change, out of scope; Docker auto-provision chosen for this phase (user decision).
- OpenAI-compatible embedding provider — existing REQ-INFRA-01 / issue #412, separate phase.
- Auto-pull of the Ollama embed model (`ollama pull nomic-embed-text`) from the wizard — nice-to-have; this phase only checks and hints.
- Non-interactive one-shot flag (e.g. `init --yes` full-auto) — future; this phase is the interactive path.
- `curl | bash` distribution installer — explicitly deferred since Phase 10.

</user_constraints>

<phase_requirements>
## Phase Requirements

No formal REQUIREMENTS.md IDs are assigned to this phase (it is a roadmap-tracked phase, not tied to REQ-CI/REQ-SQ/etc.). The phase is fully scoped by CONTEXT.md decisions D-01 through D-18 above, which this research supports as follows:

| Decision | Research Support |
|----------|------------------|
| D-05, D-06, D-07 | Docker CLI exit codes and stderr patterns verified empirically (see Code Examples, Common Pitfalls) |
| D-08 | pgx connect-poll loop verified functionally correct; timing guidance cited from community sources |
| D-11, D-13 | koanf empty-string override behavior verified via reproduction; doctor skip-path pinpointed to exact lines |
| D-12 | Ollama auth-header gap in `OllamaEmbedder` confirmed by source read; Ollama cloud auth requirement confirmed via docs |
| D-14 | Windows daemon build-tag gap confirmed via cross-compile; exact functions affected enumerated |
| D-15 | Exact location and shape of registration logic to extract confirmed (`commands.go`, not `init.go`) |

</phase_requirements>

## Standard Stack

No new external dependencies are required for this phase. All work is achievable with:

| Library | Version (verified in go.mod) | Purpose | Why Standard |
|---------|------|---------|--------------|
| `os/exec` | stdlib | Shell out to `docker` CLI | D-06 explicitly forbids a Docker SDK dependency |
| `github.com/jackc/pgx/v5` | v5.7.2 (already a dependency) | Connect-poll for Postgres readiness, URL validation | Already used by `doctor.CheckPostgreSQL`; same shape reused |
| `bufio` | stdlib | Prompt scanning | Existing pattern (`promptWithDefault`, `promptConsequential`) |
| `github.com/knadh/koanf/v2` + `structs`/`file`/`yaml` providers | v2.3.4 / v1.0.0 / v1.2.1 / v1.1.0 (already dependencies) | Config load/merge — empty-string override verified | Already the project's config library |

**Installation:** None — no `go get` needed. This phase only adds new `.go` files in `cmd/nano-brain/` using stdlib + existing deps.

## Package Legitimacy Audit

**Not applicable.** This phase introduces zero new external packages (confirmed above — `os/exec`, `pgx/v5`, `bufio`, `koanf` are all either stdlib or already present in `go.mod`). No `npm view` / `package-legitimacy check` gate is required.

## Architecture Patterns

### System Architecture Diagram

```
 nano-brain init  (no args, TTY required)
        │
        ▼
 ┌─────────────────────────┐
 │ Config-exists gate (D-03)│──keep──▶ jump to [Doctor] ─▶ [Serve] ─▶ [Register] ─▶ [MCP]
 └─────────────────────────┘
        │ overwrite / no existing config
        ▼
 ┌─────────────────────────────────────────────────────────┐
 │ Database step (D-05..D-09)                               │
 │  1. pgx.Connect(default URL, 3s) ──reachable──▶ done      │
 │        │ unreachable                                      │
 │        ▼                                                   │
 │  2. os/exec "docker info" (3s) ──not found/daemon down──▶ prompt remote URL (D-09: live-validate)
 │        │ docker available                                  │
 │        ▼                                                   │
 │  3. Prompt "start via Docker?" [Y/n]                        │
 │        │ yes                                                │
 │        ▼                                                   │
 │  4. docker run -d nanobrain-pg pgvector/pgvector:pg17        │
 │       ├─ exit 125 "already in use by container" ──▶ docker start nanobrain-pg
 │       ├─ exit 125 "port is already allocated" ──▶ docker rm stray + retry on :5433
 │       └─ exit 0 ──▶ pgx.Connect poll loop (≤30s, 500ms interval, D-08)
 └─────────────────────────────────────────────────────────┘
        │
        ▼
 ┌─────────────────────────────────────────────────────────┐
 │ Embedding step (D-11, D-12)                               │
 │  "Enable semantic embeddings?" [Y/n]                       │
 │   No  ──▶ embedding.provider: ""  (BM25-only note)          │
 │   Yes ──▶ detectOllama(default URL)                         │
 │             found    ──▶ confirm URL+model                  │
 │             not found──▶ prompt provider (ollama URL | voyage)
 └─────────────────────────────────────────────────────────┘
        │
        ▼
 ┌─────────────────────────┐
 │ Advanced gate (D-02)      │──N (default)──▶ skip to preview
 └─────────────────────────┘
        │ Y
        ▼
 [existing harvester/summarization/search/watcher/logging prompts — unchanged]
        │
        ▼
 [Config preview + write (existing os.WriteFile 0600 pattern)]
        │
        ▼
 [Doctor] runDoctorCmd — now embedding checks skip cleanly when provider=="" (D-13)
        │
        ▼
 [Serve] (D-14) "Start server now?" [Y/n]
   ├─ already running (PID+health) ──▶ skip, note
   ├─ doctor FAILed Postgres ──▶ do not start, print fix, exit non-zero
   ├─ runtime.GOOS == "windows" ──▶ print manual `nano-brain serve` instruction, skip daemon call
   └─ else: runServeDaemon(configPath) + waitForServerHealthy(timeout)
        │
        ▼
 [Register] (D-15) "Register this directory?" [<cwd>]
   → extracted helper shared with `init --root` HTTP path (POST /api/v1/init, triggerInitBackground)
        │
        ▼
 [MCP] (D-16) promptMCPClientConfig(scanner, root, workspaceName) — Phase 10 code, unchanged
        │
        ▼
 [Summary] (D-17) server URL + workspace + configured clients + "restart your AI client"
```

### Recommended Project Structure
```
cmd/nano-brain/
├── init.go                  # runInteractiveInit — becomes a thin orchestrator calling the steps below
├── init_db.go                # NEW: database detection/provisioning step (D-05..D-10)
├── docker_provision.go       # NEW: os/exec docker CLI wrapper — info/run/start, exit-code classification
├── init_embedding.go         # NEW: embedding step (D-11, D-12)
├── init_serve.go             # NEW: serve step incl. Windows guard (D-14)
├── init_register.go          # NEW: extracted registration helper shared with commands.go (D-15)
├── commands.go                # runInitCmd --root branch calls the same extracted helper
├── mcp_client_config.go       # unchanged (D-16 reuse)
└── *_test.go                  # one test file per new step file, mirroring existing commands_test.go patterns
```

### Pattern 1: Injectable exec runner for Docker CLI calls
**What:** Wrap `exec.CommandContext` calls behind a small interface/function-var so tests can substitute canned stdout/stderr/exit-code without invoking real Docker.
**When to use:** All Docker CLI shell-outs (`docker info`, `docker run`, `docker start`).
**Example:**
```go
// Source: pattern mirrors existing runServeDaemonFn / isTTYFn test-hook idiom in client.go:18-19
type dockerRunner func(ctx context.Context, args ...string) (stdout, stderr string, exitCode int, err error)

var runDocker dockerRunner = func(ctx context.Context, args ...string) (string, string, int, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// docker binary itself not found — exec.ErrNotFound wrapped in *exec.Error
			return outBuf.String(), errBuf.String(), -1, err
		}
	}
	return outBuf.String(), errBuf.String(), exitCode, nil
}
```
Tests override `runDocker` the same way `commands_test.go` overrides `runServeDaemonFn` (see `withRecoveryHooks`, `commands_test.go:500-525`).

### Pattern 2: Docker daemon-vs-binary detection
**What:** Distinguish "docker not installed" from "Docker Desktop not running" so the wizard can print the correct guidance.
**When to use:** D-05 step 2 (`docker info` check).
**Example:**
```go
// VERIFIED empirically in this research session against a real docker CLI.
_, stderr, exitCode, err := runDocker(ctx, "info")
if err != nil {
    // *exec.Error wrapping ErrNotFound — docker binary is not on PATH at all
    return dockerStatusNotInstalled
}
if exitCode != 0 {
    // docker binary exists, daemon unreachable — stderr contains
    // "Cannot connect to the Docker daemon" on both macOS and Linux
    if strings.Contains(stderr, "Cannot connect to the Docker daemon") {
        return dockerStatusDaemonNotRunning
    }
    return dockerStatusUnknownError
}
return dockerStatusAvailable
```

### Pattern 3: pgx connect-poll loop (D-08)
**What:** Poll `pgx.Connect` + `Ping` on an interval, treating connection-refused as transient, not fatal.
**When to use:** After `docker run`/`docker start` succeeds, before writing `database.url` to config.
**Example:**
```go
// Source: mirrors doctor.CheckPostgreSQL (internal/health/doctor/doctor.go:61-86) connect+ping shape,
// wrapped in a retry loop. Mechanism VERIFIED in this session (see Code Examples).
func waitForPostgresReady(ctx context.Context, dbURL string, timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		connCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		conn, err := pgx.Connect(connCtx, dbURL)
		if err == nil {
			pingErr := conn.Ping(connCtx)
			conn.Close(connCtx)
			cancel()
			if pingErr == nil {
				return nil
			}
		} else {
			cancel()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("postgres did not become ready within %s", timeout)
		}
		time.Sleep(interval)
	}
}
```

### Pattern 4: Windows serve-step guard (D-14)
**What:** Runtime check (not build tag) inside the wizard's serve step, since `runServeDaemon` itself is compiled out entirely on Windows.
**When to use:** Immediately before calling `runServeDaemonFn` from the wizard.
**Example:**
```go
if runtime.GOOS == "windows" {
    fmt.Println("  Background daemon mode is not yet supported on Windows.")
    fmt.Printf("  Start the server manually in another terminal: nano-brain serve\n")
    return serveStepSkippedWindows
}
runServeDaemonFn(configPath)
```
Because `daemon.go` (which defines `runServeDaemon`) is `//go:build !windows` with no counterpart file, this guard MUST live in a file that is NOT itself Windows-excluded (e.g. `init_serve.go` with no build tag), and it must call `runServeDaemonFn` (the test-hook var already declared in the non-tagged `client.go:19`) — never `runServeDaemon` directly — to keep the wizard file buildable on Windows. On Windows, `runServeDaemonFn` will simply not exist (compiled out with the rest of `daemon.go`), so the guard's early `return` before touching that variable is what keeps the wizard file itself compiling; the wizard's serve-step file must not reference `runServeDaemonFn` unconditionally at package scope on Windows. **Confirm this at plan time**: the cleanest fix is very likely a `//go:build !windows` / `//go:build windows` pair of thin wrapper files (`init_serve_windows.go` returning `serveStepSkippedWindows` unconditionally, `init_serve_unix.go` doing the real call) rather than a single runtime-checked file, because `runServeDaemonFn` itself does not exist under Windows compilation.

### Anti-Patterns to Avoid
- **Using `docker exec ... pg_isready` for readiness (D-08 explicit rejection):** validates the container's internal socket, not the URL/auth the app will actually use. A container can be "ready" per `pg_isready` while the configured `database.url` (wrong user/db/port after a 5433 fallback) still fails. Always poll the real target URL with `pgx.Connect`.
- **Retrying `docker run` on the same port without cleanup:** VERIFIED in this session — a `docker run -d` that fails on port-bind still leaves a stray container in `Created` state occupying the name. Retrying with the same `--name` on a different port will then collide on the *name*, not the port, producing a confusing second error. Always `docker rm` the stray container (matched by name) before retrying.
- **Treating any non-zero `docker info` exit the same as "not installed":** collapses two different remediation paths (install Docker vs. start Docker Desktop) into one unhelpful message.
- **Calling `runServeDaemon` (not `runServeDaemonFn`) from new wizard code:** bypasses the existing test-hook seam and breaks `commands_test.go`-style testability.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|--------------|-----|
| Postgres liveness check | A custom TCP-dial-only check | `pgx.Connect` + `Ping`, same as `doctor.CheckPostgreSQL` | Already validates auth + protocol, not just port-open |
| Server health polling | A new polling loop | `waitForServerHealthy` (`client_helpers.go:88`) | Exact behavior needed already exists and is tested |
| MCP client config writers | New per-client writers | `promptMCPClientConfig` + `writeClaudeCodeMCPConfig`/`writeOpenCodeMCPConfig`/`writeCodexMCPConfig` (`mcp_client_config.go`) | Phase 10 already built idempotent, tested merge-writers |
| TTY/non-TTY prompt safety | New EOF-handling logic | `promptConsequential` (distinguishes Enter from closed stdin) | Prevents unconsented writes on piped/non-interactive stdin |

**Key insight:** This phase's biggest risk is scope creep into rebuilding things that already exist. The `docker` CLI wrapper and the pgx connect-poll loop are the only genuinely new primitives; everything else is composition of Phase 9/10/12 work.

## Runtime State Inventory

Not applicable — this is a net-new feature phase (interactive wizard flow), not a rename/refactor/migration. No existing runtime state (databases, service configs, OS registrations, secrets, build artifacts) is being renamed or relocated.

## Common Pitfalls

### Pitfall 1: Stray container left behind after a failed port-bind `docker run`
**What goes wrong:** D-07's port-5433 fallback retries `docker run` with the same `--name nanobrain-pg` after a port conflict on 5432. If the wizard doesn't clean up the first failed attempt, the retry collides on the container *name* (exit 125, "already in use") instead of succeeding on the new port.
**Why it happens:** Docker creates the container object (assigning it the name) before it attempts network binding; when binding fails, the container remains in `Created` state rather than being deleted (VERIFIED in this session).
**How to avoid:** Before retrying with a new port, run `docker rm nanobrain-pg` (ignoring "no such container" errors) if the prior attempt's exit code was 125.
**Warning signs:** Second `docker run` attempt errors with "is already in use by container" instead of a port-related message.

### Pitfall 2: Postgres self-restarts once during first-time container init
**What goes wrong:** A poll loop that treats the *first* successful TCP connect as "ready" can catch the brief window between Postgres's internal init-db restart, where a connection is accepted then immediately dropped.
**Why it happens:** Community-documented Postgres/testcontainers behavior: on first startup with an empty data directory, Postgres initializes, logs "database system is ready to accept connections", then shuts down and restarts once before serving external connections — this is normal Postgres init behavior, not specific to pgvector's image.
**How to avoid:** Use `pgx.Connect` + `Ping` (not just a raw TCP dial) in the poll loop — `Ping` will fail cleanly if the connection is torn down mid-handshake, and the loop will correctly retry rather than falsely reporting success on a connection about to be dropped. D-08's ~30s/500ms budget comfortably covers a double-restart cycle even under slow disk/cold cache.
**Warning signs:** Intermittent "connection reset" immediately after a reported-ready state, only on genuinely first-time container creation (not on `docker start` of an existing volume).

### Pitfall 3: Doctor's embedding checks currently hard-default to Ollama, masking "disabled" as "failing"
**What goes wrong:** Today, `CheckEmbeddingProvider` and `CheckEmbeddingModel` both `if cfg.Provider == "" { cfg.Provider = "ollama" }` before checking — meaning a user who intentionally disabled embeddings (D-11's `provider: ""`) gets a doctor FAIL trying to reach a nonexistent Ollama, not a clean skip.
**Why it happens:** These functions predate the "optional embeddings" wizard flow; they were written assuming Ollama-as-default was always the intended provider.
**How to avoid:** Per D-13, insert an early `if cfg.Provider == "" { return Check{Status:"skip", Detail:"disabled — BM25-only"}, nil }` guard at the top of `CheckEmbeddingProvider` (`internal/health/doctor/doctor.go:100`), and a matching skip in `CheckEmbeddingModel` (`doctor.go:145`) before either function's existing `if cfg.Provider == "" { cfg.Provider = "ollama" }` fallback line. `RunAll` needs no change — both functions already sit inline in the sequence.
**Warning signs:** `nano-brain doctor` reports FAIL for embedding provider/model even though the user explicitly chose BM25-only during setup.

### Pitfall 4: Windows daemon functions don't exist to guard against — the whole CLI doesn't build on Windows today
**What goes wrong:** A naive "wrap the daemon call in a `runtime.GOOS` check" fix, if placed inside `daemon.go` itself or any file that calls `runServeDaemonFn`/`pidFilePath`/etc. unconditionally at compile time, does nothing — those symbols simply don't exist when compiling for `GOOS=windows`, because the entire file carrying them is excluded.
**Why it happens:** `//go:build !windows` excludes the *file*, not conditionally-compiled statements inside it. There is no `daemon_windows.go` stub providing empty/error implementations.
**How to avoid:** VERIFIED via `GOOS=windows go build ./cmd/nano-brain/` in this session — it fails today (pre-existing, unrelated to this phase) with `undefined: runServeDaemon`, `undefined: pidFilePath`, `undefined: runServeCmd`, `undefined: runStopCmd`, `undefined: runRestartCmd`. This phase does not need to fix general Windows CLI support, but the wizard's own new serve-step code must not introduce a *new* unconditional reference to these symbols that would newly break under `GOOS=windows` cross-compilation (it doesn't today because those symbols are already broken on Windows). Confirm with the team whether Windows is even a supported build target currently (it appears not to be) before over-engineering the guard — a comment noting the pre-existing gap plus keeping the new wizard file's Windows-guard logic in a `!windows`-tagged file (matching daemon.go) is the pragmatic option if Windows CI doesn't currently exist.
**Warning signs:** Any CI matrix job attempting `GOOS=windows go build ./...` — check whether one exists before assuming Windows is a currently-working target.

### Pitfall 5: `runInitCmd` lives in `commands.go`, not `init.go`
**What goes wrong:** The scout report (used as prior input to this research) states the registration path lives in `init.go`/`commands.go` ambiguously and cites wrong line ranges. Planning a refactor against the wrong file will produce a broken diff.
**Why it happens:** Both files are in the same package and easy to conflate; the scout's multiple exploration passes recorded inconsistent locations across its own report.
**How to avoid:** VERIFIED by direct grep in this session: `runInitCmd` (the full function, including the `--root` HTTP registration body, `triggerInitBackground`, and the MCP-prompt call) is defined in `cmd/nano-brain/commands.go:14-132`. `cmd/nano-brain/init.go` contains only `detectOllama` and `runInteractiveInit` (lines 1-267). The D-15 extraction (shared registration helper) must be carved out of `commands.go`, with the new helper likely also living in `commands.go` or a new `init_register.go` that both `commands.go` and `init.go` import from (same package, so no import needed — just function visibility).
**Warning signs:** Grep for `func runInitCmd` before editing — do not trust either the CONTEXT.md code_context section or the scout report's line numbers for this specific function without reverifying (both were slightly imprecise for this project's current line numbers vs. described extraction target).

### Pitfall 6: Ollama cloud / hosted endpoints require Bearer auth that `OllamaEmbedder` does not send
**What goes wrong:** D-12 allows "any Ollama-compatible URL, local or cloud." If a user points the wizard at `https://ollama.com` (the official cloud API) or another auth-gated remote Ollama-compatible endpoint, `doctor.CheckEmbeddingProvider`'s `GET /api/tags` and `OllamaEmbedder.Embed`'s `POST /api/embed` will both fail with 401, because neither sends an `Authorization` header — confirmed by reading `internal/embed/ollama.go` (no header-setting beyond `Content-Type`) and `doctor.go:120` (`http.NewRequestWithContext` with no auth header).
**Why it happens:** `OllamaEmbedder` was built for local, unauthenticated Ollama; official Ollama cloud documents `Authorization: Bearer $OLLAMA_API_KEY` as required for `ollama.com/api/*` endpoints.
**How to avoid (in scope for this phase — a hint, not new code):** Per the phase's explicit instruction ("If not: note it as a wizard hint, NOT new scope"), the wizard should print a caveat when a user enters a non-localhost/non-private-IP Ollama URL: something like "Note: hosted/cloud Ollama-compatible endpoints often require an API key; nano-brain does not currently send an Authorization header (see issue tracker) — self-hosted or local Ollama endpoints are the tested path." Do not attempt to add auth-header support to `OllamaEmbedder` in this phase.
**Warning signs:** A user configures a cloud Ollama URL, `doctor` reports the embedding provider check failing with HTTP 401, and there is no config field to supply an API key for the `ollama` provider (only `voyageai` has `VoyageAPIKey`).

## Code Examples

### Docker daemon detection and exit-code classification (VERIFIED empirically, 2026-07-02)
```bash
# docker info when daemon IS running:
$ docker info >/dev/null 2>&1; echo $?
0

# docker run -d with a container name that already exists:
$ docker run -d --name nanobrain-pg -p 5432:5432 ... pgvector/pgvector:pg17
docker: Error response from daemon: Conflict. The container name "/nanobrain-pg"
is already in use by container "<id>". You have to remove (or rename) that
container to be able to reuse that name.
Run 'docker run --help' for more information
$ echo $?
125

# docker run -d with a port already bound by another container:
$ docker run -d --name nanobrain-pg2 -p 5432:5432 ... pgvector/pgvector:pg17
docker: Error response from daemon: failed to set up container networking:
driver failed programming external connectivity on endpoint nanobrain-pg2
(<id>): Bind for 0.0.0.0:5432 failed: port is already allocated
Run 'docker run --help' for more information
$ echo $?
125
# NOTE: `docker ps -a` afterward shows nanobrain-pg2 in "Created" state — it was
# NOT automatically removed. Must `docker rm nanobrain-pg2` before retrying.

# docker start on an existing stopped container:
$ docker start nanobrain-pg; echo $?
nanobrain-pg
0
```

### pgx connect-poll loop mechanics (VERIFIED functionally correct, 2026-07-02)
```go
// Reproduction used in this research session, adapted from doctor.CheckPostgreSQL's
// connect+ping shape (internal/health/doctor/doctor.go:61-86):
url := "postgres://nanobrain:nanobrain@localhost:5433/nanobrain_dev?sslmode=disable"
for i := 0; i < 60; i++ {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    conn, err := pgx.Connect(ctx, url)
    if err != nil {
        cancel()
        time.Sleep(500 * time.Millisecond)
        continue // connection refused while container is still starting
    }
    if pingErr := conn.Ping(ctx); pingErr != nil {
        conn.Close(ctx)
        cancel()
        time.Sleep(500 * time.Millisecond)
        continue
    }
    conn.Close(ctx)
    cancel()
    break // ready
}
// Observed: on a warm/cached pgvector:pg17 image + reused Docker layer cache,
// readiness was reached on the FIRST attempt (<20ms). This does not represent a
// cold-pull-plus-first-init timing; treat the ~30s/500ms budget in D-08 as
// necessary headroom for cold-start machines, not evidence the mechanism itself
// is slow — the mechanism is correct, only the *worst-case* duration is unverified
// in this sandboxed environment (no way to force a cold image pull here).
```

### koanf empty-string override (VERIFIED via standalone reproduction, 2026-07-02)
```go
// Reproduction confirms the exact provider chain nano-brain's config.Load() uses
// (structs.Provider for defaults, then file.Provider+yaml.Parser for the config file):
type Emb struct {
	Provider string `koanf:"provider"`
	URL      string `koanf:"url"`
}
type Cfg struct {
	Embedding Emb `koanf:"embedding"`
}

k := koanf.New(".")
defaults := &Cfg{Embedding: Emb{Provider: "ollama", URL: "http://localhost:11434"}}
_ = k.Load(structs.Provider(defaults, "koanf"), nil)
_ = k.Load(file.Provider("config.yml"), yaml.Parser()) // file contains: embedding:\n  provider: ""\n  url: ""
var out Cfg
_ = k.Unmarshal("", &out)
// Result: out.Embedding.Provider == ""  (NOT "ollama")
// Confirms D-11's `provider: ""` YAML value correctly overrides the default.
// No `provider: none` sentinel or special-casing is needed in config.go.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `runInteractiveInit` asks ~20 questions unconditionally (harvester, summarization, search tuning, watcher, logging, DB, embedding — all every time) | Progressive wizard: auto-detect what's resolvable, ask only what isn't, gate advanced questions behind an explicit opt-in (D-01/D-02) | This phase | Cuts a fresh-machine setup from ~20 prompts to ≤6; makes `init` safely re-runnable (D-03) |
| Interactive init ends with "start the server and run `init --root ...` manually" (dead end) | Wizard chains directly through doctor → serve → register → MCP config → summary (D-14..D-17) | This phase | Achieves the "one command" acceptance bar described in CONTEXT.md's Specific Ideas |
| Doctor always assumes an embedding provider is intended | Doctor recognizes `provider == ""` as an intentional BM25-only choice (D-13) | This phase | Doctor no longer falsely reports FAIL for a valid, deliberate configuration |

**Deprecated/outdated:** None — no libraries or APIs used by this phase are being deprecated; this is purely an internal control-flow refactor plus two new integration points (Docker CLI, connect-poll).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|----------------|
| A1 | Cold-start pgvector:pg17 container (fresh pull, empty volume, slow disk) settles within the ~30s/500ms budget D-08 specifies | Common Pitfalls #2, Code Examples | If a genuinely cold pull+init exceeds 30s (e.g., slow network pulling the image itself, which is a separate step from container *init* time), the wizard would report a false "not ready" even though provisioning is proceeding normally — mitigate by ensuring the image pull (`docker run` implicitly pulls if missing) is not counted against the *readiness* timeout, i.e., start the 30s clock only after `docker run`/`docker start` returns exit 0, not before |
| A2 | On Windows, no CI or user currently exercises `GOOS=windows` builds of nano-brain (informed by the absence of any Windows-related git history and the fact the build already fails) | Common Pitfalls #4, Architecture Pattern 4 | If Windows IS a currently-supported/tested target via some mechanism not found in this research (e.g., a separate build script, WSL-only usage treated as "Windows support"), the recommended fix (build-tag-paired wrapper files) may be more urgent than "pragmatic, low-priority" — worth a direct question to the user/planner before treating this as low-risk |
| A3 | Community-documented "Postgres restarts once on first init" behavior applies identically to the `pgvector/pgvector:pg17` image (a postgres:17 base with the pgvector extension pre-installed), not just vanilla `postgres` images | Common Pitfalls #2 | If pgvector's image has a different init sequence (e.g., additional restart for extension setup), the connect-poll loop's resilience assumption (survives one connect-then-drop cycle) may need to survive two |

## Open Questions (RESOLVED)

1. **Is Windows a currently working/tested nano-brain build target at all?**
   - RESOLVED (2026-07-02, plan 13-06/13-07): Windows is a pre-existing broken build target (`daemon.go` has no windows counterpart) — out of scope to fix. The wizard's serve step must not newly break Windows compilation; it runtime-guards with `runtime.GOOS == "windows"` and prints the manual `nano-brain serve` instruction. No `daemon_windows.go` this phase.
   - What we know: `GOOS=windows go build ./cmd/nano-brain/` fails today with 5 undefined symbols, all sourced from the single `//go:build !windows` file `daemon.go`. No windows-specific file exists anywhere in `cmd/nano-brain/`.
   - What's unclear: Whether this is a known, accepted limitation (e.g., "nano-brain is Unix-only for now, Windows users run via WSL") or an unintentional gap that happens to have gone unnoticed because nothing currently forces a Windows build.
   - Recommendation: The planner should treat "the wizard's serve step must not newly break Windows compilation" as the goal (low bar, since it's already broken), rather than "the wizard must gracefully degrade at runtime on a working Windows binary" (which presumes a working Windows binary exists to degrade). If the project intends real Windows support soon, that's a separate phase-sized effort (stub `daemon_windows.go`), not in scope here. Flag this explicitly to the user before planning locks in a specific Windows-guard implementation.

2. **What is the actual worst-case first-pull-plus-init time for `pgvector/pgvector:pg17` on a genuinely cold machine (no cached layers, average broadband)?**
   - RESOLVED (2026-07-02, plan 13-01/13-03): Pull time is unbounded and excluded from the readiness budget — the wizard prints "Pulling pgvector/pgvector:pg17 image..." during the blocking `docker run` call, and the 30s/500ms pgx connect-poll clock starts only after `docker run` returns. Exact cold-pull time never needs to be known.
   - What we know: With cached layers/warm image locally, readiness after container start was <20ms. Community sources describe Postgres's own init-restart cycle as typically completing within single-digit seconds once the container is running, but say nothing about layer *pull* time, which varies by network and is NOT what D-08's 30s budget is meant to cover.
   - What's unclear: Exact pull time for the `pgvector/pgvector:pg17` image (its size wasn't measured in this session) on a slow connection.
   - Recommendation: Structure the implementation so the wizard prints "Pulling pgvector/pgvector:pg17 image..." during the `docker run` call itself (which blocks until the image is available) and only starts the 30s/500ms readiness clock after `docker run` returns — this cleanly separates "image pull time" (unbounded, network-dependent, already visible via Docker's own pull progress on stderr) from "container init time" (bounded, the actual target of D-08's timeout).

3. **Should the wizard support an `OLLAMA_API_KEY`-style config field for the `ollama` provider today, given Ollama cloud requires it?**
   - RESOLVED (2026-07-02, plan 13-04): No code change to `OllamaEmbedder` — per D-12's explicit scope, the wizard accepts any Ollama-API-compatible URL as-is and prints a caveat hint for non-local URLs that authenticated endpoints (e.g. Ollama cloud) are not supported until a follow-up auth-header change. Hint-only.
   - What we know: D-12 explicitly limits scope to "Ollama-API-compatible URLs as-is" and this research confirms `OllamaEmbedder` sends no auth header at all, so cloud/authenticated endpoints will silently 401.
   - What's unclear: Whether users are actually expected to try Ollama cloud in practice for this phase, or whether "any Ollama-compatible URL" was intended primarily for self-hosted remote Ollama (which typically has no auth by default).
   - Recommendation: Per the phase's explicit instruction, treat this as a wizard-hint-only fix (print a caveat for non-local URLs), not a code change to `OllamaEmbedder`. Confirmed correct scope boundary — no action needed beyond the hint text.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|--------------|-----------|---------|----------|
| Docker CLI | D-05/D-06 Docker auto-provision | Yes (verified in this dev environment: `/usr/local/bin/docker`, daemon responsive, `docker info` exit 0) | Server 29.5.3 | If absent/daemon down at wizard-run time: prompt for remote Postgres URL (D-05 step 4) — already the designed fallback |
| PostgreSQL / pgvector | Core dependency | N/A at wizard design time — provisioned per-user at runtime | `pgvector/pgvector:pg17` (per D-06, pinned) | Remote URL entry (D-09) |
| Ollama (local) | Optional embeddings | Not checked in this dev environment (out of scope — server-side runtime dependency, not a build/CI dependency) | — | BM25-only via `provider: ""` (D-11) — already the designed fallback |
| Go toolchain | Build | Yes | go 1.25 (go.mod) | — |
| `GOOS=windows` cross-compile | Wizard's Windows serve-step guard | No — fails today, pre-existing (see Pitfall #4) | — | No fallback needed for THIS phase's scope; documented as pre-existing, out of scope to fix |

**Missing dependencies with no fallback:** None blocking this phase's implementation — Windows cross-compile failure is pre-existing and explicitly out of scope to fix.

**Missing dependencies with fallback:** Docker (fallback: remote URL prompt), Ollama (fallback: BM25-only) — both fallbacks are already the phase's designed behavior, not gaps.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing`, table-driven style |
| Config file | none — plain `go test` |
| Quick run command | `go test -race -short ./cmd/nano-brain/... ./internal/health/doctor/...` |
| Full suite command | `go test -race -short ./...` (+ `-tags=integration` for DB-backed suites against `nanobrain_test`/`:3199`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|---------------------|--------------|
| D-05/D-06/D-07 | Docker detection & provisioning exit-code classification (not-installed / daemon-down / name-conflict / port-conflict / success) | unit (injected `runDocker` fake) | `go test -race -short ./cmd/nano-brain/ -run TestDockerProvision` | ❌ Wave 0 — new file `docker_provision_test.go` |
| D-08 | pgx connect-poll loop distinguishes transient refuse vs. ready | unit (httptest-style fake pgx target is impractical; use a `net.Listen` TCP stub that accepts-then-closes to simulate refuse→ready, OR structure the poll function to accept an injected `connectFn` for pure unit testing) | `go test -race -short ./cmd/nano-brain/ -run TestWaitForPostgresReady` | ❌ Wave 0 — new file `init_db_test.go` |
| D-09 | User-entered URL live-validated, "save anyway" escape hatch works | unit (`promptReader` injection, existing pattern) | `go test -race -short ./cmd/nano-brain/ -run TestPromptPostgresURL` | ❌ Wave 0 |
| D-11/D-13 | `provider: ""` writes correctly; doctor skips (not fails) when disabled | unit | `go test -race -short ./internal/health/doctor/ -run TestCheckEmbeddingProvider_Disabled` | ❌ Wave 0 — extend `doctor_test.go` |
| D-12 | Ollama auto-detect found/not-found branches | unit (reuses existing `detectOllama` — already implicitly covered; add explicit case for wizard step) | `go test -race -short ./cmd/nano-brain/ -run TestEmbeddingStep` | ❌ Wave 0 |
| D-14 | Serve step: already-running skip, doctor-FAIL abort, Windows guard | unit (mirrors existing `TestPromptStartServer`/`withRecoveryHooks` pattern in `commands_test.go`) | `go test -race -short ./cmd/nano-brain/ -run TestInitServeStep` | ❌ Wave 0 |
| D-15 | Registration helper extraction — behavior-preserving | unit (reuse existing `TestInitCmdBuildsCorrectBody` pattern, add a second caller-path test) | `go test -race -short ./cmd/nano-brain/ -run TestInitCmdBuildsCorrectBody` | Partial — existing test in `commands_test.go:184`, needs extension for the new shared-helper call site |
| D-16 | MCP step reuse — no regression | existing | `go test -race -short ./cmd/nano-brain/ -run TestPromptMCPClientConfig` (exact name TBD — verify Phase 10 test names at plan time) | Likely ✅ — Phase 10 already covered this |
| D-01..D-04, D-17, D-18 | End-to-end wizard flow, question count, docs accuracy | manual-only / smoke | Manual TTY run against `nanobrain_test`/`:3199` per project convention; docs reviewed by inspection | Manual — no automated doc-drift check exists for `SETUP_AGENT.md` |

### Sampling Rate
- **Per task commit:** `go test -race -short ./cmd/nano-brain/... ./internal/health/doctor/...`
- **Per wave merge:** `go test -race -short ./...` (integration tag suite against `nanobrain_test`/`:3199` where DB-touching)
- **Phase gate:** Full suite green before `/gsd-verify-work`; plus one manual end-to-end wizard run (fresh config, Docker available) as a UAT step since D-01/D-17's "≤6 questions" and summary formatting are not mechanically assertable from unit tests alone.

### Wave 0 Gaps
- [ ] `cmd/nano-brain/docker_provision_test.go` — covers D-05/D-06/D-07 exit-code classification
- [ ] `cmd/nano-brain/init_db_test.go` — covers D-08/D-09 connect-poll and URL validation
- [ ] Extend `internal/health/doctor/doctor_test.go` — covers D-13 disabled-provider skip path
- [ ] `cmd/nano-brain/init_serve_test.go` (or extend `commands_test.go`) — covers D-14 serve-step branching including Windows guard
- [ ] Confirm exact Phase 10 MCP test file/names at plan time (not verified in this research pass — low risk, existing coverage very likely present per STATE.md Phase 10 completion notes)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|-----------------|---------|--------------------|
| V2 Authentication | No | Wizard does not add new auth surfaces; existing `auth.enabled`/bind-safety guard (`bindsafety.go`) untouched |
| V3 Session Management | No | N/A — CLI tool, no sessions |
| V4 Access Control | No | N/A |
| V5 Input Validation | Yes | User-entered Postgres URLs and Ollama/embedding URLs must be parsed with `net/url.Parse` before use (existing pattern in `doctor.go`); reject/warn on non-`postgres://` schemes for DB URL before attempting connect |
| V6 Cryptography | No | No new crypto — config file permission (0600) already enforced by existing `os.WriteFile` pattern, must be preserved for any new write paths |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|------------------------|
| Command injection via user-entered values passed to `os/exec` | Tampering | The wizard never interpolates user input directly into a shell string — `exec.CommandContext(ctx, "docker", args...)` with a fixed, hardcoded argument list (D-06's exact command) means no user-controlled data reaches `docker run`'s arguments except the port number (validated as an int in existing `strconv.Atoi` pattern) and possibly a custom container name if ever exposed (not currently planned — D-06/D-07 use the fixed `nanobrain-pg` name) |
| Config file world-readable after write (credential leakage — DB password, Voyage API key) | Information Disclosure | Preserve the existing `os.WriteFile(configPath, ..., 0600)` + explicit `os.Chmod` pattern already used both by `runInteractiveInit` and `mergeJSONMCPEntry`/`mergeCodexTOMLEntry` — any NEW file written by this phase's wizard steps must follow the same 0600 convention |
| SSRF-adjacent: wizard performs live HTTP/pgx connects to attacker-supplied "remote Postgres URL" / "Ollama URL" during setup | Tampering / Information Disclosure (low severity — local CLI tool, user-supplied input, user's own machine) | Existing `doctor.CheckPostgreSQL`/`CheckEmbeddingProvider` already perform this class of connect with a 3s timeout; no new risk introduced by extending the same pattern to wizard-time validation (D-09) — this is a deliberate, expected local-admin action, not a remote-attacker-controlled input path |

## Sources

### Primary (HIGH confidence — verified via tool execution in this research session)
- Local Docker CLI (`docker info`, `docker run`, `docker start`) exit codes and stderr strings — verified via direct execution against a real Docker daemon on this machine
- koanf empty-string override behavior — verified via standalone Go reproduction using the exact provider chain (`structs.Provider` → `file.Provider`+`yaml.Parser`) found in `internal/config/config.go:298-330`
- `GOOS=windows go build ./cmd/nano-brain/` failure and exact undefined symbols — verified via direct cross-compile attempt
- `cmd/nano-brain/init.go`, `cmd/nano-brain/commands.go`, `cmd/nano-brain/client_helpers.go`, `cmd/nano-brain/daemon.go`, `internal/health/doctor/doctor.go`, `internal/embed/ollama.go`, `internal/embed/factory.go`, `internal/config/config.go` — read directly in this session
- `go.mod` — read directly to confirm `pgx/v5` v5.7.2, `koanf/v2` v2.3.4, and absence of any Docker SDK dependency

### Secondary (MEDIUM confidence — WebSearch cross-checked against official docs)
- Ollama cloud API `Authorization: Bearer $OLLAMA_API_KEY` requirement — [docs.ollama.com/api/authentication](https://docs.ollama.com/api/authentication)
- Postgres/pgvector container self-restart-once-on-first-init behavior — testcontainers-for-go documentation and community sources (exact pgvector-image-specific confirmation is a documented assumption, see Assumptions Log A3)
- Docker "Cannot connect to the Docker daemon" vs. "command not found" distinguishability — cross-referenced across multiple Docker troubleshooting sources, consistent with this session's understanding of exec error types (`*exec.Error` for not-found vs. `*exec.ExitError` for daemon-down)

### Tertiary (LOW confidence — not independently verified, flagged for planner awareness)
- Exact pull time for `pgvector/pgvector:pg17` on a genuinely cold/slow connection (see Open Question #2) — not measured in this session (no way to force a true cold pull in this sandbox)
- Whether Windows is currently an intentionally-supported nano-brain target at all (see Open Question #1) — inferred from absence of evidence, not confirmed with the team

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies, all verified present in go.mod
- Architecture: HIGH — direct source reads of every integration point, corrected against scout report's minor line/file inaccuracies
- Docker CLI behavior: HIGH — empirically verified against real Docker daemon in this session, not just documentation
- Pitfalls: HIGH for Docker/koanf/Windows-build (all verified this session); MEDIUM for Postgres-restart-timing specifics (community-sourced, not independently reproduced under cold-start conditions)

**Research date:** 2026-07-02
**Valid until:** 30 days (stable stdlib/pgx/koanf behavior; Docker CLI output strings are considered stable across recent versions but should be re-verified if `docker` major version changes significantly before implementation)
