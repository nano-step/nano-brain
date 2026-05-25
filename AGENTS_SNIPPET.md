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
