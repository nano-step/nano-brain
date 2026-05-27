<!-- OPENCODE-MEMORY:START -->
<!-- Managed block - do not edit manually. Updated by: npx nano-brain init -->

## Memory System (nano-brain)

This project uses **nano-brain** for persistent context across sessions.

> **Container setup required:** Each agent container must install the wrapper script to avoid
> SQLite conflicts. See your project's nano-brain setup guide.

### Quick Reference

All commands use HTTP API (nano-brain runs as Docker service on port 3100):

| I want to... | Command |
|--------------|---------|
| Recall past work on a topic | `curl -s localhost:3100/api/query -d '{"query":"topic"}'` |
| Find exact error/function name | `curl -s localhost:3100/api/search -d '{"query":"exact term"}'` |
| Explore a concept semantically | `curl -s localhost:3100/api/query -d '{"query":"concept"}'` |
| Save a decision for future sessions | `curl -s localhost:3100/api/write -d '{"content":"...","tags":"decision"}'` |
| Check index health | `curl -s localhost:3100/api/status` |
| Write a note with tags | `curl -s localhost:3100/api/write -d '{"content":"...","tags":"decision,auth"}'` |
| Supersede old info | `curl -s localhost:3100/api/write -d '{"content":"new info","supersedes":"<path>"}'` |
| See file dependencies | Use MCP tool: `memory_focus` with `{"filePath":"src/server.ts"}` |
| Find cross-repo Redis usage | Use MCP tool: `memory_symbols` with `{"type":"redis_key","pattern":"sinv:*"}` |
| Analyze cross-repo impact | Use MCP tool: `memory_impact` with `{"type":"redis_key","pattern":"sinv:*:compressed"}` |
| Search across all workspaces | `curl -s localhost:3100/api/query -d '{"query":"topic","scope":"all"}'` |
| Filter by tags | `curl -s localhost:3100/api/query -d '{"query":"topic","tags":"decision"}'` |

### Session Workflow

**Start of session:** Check memory for relevant past context before exploring the codebase.
```
curl -s localhost:3100/api/query -d '{"query":"what have we done regarding {current task topic}"}'
```

**End of session:** Save key decisions, patterns discovered, and debugging insights.
```bash
curl -s localhost:3100/api/write -d '{"content":"## Summary\n- Decision: ...\n- Why: ...\n- Files: ...","tags":"summary"}'
```

### When to Search Memory vs Codebase

- **"Have we done this before?"** → `curl -s localhost:3100/api/query` (searches past sessions)
- **"Where is this in the code?"** → grep / ast-grep (searches current files)
- **"How does this concept work here?"** → Both (memory for past context + grep for current code)

<!-- OPENCODE-MEMORY:END -->

## ⛔ CRITICAL: nano-brain Server Rule

**NEVER start nano-brain server inside the container.** The server runs via Docker compose on the HOST only.
- nano-brain server starts ONLY via `npx nano-brain docker start` or `docker compose up -d` in the nano-brain project directory
- Inside containers: use HTTP API (`curl localhost:3100/api/*`) for memory operations
- MCP tools access the server via remote proxy at `http://host.docker.internal:3100/mcp`

## ⚠️ npx nano-brain — Known Caveats

- **NEVER run `npx nano-brain` from the nano-brain source directory.** npm resolves the local `package.json` (name: `nano-brain`) instead of the registry package, causing binary-not-found errors or running stale source code.
- Always run `npx nano-brain` from a **different directory** (e.g., your project root, `/tmp`, home dir).
- The npm package downloads a pre-built Go binary from GitHub Releases — no Go toolchain required on the host.
- Supported platforms: `darwin-arm64`, `darwin-amd64`, `linux-arm64`, `linux-amd64`.




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

## Development Workflow

### OpenSpec-First (MANDATORY)

**Every feature, fix, or refactor MUST go through OpenSpec before implementation.**

1. **Propose** → `openspec new change "<name>"` → create proposal.md, design.md, specs, tasks.md
2. **Validate** → `openspec validate "<name>" --strict --no-interactive`
3. **Implement** → `/opsx-apply` or work through tasks.md
4. **Archive** → `openspec archive "<name>"` after merge

**No exceptions.** Do not skip straight to coding. The proposal captures *why*, the spec captures *what*, the design captures *how*, and tasks capture *the plan*. This applies to:
- New features (even small ones)
- Bug fixes that change behavior
- Refactors that touch multiple files

**Only skip OpenSpec for:** typo fixes, dependency bumps, or single-line config changes.

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

<!-- HARNESS:END -->
