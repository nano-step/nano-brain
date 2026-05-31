<!-- OPENCODE-MEMORY:START -->
<!-- Managed block - do not edit manually. Updated by: npx nano-brain init -->

## Memory System (nano-brain)

This project uses **nano-brain** for persistent context across sessions.

> **Container setup required:** Each agent container must install the wrapper script to avoid
> SQLite conflicts. See your project's nano-brain setup guide.

### Quick Reference

All commands use HTTP API (nano-brain runs as Docker service on port 3100):

> **Container agents:** server is on the HOST — always use `http://host.docker.internal:3100` inside containers. `localhost:3100` only works on the host itself.

| I want to... | Command |
|--------------|---------|
| Recall past work on a topic | `curl -s http://host.docker.internal:3100/api/query -d '{"query":"topic"}'` |
| Find exact error/function name | `curl -s http://host.docker.internal:3100/api/search -d '{"query":"exact term"}'` |
| Explore a concept semantically | `curl -s http://host.docker.internal:3100/api/query -d '{"query":"concept"}'` |
| Save a decision for future sessions | `curl -s http://host.docker.internal:3100/api/write -d '{"content":"...","tags":"decision"}'` |
| Check index health | `curl -s http://host.docker.internal:3100/api/status` |
| Write a note with tags | `curl -s http://host.docker.internal:3100/api/write -d '{"content":"...","tags":"decision,auth"}'` |
| Supersede old info | `curl -s http://host.docker.internal:3100/api/write -d '{"content":"new info","supersedes":"<path>"}'` |
| See file dependencies | Use MCP tool: `memory_focus` with `{"filePath":"src/server.ts"}` |
| Find cross-repo Redis usage | Use MCP tool: `memory_symbols` with `{"type":"redis_key","pattern":"sinv:*"}` |
| Analyze cross-repo impact | Use MCP tool: `memory_impact` with `{"type":"redis_key","pattern":"sinv:*:compressed"}` |
| Search across all workspaces | `curl -s http://host.docker.internal:3100/api/query -d '{"query":"topic","scope":"all"}'` |
| Filter by tags | `curl -s http://host.docker.internal:3100/api/query -d '{"query":"topic","tags":"decision"}'` |

### Session Workflow

**Start of session:** Check memory for relevant past context before exploring the codebase.
```
curl -s http://host.docker.internal:3100/api/query -d '{"query":"what have we done regarding {current task topic}"}'
```

**End of session:** Save key decisions, patterns discovered, and debugging insights.
```bash
curl -s http://host.docker.internal:3100/api/write -d '{"content":"## Summary\n- Decision: ...\n- Why: ...\n- Files: ...","tags":"summary"}'
```

### When to Search Memory vs Codebase

- **"Have we done this before?"** → `curl -s http://host.docker.internal:3100/api/query` (searches past sessions)
- **"Where is this in the code?"** → grep / ast-grep (searches current files)
- **"How does this concept work here?"** → Both (memory for past context + grep for current code)

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




## Git Worktree Rules (MANDATORY)

**All worktrees MUST live inside the repo, under `.opencode/worktrees/`.**

Why: keeps worktree state co-located with the repo, avoids polluting the parent directory, and the path is already gitignored.

```bash
# CORRECT — worktree inside the repo
git worktree add .opencode/worktrees/feat-NNN-short-name b-main

# WRONG — pollutes parent dir, hard to track
git worktree add ../nano-brain-foo b-main
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

## Development Workflow

### OpenSpec-First (MANDATORY)

Features, fixes, and refactors touching multiple files go through OpenSpec before coding.

1. **Propose** → `openspec new change "<name>"` → proposal.md, design.md, specs, tasks.md
2. **Validate** → `openspec validate "<name>" --strict --no-interactive`
3. **Implement** → `/opsx-apply` or work through tasks.md
4. **Archive** → `openspec archive "<name>"` after merge

Skip only for: typo fixes, dependency bumps, single-line config changes.

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
3. Tiny → patch direct. Normal/high-risk → `/opsx-propose` for OpenSpec proposal
4. Run deep-design gap analysis (Metis + Oracle) → revise until clean pass
5. Implement → run validation ladder → user-flow test (if required)
6. Review gate → PR → bot review loop → merge → `openspec archive`

### Gate lifecycle

```
① PRE-WORK → ② IN-PROGRESS → ③ PRE-MERGE → ④ POST-MERGE → ⑤ NEXT-READY → ⑥ RETRO-GATE
```

- All gates must PASS before proceeding. FAIL = BLOCK.
- Agent MUST NOT start next feature until ⑤ NEXT-READY passes.
- Run via: `./scripts/harness-check.sh <phase>`

### Key forbidden practices

- **No `_ = err` on constructor calls in startup paths.** Use `log.Warn` (optional) or `log.Fatal` (critical).
- **No claiming "tests pass" without pasting output.**
- **No self-review.** Implementing agent must not run its own Review Gate.
- **No starting work without a GitHub issue.**
- **No archiving without Review Verdict = PASS.**
- **No modifying harness rules without user approval.**
- **No direct commits to `master` or `b-main`.** Always work on a feature branch (`feat/`, `fix/`, `chore/`, `docs/`) and open a PR. The only exception is a merge commit produced by resolving an existing PR's conflicts — and even then the resolution should normally happen on the PR's head branch, not on the target.
- **No `git push origin <branch>` without first verifying you are ON `<branch>`.** Always run `git branch --show-current` (or check `git status` header) before pushing. Pushing while on the wrong branch silently returns "Everything up-to-date" without error. Use `git push` (no args, relies on upstream tracking) when in doubt.
- **No merging trunk-into-trunk locally.** If a PR's base needs to absorb its head (e.g. `b-main → master`), let the GitHub merge button handle it after conflicts are resolved on the PR head. Local `git merge origin/<other-trunk>` followed by `push origin <this-trunk>` bypasses CI gates and PR review history.

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
| `master` / `b-main` push, PR | `.github/workflows/ci.yml` | `go build` + `go test -race -short` against ephemeral PG service |

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
