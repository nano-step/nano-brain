<!-- OPENCODE-MEMORY:START -->
<!-- Managed block - do not edit manually. Updated by: nano-brain skill -->

## Memory System (nano-brain)

This project uses **nano-brain** for persistent context across sessions. Agents talk to the daemon via the registered MCP server `nano-brain` (streamable HTTP at `/mcp`).

### Quick Reference

All operations are MCP tool calls. Every tool takes a `workspace` (SHA-256 hash; resolve once via `memory_workspaces_resolve` with `{path: "<project root>"}`).

| I want to... | MCP tool | Required args |
|---|---|---|
| Recall past work on a topic | `memory_query` | `workspace`, `query` |
| Find exact error/function name | `memory_search` | `workspace`, `query` |
| Explore a concept semantically | `memory_vsearch` | `workspace`, `query` |
| Save a decision for future sessions | `memory_write` | `workspace`, `content` (+ `tags`, `title`) |
| Catch up at session start | `memory_wake_up` | `workspace` |
| Fetch one doc by ID/path | `memory_get` | `workspace`, `path` |
| Check daemon health | `memory_status` | (none) |
| List all registered workspaces | `memory_workspaces_list` | (none) |

### Session Workflow

**Start of session:** List workspaces first, then resolve and query.

```
memory_workspaces_list()                    // → list all workspaces with paths
memory_workspaces_resolve(path="<path>")   // → workspace hash (if you know the path)
memory_wake_up(workspace=<hash>, limit=8)
memory_query(workspace=<hash>, query="<task topic>")
```

**If you don't know the workspace path:**
```
memory_workspaces_list()  // → find workspace by name or path
```

**End of session:** Save key decisions, patterns discovered, and debugging insights.

```
memory_write(
  workspace=<hash>,
  content="## Summary\n- Decision: ...\n- Why: ...\n- Files: ...",
  tags=["summary", "decision"],
  collection="memory"
)
```

### Code Intelligence Tools

