# Phase 13: Interactive Init Wizard - Context

**Gathered:** 2026-07-02
**Status:** Ready for planning

<domain>
## Phase Boundary

Make `nano-brain init` (no args) a **one-command interactive setup** that takes a user from "binary installed" to "MCP tools working in their AI client" in a single flow: detect/provision PostgreSQL → optional embeddings → write config → start server → register workspace → MCP client auto-config. Replaces the current 10-step manual flow documented in `docs/SETUP_AGENT.md`.

The `init --root <path>` non-interactive registration path keeps its existing contract (server must be running, `--json` for scripts) — this phase upgrades the **no-args interactive path** (`runInteractiveInit`) and chains it into the existing registration + MCP config machinery.

**User-locked product decisions (Tâm, 2026-07-02):**
1. **PostgreSQL:** check Docker; if present, ask user "use default config?" → yes = auto-provision the pgvector container; no / no Docker → guide install OR let user enter a remote PostgreSQL URL.
2. **Embeddings:** ask user enable or not; check for local Ollama OR let user enter any embed URL — not necessarily localhost Ollama (Ollama cloud / remote Ollama-compatible endpoints are valid).
3. **Final step:** show the list of supported MCP clients and let the user choose which to auto-configure.

</domain>

<decisions>
## Implementation Decisions

### Wizard flow & question budget
- **D-01:** Core flow asks **≤6 questions total**: (1) overwrite/keep existing config, (2) database (only if not auto-resolvable), (3) enable embeddings y/n (+ URL/model if yes and not auto-detected), (4) start server now y/n, (5) register current directory y/n, (6) per-MCP-client y/n (existing Phase 10 prompts). Everything else — harvester (auto-detect, no prompt), summarization (off), search/watcher/logging (defaults) — uses defaults silently.
- **D-02:** Add an "Advanced settings? [y/N]" gate at the point where today's detailed prompts begin. Default N skips straight to config preview. Y preserves the existing detailed prompt sequence (harvester dirs, summarization, search tuning, watcher, logging) unchanged — no functionality removed, just gated.
- **D-03:** If config already exists: offer "[k]eep / [o]verwrite" (default keep). Keep → skip all config questions and jump directly to the service steps (doctor → serve → register → MCP). This makes `nano-brain init` safely re-runnable as a "resume setup" command.
- **D-04:** Non-TTY behavior unchanged: interactive path requires a TTY (reuse `isTTYFn`); non-interactive callers keep using `init --root --json` / hand-written config. No new non-interactive contract in this phase.

