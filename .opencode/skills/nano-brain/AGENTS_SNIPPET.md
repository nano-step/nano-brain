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

### Session Workflow

**Start of session:** Resolve workspace once, then wake up + query the topic.

```
memory_workspaces_resolve(path="<project root>")  // → workspace hash
memory_wake_up(workspace=<hash>, limit=8)
memory_query(workspace=<hash>, query="<task topic>")
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
| Trace forward call chain from an entry point | `memory_trace` |
| Find a symbol by name/kind across the workspace | `memory_symbols` |

### When to Search Memory vs Codebase vs Code Intelligence

- **"Have we done this before?"** → `memory_query` (searches past sessions + decisions)
- **"Where is this in the code?"** → grep / ast-grep (searches current files)
- **"How does this concept work here?"** → both (memory for past context + grep for current code)
- **"What calls this function?"** → `memory_graph(node="<name>", direction="in")`
- **"What breaks if I change X?"** → `memory_impact(node="<name>", max_depth=2)`
- **"Walk the call chain from entry point X"** → `memory_trace(node="<name>", max_depth=5)`

<!-- OPENCODE-MEMORY:END -->
