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