### Database: detect → Docker provision → remote URL fallback
- **D-05:** Detection order: (1) try connecting to the configured/default Postgres URL (`pgx.Connect`, 3s timeout — reuse `doctor.CheckPostgreSQL` shape). Reachable → use it, zero DB questions. (2) Not reachable → check `docker info` (CLI, 3s timeout). (3) Docker available → ask "PostgreSQL not found. Start one via Docker with default settings? [Y/n]". (4) No Docker or user declines → prompt for a PostgreSQL URL (remote or self-managed), with install guidance printed (`https://docs.docker.com/get-docker/`, pgvector requirement).
- **D-06:** Docker provisioning shells out to the `docker` CLI (`os/exec`) — **no Docker SDK dependency** (single-binary constraint, and the CLI is the only thing guaranteed present if `docker info` succeeded). Container: `docker run -d --name nanobrain-pg --restart unless-stopped -p 5432:5432 -e POSTGRES_USER=nanobrain -e POSTGRES_PASSWORD=nanobrain -e POSTGRES_DB=nanobrain_dev pgvector/pgvector:pg17` — identical to the documented manual command in SETUP_AGENT.md Step 4.
- **D-07:** Port 5432 conflict handling: if connect to :5432 failed but the port is occupied (container run fails with port-bind error), offer port 5433 and adjust the generated `database.url` accordingly. If a container named `nanobrain-pg` already exists but is stopped, `docker start nanobrain-pg` instead of `docker run`.
- **D-08:** After provisioning, poll readiness by attempting `pgx.Connect` in a loop (up to ~30s, 500ms interval) — not `docker exec pg_isready` (connect-poll validates the actual URL the config will use, including auth).
- **D-09:** Any user-entered Postgres URL is validated with a live connect before writing config. On failure: show the error, re-prompt (allow "save anyway" escape hatch with a warning so an intentionally-offline setup isn't blocked).
- **D-10:** Migrations stay auto-run at server start (existing `RunMigrations` in `startServer`) — the wizard does NOT run migrations itself. No new migration path.

### Embeddings: optional, any Ollama-compatible URL
- **D-11:** New first question: "Enable semantic embeddings? [Y/n]". **No** → write `embedding:\n  provider: ""` — the server already degrades gracefully to BM25-only (`main.go:490-504` guards on `Provider != ""`); print a one-line note ("BM25 keyword search only — re-run `nano-brain init` to enable embeddings later").
- **D-12:** **Yes** → auto-detect local Ollama (`detectOllama`, existing). Found → confirm defaults (URL + `nomic-embed-text`). Not found → prompt for provider: `ollama` (any Ollama-compatible URL, local or cloud — the existing `NewOllamaEmbedder(url, model, dim)` already takes an arbitrary URL) or `voyage`. Do NOT build a new OpenAI-compatible embedding provider in this phase — that is REQ-INFRA-01 / issue #412, separate work. "Any URL" here means any **Ollama-API-compatible** endpoint.
- **D-13:** `doctor` must stop treating missing embeddings as failure when disabled: `CheckEmbeddingProvider`/`CheckEmbeddingModel` return `skip` (detail: "disabled — BM25-only") when `cfg.Embedding.Provider == ""` instead of defaulting to ollama and FAILing. This is the only doctor behavior change; when a provider IS configured, checks behave exactly as today.

### Chain to running state (config → doctor → serve → register → MCP)
- **D-14:** After config write + doctor: ask "Start nano-brain server now? [Y/n]" → reuse `runServeDaemon(configPath)` + `waitForServerHealthy` (existing, `client_helpers.go:88`). If a server is already running (PID check / health OK), skip with a "already running" note. If doctor reported a FAIL on PostgreSQL, do not attempt server start — print what to fix and exit non-zero.
- **D-15:** After server healthy: ask "Register this directory as a workspace? [<cwd>]" (Enter = cwd, or type another path, empty-ish "n" to skip). Registration reuses the existing `--root` code path logic (POST `/api/v1/init`, `triggerInitBackground`) — refactor `runInitCmd`'s registration body into a callable helper rather than duplicating it.
- **D-16:** MCP client step reuses Phase 10's `promptMCPClientConfig` verbatim (Claude Code / OpenCode / Codex CLI, per-client Y/N, idempotent merge writers). "Show the list of supported tools and let the user choose" is exactly what it does. No new clients added in this phase.
- **D-17:** Final output: a short summary block — server URL, workspace name/hash, which MCP clients were configured, and the single next action ("restart your AI client"). This replaces today's dead-end ("start the server and run init --root ...").

### Docs
- **D-18:** Rewrite `docs/SETUP_AGENT.md` around the new flow: prerequisites check (Node/Docker/optionally Ollama) → `npm install -g` → `nano-brain init` → verify. The per-step manual instructions move to a "Manual setup / troubleshooting" appendix (they remain valid for VPS/team path). Update README's Start section to `npm install -g @nano-step/nano-brain && nano-brain init`.

### Claude's Discretion
- Exact prompt wording, section headers, and summary formatting.
- Internal decomposition of `runInteractiveInit` (it will need splitting into testable steps — follow existing `promptWithDefault`/`promptConsequential` + injected `promptReader`/`isTTYFn` test-hook patterns).
- Whether Docker detection uses `docker info` or `docker version` (whichever is cheaper/more reliable).
- Timeout values within the ranges noted above.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `runInteractiveInit` (`cmd/nano-brain/init.go:26-253`) — the wizard to upgrade; already writes YAML config with preview + confirm, then runs doctor.
- `promptWithDefault` (`init.go:255`), `promptConsequential` (`mcp_client_config.go:349`) — prompt helpers; `promptReader`/`promptWriter`/`isTTYFn` test hooks (`client.go:18-29`).
- `doctor.CheckPostgreSQL`, `CheckPgvector`, `CheckEmbeddingProvider`, `CheckEmbeddingModel`, `CheckServerRunning` (`internal/health/doctor/doctor.go`) — modular, individually callable checks for progressive validation inside the wizard.
- `runServeDaemon` (`daemon.go:65-141`, Unix-only `//go:build !windows`), `readPID`/`isRunning`, `waitForServerHealthy` (`client_helpers.go:88-118`).
- `promptMCPClientConfig` (`mcp_client_config.go:244-279`) + idempotent JSON/TOML merge writers for Claude Code (`.mcp.json`), OpenCode (`opencode.json`), Codex (`~/.codex/config.toml`).
- `detectOllama` (`init.go:16`), `detectOpenCodeStorageDir`/`detectClaudeCodeStorageDir` (`detect.go`).
- Registration path: `runInitCmd` `--root` branch (`init.go`/`commands.go`) — POST `/api/v1/init`, returns `WorkspaceHash`/`Name`/`AgentsSnippet`, then `triggerInitBackground`.
- Server auto-runs goose migrations at start (`main.go:295`, `storage/migrate.go`) — first-start DB schema is zero-touch.

### Established Patterns
- No TUI library — plain `bufio.Scanner` (verified: no bubbletea/survey in go.mod). Keep it that way.
- Graceful embedding degrade already exists server-side (`main.go:490-504`): provider `""` or init failure → nil queue, BM25-only search, no crash.
- Config: koanf, defaults in `internal/config/defaults.go`, file at `~/.nano-brain/config.yml` (0600), `NANO_BRAIN_*` env overrides. Wizard writes whole-file YAML (acceptable — it's the config owner).
- CLI tests: mock `isTTYFn`, inject `promptReader`, `httptest.Server` for API calls (`commands_test.go`). Daemon code Unix-gated.

### Integration Points
- `runInteractiveInit` body — restructure into: config-exists gate → DB step → embedding step → advanced gate → write+doctor → serve step → register step → MCP step (reusing Phase 10).
- `doctor.CheckEmbeddingProvider`/`CheckEmbeddingModel` — add disabled-provider skip path.
- `runInitCmd` — extract registration logic into a helper shared by the `--root` branch and the wizard.
- `docs/SETUP_AGENT.md`, `README.md` — new flow docs.

</code_context>

<specifics>
## Specific Ideas

- The exact Docker command must match the documented one (`nanobrain-pg`, `pgvector/pgvector:pg17`, `restart unless-stopped`, nanobrain/nanobrain/nanobrain_dev) so existing docs, doctor hints, and users' mental models stay consistent.
- "One command" is the acceptance bar: fresh machine with Docker → `npm i -g @nano-step/nano-brain && nano-brain init` → answer a handful of prompts → MCP tools respond in Claude Code after client restart.
- Windows: daemon start is Unix-only today; on Windows the serve step should print the manual `nano-brain serve` instruction instead of failing (guard with build tag or runtime check). Do not build Windows daemon support in this phase.

</specifics>

<deferred>
## Deferred Ideas

- Embedded/zero-Docker database (PGlite/SQLite-vec) — architectural change, out of scope; Docker auto-provision chosen for this phase (user decision).
- OpenAI-compatible embedding provider — existing REQ-INFRA-01 / issue #412, separate phase.
- Auto-pull of the Ollama embed model (`ollama pull nomic-embed-text`) from the wizard — nice-to-have; this phase only checks and hints.
- Non-interactive one-shot flag (e.g. `init --yes` full-auto) — future; this phase is the interactive path.
- `curl | bash` distribution installer — explicitly deferred since Phase 10.

</deferred>