Symbol-level analysis (requires the workspace to be indexed by the daemon's watcher — check `memory_status.queue_pending` if results are empty).

| I want to... | MCP tool |
|---|---|
| Find 1-hop callers/callees of a symbol | `memory_graph` |
| Assess risk of changing a symbol (reverse impact BFS) | `memory_impact` |
| Find what a symbol depends on (forward traversal) | `memory_impact` (direction="out") |
| Trace forward call chain from an entry point | `memory_trace` |
| Find a symbol by name/kind across the workspace | `memory_symbols` |

### When to Search Memory vs Codebase vs Code Intelligence

- **"Have we done this before?"** → `memory_query` (searches past sessions + decisions)
- **"Where is this in the code?"** → grep / ast-grep (searches current files)
- **"How does this concept work here?"** → both (memory for past context + grep for current code)
- **"What calls this function?"** → `memory_graph(node="<name>", direction="in")`
- **"What breaks if I change X?"** → `memory_impact(node="<name>", max_depth=2)`
- **"What does X depend on?"** → `memory_impact(node="<name>", direction="out")`
- **"Walk the call chain from entry point X"** → `memory_trace(node="<name>", max_depth=5)`

See `.opencode/skills/nano-brain/SKILL.md` for the full reference (all MCP tools, recipes, troubleshooting).

<!-- OPENCODE-MEMORY:END -->

## RRI-T Test Instance

For RRI-T testing (skill: `rri-t-testing`), use a **separate nano-brain instance on port 8899** to avoid clashing with the default 3100 server (another process in this container uses 3100).

- **Custom config**: `/tmp/nano-brain-custom/config.yml` (port 8899, isolated logs/summaries dir)
- **Launch**:
  ```bash
  NANO_BRAIN_CONFIG=/tmp/nano-brain-custom/config.yml ./nano-brain serve
  ```
- **Health check**: `curl -s http://localhost:8899/api/status`
- **Precedence**: `--config` flag > `NANO_BRAIN_CONFIG` env > `~/.nano-brain/config.yml` (default)

Never run RRI-T against the default 3100 instance — it pollutes production memory and conflicts with the sibling process.

<!-- BEHAVIORAL-GUIDELINES:START -->
# Behavioral Guidelines (Always Apply)

Reduce common LLM coding mistakes. Apply to every task regardless of scope.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them — don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it — don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.

<!-- BEHAVIORAL-GUIDELINES:END -->

## ⛔ CRITICAL: nano-brain Server Rule

**NEVER start nano-brain server inside the container.** The server runs via Docker compose on the HOST only.
- nano-brain server starts ONLY via `npx nano-brain docker start` or `docker compose up -d` in the nano-brain project directory
- Inside containers: use HTTP API (`curl http://host.docker.internal:3100/api/*`) for memory operations — `localhost:3100` does NOT work inside containers
- MCP tools access the server via remote proxy at `http://host.docker.internal:3100/mcp`

## ⚠️ npx nano-brain — Known Caveats

- **NEVER run `npx nano-brain` from the nano-brain source directory.** npm resolves the local `package.json` (name: `nano-brain`) instead of the registry package, causing binary-not-found errors or running stale source code.
- Always run `npx nano-brain` from a **different directory** (e.g., your project root, `/tmp`, home dir).
- The npm package downloads a pre-built Go binary from GitHub Releases — no Go toolchain required on the host.
- Supported platforms: `darwin-arm64`, `darwin-amd64`, `linux-arm64`, `linux-amd64`.




## ⛔ CRITICAL: Testing Isolation (MANDATORY)

**Any test, benchmark, reindex experiment, or throwaway server MUST target the test database and test port — NEVER the dev database (`nanobrain_dev`) or dev server (`:3100`).**

- **Test DB:** `nanobrain_test` · **Test port:** `3199` · config: `config.test.yml`. (See the "Test Database & Isolation" table above.)
- **Run a standalone test/bench server** alongside the running dev server with:
  ```bash
  NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1 NANO_BRAIN_SERVER_PORT=3199 NANO_BRAIN_FLOW_ENABLED=true \
    DATABASE_URL="postgres://nanobrain:nanobrain@localhost:5432/nanobrain_test" ./nano-brain serve
  ```
  (`NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1` bypasses the single-instance guard so it coexists with `:3100`.)
- The capability benchmark bootstraps this automatically: `benchmarks/capability/setup.sh` (clean `nanobrain_test` → migrate → :3199 server → index only this repo). The harness defaults to `http://localhost:3199`.
- **NEVER** run `POST /api/v1/reindex`, `force_wipe`, or destructive ops against the **dev** workspace to set up a test — index into `nanobrain_test` instead.
- **NEVER kill processes with broad `pkill -f`/`lsof | xargs kill` patterns.** They can take down Postgres (the Docker container `nanobrain-pg`) or Docker itself. Capture the exact PID when you launch a server (e.g. `echo $! > /tmp/nb-bench.pid`) and kill only that PID.
- Postgres runs as a **Docker container** (`nanobrain-pg`, image `pgvector/pgvector:pg17`, volume `docker_nanobrain_pgdata`) via `docker compose`. Data survives container restarts; if 5432 is down, bring it back with `docker compose up -d postgres` — do not start a stray brew/native cluster on 5432.

## ⛔ CRITICAL: Privacy — No Real Workspace Names (MANDATORY)

**NEVER commit, push, or publish real workspace names, paths, or hashes.** These are private user data.

**Forbidden in any committed file, PR body, issue body, or public output:**
- Real workspace names (`Phil-timeshel`, `capyhome`, `zengamingx`, etc.)
- Real workspace hashes (SHA-256 from the `workspaces` table)
- Real filesystem paths (`/Users/tamlh/...`, `/home/user/...`, etc.)

**Use generic placeholders instead:**
| Real | Placeholder |
|------|-------------|
| Any private workspace name | `rails-app`, `next-app`, `express-app` |
| Any workspace hash | `PLACEHOLDER_HASH` |
| Any user home path | `/data/workspaces/<generic-name>` |

**Before committing or creating a PR/issue**, grep for known private names:
```bash
grep -rn 'Phil-timeshel\|capyhome\|zengamingx\|/Users/tamlh/workspaces/self/Projects/' --include='*.go' --include='*.md' --include='*.json' --include='*.sh' --include='*.yml' .
```

**Exception:** `nano-brain` itself (`/Users/tamlh/workspaces/self/AI/Tools/nano-brain`) is the open-source project — its path is public.

## Git Worktree Rules (MANDATORY)

**All worktrees MUST live inside the repo, under `.opencode/worktrees/`.**

Why: keeps worktree state co-located with the repo, avoids polluting the parent directory, and the path is already gitignored.

```bash
# CORRECT — worktree inside the repo
git worktree add .opencode/worktrees/feat-NNN-short-name master

# WRONG — pollutes parent dir, hard to track
git worktree add ../nano-brain-foo master
```

After PR merge, clean up:

```bash
git worktree remove .opencode/worktrees/feat-NNN-short-name
git branch -D feat/NNN-short-name   # if local branch still exists
```

If you find an old worktree outside the repo, move it:

```bash
git worktree move ../nano-brain-foo .opencode/worktrees/feat-NNN-short-name
```

`.opencode/worktrees/` is already listed in `.gitignore`. Do not commit anything inside it from the main checkout.

## File Writing Rules (MANDATORY)

**NEVER write an entire file at once.** Always use chunk-by-chunk editing:

1. **Use the Edit tool** (find-and-replace) for all file modifications — insert, update, or delete content in targeted chunks
2. **Only use the Write tool** for brand-new files that don't exist yet, AND only if the file is small (< 50 lines)
3. **For new large files (50+ lines):** Write a skeleton first (headers/structure only), then use Edit to fill in each section chunk by chunk
4. **Why:** Writing entire files at once causes truncation, context window overflow, and silent data loss on large files

**Anti-patterns (NEVER do these):**
- `Write` tool to overwrite an existing file with full content
- `Write` tool to create a file with 100+ lines in one shot
- Regenerating an entire file to change a few lines

## Project Architecture

**Stack:** Go 1.23, PostgreSQL 17 + pgvector 0.8.2, Echo v4, sqlc, goose v3, zerolog, koanf, fsnotify.
**Binary:** `CGO_ENABLED=0` static build. No DI framework. Constructor injection throughout.
**Entry:** `cmd/nano-brain/` — CLI dispatcher + server startup. `internal/` — 17 packages.
**Injection pattern:** config, logger, `*pgxpool.Pool` passed at construction; `sqlc.Queries` wraps the pool.

### Agent-Oriented Design Principles

nano-brain is **built for agents, not humans**. Every design decision optimizes for how agents actually work:

1. **MCP is the primary interface.** Agents call tools, not REST APIs. Every capability is a tool call.
2. **50ms latency target for code intelligence.** Agents skip tools that are slow. `memory_impact`, `memory_trace`, `memory_graph` must be sub-50ms.
3. **Impact analysis is the #1 use case.** "What breaks if I change this?" — the most common agent query. Pre-computed blast radius via reverse BFS.
4. **Call chains > control flow.** Agents trace execution across files (inter-procedural), not within functions (intra-procedural). Optimize `memory_trace` and `memory_graph` over `memory_flowchart`.
5. **Component composition > internal logic.** For frontend frameworks, "who uses this component?" is more valuable than "what does the template do?"
6. **Session continuity matters.** Agents work across sessions. `memory_write` at end of session, `memory_wake_up` + `memory_query` at start.
7. **Structured results, not raw bytes.** Agents don't want 500-line file dumps. They want structured data: symbol names, line numbers, edge lists, blast radius counts.

### Cross-Cutting Conventions

- **Errors:** `fmt.Errorf("<context>: %w", err)` — no custom error types, no bare `errors.New` in callers
- **Logging:** zerolog structured; scope per component via `.With().Str("component","x").Logger()`
- **Context:** `ctx context.Context` first param on all I/O functions; `errgroup` for goroutine lifecycle
- **Interfaces:** small, role-based (Embedder, Querier, Harvester); defined on the consumer side
- **Config:** koanf YAML + env, dynamic reload via `RWMutex`; hot-reload via `POST /api/reload-config`
- **DB:** `storage.NewPool()` → `*pgxpool.Pool` → `sqlc.New(pool)` — generated files are DO NOT EDIT

### Testing

- **Unit:** `package <name>_test`, inline struct mocks (no gomock), table-driven with `t.Run`
- **Integration:** `//go:build integration`, `testutil.SetupTestDB(t)` creates an isolated PG schema per test
- **Quick:** `go build ./... && go test -race -short ./...`
- **Full:** `go test -race -tags=integration ./...`

#### Test database

Integration tests run against **`nanobrain_test`** (not `nanobrain_dev`) to avoid dirtying the dev database.

| What | Value |
|------|-------|
| Database | `nanobrain_test` |
| Default DSN | `postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_test?sslmode=disable` |
| Override | `NANO_BRAIN_TEST_DATABASE_URL=<dsn>` env var |
| Config file | `config.test.yml` in repo root (server port 3199) |
| Server port | **3199** (test) vs 3100 (dev) |

Run integration tests:
```bash
# default — uses nanobrain_test via NANO_BRAIN_TEST_DATABASE_URL or built-in default
go test -race -tags=integration ./...

# explicit override
NANO_BRAIN_TEST_DATABASE_URL="postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_test?sslmode=disable" \
  go test -race -tags=integration ./...

# start a test server instance
NANO_BRAIN_CONFIG=config.test.yml ./nano-brain
```

Each test gets an **isolated schema** (`test_<hash>`) created and dropped automatically by `testutil.SetupTestDB(t)` — tests never share state even when run in parallel.

### Key Directories

| Path | Contents | Child docs |
|------|----------|------------|
| `cmd/nano-brain/` | CLI dispatcher + server startup | `cmd/nano-brain/AGENTS.md` |
| `internal/server/handlers/` | 34 HTTP handler files | `internal/server/handlers/AGENTS.md` |
| `internal/storage/` | sqlc codegen + queries + goose migrations | `internal/storage/AGENTS.md` |
| `internal/harvest/` | Session harvesting (OpenCode, Claude Code) | `internal/harvest/AGENTS.md` |
| `internal/search/` | Hybrid search pipeline (BM25 + vector + RRF) | `internal/search/AGENTS.md` |
| `internal/embed/` | Embedding queue + provider adapters | `internal/embed/AGENTS.md` |
| `internal/mcp/` | MCP protocol tool implementations | `internal/mcp/AGENTS.md` |
| `internal/codesummarize/` | Batched LLM code symbol summarization | — |

## Autonomous Delivery Protocol (DEFAULT MODE)

When Tâm gives an implementation/bugfix directive — "fix …", "giải quyết …", "làm cái này/cái kia", or any request to change behavior (not a question) — drive it to **DONE** through the full GSD pipeline. Do not stop at each gate for approval; Tâm invokes rarely on purpose. Decide and proceed.

**Flow per request:**
1. **Locate/create the phase.** Existing backlog/roadmap phase → use it. New work → add a phase first (`/gsd-phase` add, or `/gsd-capture --backlog`).
2. **Run autonomously:** `/gsd-autonomous --only <N>` → discuss→plan→execute, auto-answered. In discuss, **spawn subagents** (`Explore`/scout) to find the root cause, form your own recommendation, and lock the decisions yourself. `workflow.auto_advance: true` is set so phases self-advance.
3. **Independent review:** after execute, spawn a **separate** reviewer agent (`gsd-code-reviewer` or `oh-my-claudecode:code-reviewer`) — the author never self-approves in the same context (R88). Apply the fixes it finds.
4. **Test with evidence:** `go test -race -short ./...` (+ `-tags=integration` when relevant) against **nanobrain_test / :3199** — never the dev DB / :3100, never broad-kill processes. Paste the real output.
5. **Ship:** `/gsd-ship` → branch from `origin/master`, open the PR.
6. **PR comments:** address review comments autonomously, re-test, push.

**Stop to ask Tâm ONLY when** a decision is genuinely his: irreversible, a business/product choice, or a real blocker. Everything else — pick the sensible default, note it in passing, keep going.

**Evidence is mandatory at every claim.** Test output, reviewer report, verification report. No "should work" — show it ran. If tests fail, say so with the output.

Commits and PRs: author `kokorolx`, **no AI footers** (no `Co-Authored-By`, no 🤖).

### Build & Codegen Commands

```bash
CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain   # Build
go test -race -short ./...                                 # Unit tests
go test -race -tags=integration ./...                      # Integration tests
sqlc generate                                              # SQL codegen
make generate-openapi                                      # OpenAPI spec regeneration
```

## Development Workflow

### GSD Core Phase Loop (MANDATORY)

Features, fixes, and refactors touching multiple files go through GSD Core's phase loop before coding.

1. **Discuss** → `/gsd-discuss-phase` — capture implementation decisions
2. **Plan** → `/gsd-plan-phase` — research, decompose, verify plans
3. **Execute** → `/gsd-execute-phase` — run plans in parallel waves
4. **Verify** → `/gsd-verify-work` — check execution matches plans
5. **Ship** → `/gsd-ship` — create PR, archive phase

Skip only for: typo fixes, dependency bumps, single-line config changes (use `/gsd-quick` or `/gsd-fast`).

## Harness Enforcement Default

Every code change follows the Engineering Harness below by default — issue first, lane classification, validate:quick minimum, feature branch, PR. This applies even to fixes discovered mid-task; finding a bug does not exempt it from intake.

**Bypass:** only when Tâm explicitly says to skip it (e.g. "do this without harness", "skip the issue"). Absent that instruction, assume strict harness applies.

<!-- HARNESS:START -->
<!-- Managed block - do not edit manually. Updated by: harness-init skill -->

## Engineering Harness

This project uses an engineering harness for risk-classified, spec-driven development.

**Full spec:** [`docs/HARNESS.md`](docs/HARNESS.md) | **Gates:** [`docs/HARNESS_GATES.md`](docs/HARNESS_GATES.md)

### Quick reference

| Document | Purpose |
|---|---|
| [`docs/HARNESS.md`](docs/HARNESS.md) | Full process — lanes, gates, validation ladder |
| [`docs/FEATURE_INTAKE.md`](docs/FEATURE_INTAKE.md) | Risk classification (tiny / normal / high-risk) |
| [`docs/templates/story.md`](docs/templates/story.md) | Story template for new work |
| [`docs/HARNESS_BACKLOG.md`](docs/HARNESS_BACKLOG.md) | Project-specific friction backlog |
| [`docs/evidence/`](docs/evidence/) | Screenshots, recordings, decision logs |

### Lanes

- **tiny** — 0-1 risk flags, direct patch
- **normal** — 2-3 risk flags, proposal required
- **high-risk** — 4+ flags OR any hard gate (auth, data-model, search-quality, embedding/vector provider, public-api-contract, audit-security, authorization, external-provider)

### Validation ladder

| Layer | Command | Required for |
|---|---|---|
| `validate:quick` | `go build ./... && go test -race -short ./...` | every lane |
| `self-review:response-shape` | Read struct + mapping loop, verify all fields assigned | user-feature only |
| `self-review:staged-files` | `git status` before every commit — no `.opencode/`, no `package-lock.json` | every lane |
| `test:integration` | `go test -race -tags=integration ./...` | normal + high-risk |
| `smoke:e2e` | Build binary → start server → curl endpoints → verify | normal + high-risk (user-feature/bug-fix) |
| `test:release` | `./nano-brain status` | before deploy |

### Change types

| Type | smoke:e2e | Review gate |
|---|:-:|:-:|
| user-feature | ✅ | ✅ |
| bug-fix | ✅ | ✅ |
| infrastructure | ❌ | ⚠️ self-verify |
| refactor | ❌ | ⚠️ self-verify |
| docs | ❌ | ❌ |
| dependency-bump | ❌ | ⚠️ self-verify |

### Flow

1. Create GitHub issue (`gh issue create --repo nano-step/nano-brain`) **before** classification
2. Read `docs/FEATURE_INTAKE.md` → classify lane + change type → label issue
3. Tiny → patch direct. Normal/high-risk → `/gsd-discuss-phase` to capture decisions
4. Run `/gsd-plan-phase` → research, decompose, verify plans
5. Run `/gsd-execute-phase` → execute plans in parallel waves
6. Run `/gsd-verify-work` → verify execution matches plans
7. Run `/gsd-ship` → create PR, archive phase

### Gate lifecycle

```
① PRE-WORK → ② IN-PROGRESS → ③ PRE-MERGE → ③.5 ASYNC-PR-REVIEW → ④ POST-MERGE → ④.5 POST-MERGE-NPM-RELEASE → ⑤ NEXT-READY → ⑥ RETRO-GATE
```

- All gates must PASS before proceeding. FAIL = BLOCK.
- Agent MUST NOT start next feature until ⑤ NEXT-READY passes.
- Gates ③.5 (async-pr-review) and ④.5 (post-merge-npm-release) are async — the harness loop spawns a background watcher that polls until terminal status.
- Run via: `./scripts/harness-check.sh <phase>`

### Key forbidden practices

- **No `_ = err` on constructor calls in startup paths.** Use `log.Warn` (optional) or `log.Fatal` (critical).
- **No claiming "tests pass" without pasting output.**
- **No self-review.** Implementing agent must not run its own Review Gate.
- **No starting work without a GitHub issue.**
- **No archiving without Review Verdict = PASS.**
- **No modifying harness rules without user approval.**
- **No using `nanobrain_dev` for harness testing.** All smoke:e2e, test:integration, and harness validation MUST use `nanobrain_test` database. The dev database (`nanobrain_dev`) is for the running production server only — test workspaces pollute it and create stale data. Use `NANO_BRAIN_DATABASE_URL="postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_test?sslmode=disable"` or `config.test.yml`.
- **No direct commits to `master`.** Always work on a feature branch (`feat/`, `fix/`, `chore/`, `docs/`) and open a PR. The only exception is a merge commit produced by resolving an existing PR's conflicts — and even then the resolution should normally happen on the PR's head branch, not on the target.
- **No `git push origin <branch>` without first verifying you are ON `<branch>`.** Always run `git branch --show-current` (or check `git status` header) before pushing. Pushing while on the wrong branch silently returns "Everything up-to-date" without error. Use `git push` (no args, relies on upstream tracking) when in doubt.
- **Single-trunk model: `master` only.** All feature branches branch from `master` and PR back to `master`. The `b-main` staging branch was retired on 2026-06-01 — no more `b-main → master` promotion step. Every merge to `master` triggers `auto-tag.yml` → `release.yml` → npm publish.

### Git push workflow (container environment)

`origin` is configured to HTTPS (`https://github.com/nano-step/nano-brain.git`) for both fetch and push — no SSH key required. Push uses the gh credential helper to inject the active token automatically.

```bash
# Step 1 — make sure kokorolx is the active gh user (has write access to nano-step/nano-brain)
gh auth status              # confirm "✓ Logged in to github.com as kokorolx"
gh auth switch --user kokorolx   # only if currently on nus-rick

# Step 2 — push normally; gh credential helper handles auth
git push origin <branch>
git push origin <tag>

# Step 3 — close GitHub issues
gh issue close <number> --repo nano-step/nano-brain --comment "..."

# Step 4 (optional) — switch back to nus-rick for day-to-day gh CLI use
gh auth switch --user nus-rick
```

**Why:** Container has no SSH key, so `origin` is HTTPS. `kokorolx` has `repo` scope and is the repo owner — required for push + issue close. `nus-rick` is a contributor only. If `git push` ever complains about credentials, fall back to:

```bash
KOKOROLX_TOKEN=$(gh auth token --user kokorolx)
git push "https://kokorolx:${KOKOROLX_TOKEN}@github.com/nano-step/nano-brain.git" <branch>
```

### Release flow

Date-based auto-release pipeline (master push → tag → binaries + npm publish):

| Trigger | Workflow | Effect |
|---|---|---|
| `master` push | `.github/workflows/auto-tag.yml` | Compute next tag `v{YYYY}.{M}.{D}.{N}` (e.g. `v2026.5.30.1`) → push tag via `RELEASE_PAT` |
| `v*` tag push | `.github/workflows/release.yml` | Cross-build 4-platform Go binaries (linux/darwin × amd64/arm64) → create GH Release with binaries → `npm publish --tag latest` both `@nano-step/nano-brain` and `nano-brain` (unscoped alias) |
| PR opened/sync | `.github/workflows/gemini-review.yml` → shared `gemini-review.yml@v1` | Gemini code review comment on PR |
| `master` push, PR | `.github/workflows/ci.yml` | `go build` + `go test -race -short` against ephemeral PG service |

Required repo secrets (set via `gh secret set --repo nano-step/nano-brain`):

| Secret | Used by | Source |
|---|---|---|
| `RELEASE_PAT` | auto-tag | Classic PAT with `repo` scope. Required so the tag push re-triggers release.yml (tags pushed by `GITHUB_TOKEN` do NOT trigger downstream workflows — GitHub anti-recursion guard) |
| `NPM_TOKEN` | release.yml (npm-publish job) | `npm token create --read-only=false` (Automation type) for npmjs.org |
| `GEMINI_API_KEY` | gemini-review | https://aistudio.google.com/apikey |

**Tag scheme:** `v{YYYY}.{M}.{D}.{N}` where `N` is the daily run-number (starts at 1, increments per push). Example: `v2026.5.30.1`, `v2026.5.30.2`. The dot before `N` is mandatory — the prior no-dot scheme (`v2026.5.301`) collided between single-digit and multi-digit days.

**Skip-release markers:** any of these in the commit subject bypass auto-tag:
- `[skip-release]`
- `[skip ci]`
- `chore: bump version` prefix (anti-loop guard for any historic publish-stable-style bump commits)

Bot commits authored as `github-actions[bot]` are also auto-skipped.

**`package.json.version`** stays at `0.0.0-dev` on master. The auto-tag workflow rewrites it in-place from the tag value before `npm publish` — the bump is NEVER committed back to master.

<!-- HARNESS:END -->

## graphify

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.

When the user types `/graphify`, invoke the `skill` tool with `skill: "graphify"` before doing anything else.

Rules:
- For codebase questions, first run `graphify query "<question>"` when graphify-out/graph.json exists. Use `graphify path "<A>" "<B>"` for relationships and `graphify explain "<concept>"` for focused concepts. These return a scoped subgraph, usually much smaller than GRAPH_REPORT.md or raw grep output.
- Dirty graphify-out/ files are expected after hooks or incremental updates; dirty graph files are not a reason to skip graphify. Only skip graphify if the task is about stale or incorrect graph output, or the user explicitly says not to use it.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).
