# internal/mcp — MCP Protocol Server

MCP (Model Context Protocol) server — 13 tools for AI agent memory and code intelligence access.

## Transport

- **Streamable HTTP:** `POST /mcp` (spec 2025-03-26, preferred)
- **SSE legacy:** `GET /sse` (subscribe) + `POST /sse` (send messages)
- Keep-alive pings every 30s (`KeepAliveInterval`) to prevent proxy timeouts

## Files

| File | Purpose |
|------|---------|
| `server.go` | `NewMCPServer()` — creates mcp-go `*Server` with name + keep-alive |
| `adapter.go` | `Adapter` — bridges mcp package to storage/search/embed deps |
| `tools.go` | **1206 lines** — all 13 tool definitions + handlers in ONE file |
| `streamable.go` | Streamable HTTP transport handler |
| `sse.go` | SSE transport handler (legacy) |

## Tools (registered in `RegisterTools`)

| Tool | Description |
|------|-------------|
| `memory_query` | Hybrid search — BM25 + vector + RRF + recency decay |
| `memory_search` | BM25 keyword-only search |
| `memory_vsearch` | Vector similarity search |
| `memory_get` | Fetch document by source path |
| `memory_write` | Write or update a document |
| `memory_tags` | List tags with doc counts |
| `memory_status` | Server and embedding queue status |
| `memory_update` | Trigger re-embedding for a document |
| `memory_wake_up` | Workspace briefing (recent activity summary) |
| `memory_symbols` | Code symbol lookup by type/pattern |
| `memory_graph` | Dependency graph traversal |
| `memory_impact` | Cross-repo impact analysis |
| `memory_trace` | Trace symbol usage across workspace |

## tools.go Structure

Each tool is a `register*` function that calls `server.AddTool(...)` with:
- Tool name string
- Description string
- JSON schema (`toolSchema(props, required)`)
- Handler closure capturing `*Adapter`

Shared helpers: `toolSchema`, `textResult`, `errResult`, `argString`, `argInt`, `argStringSlice`, `parseArgs`.

## Adding a New Tool

1. Add a `registerMemoryFoo(server, a)` call in `RegisterTools` (tools.go:21-35)
2. Implement the `register*` function with schema + handler closure
3. No separate files needed — all tools live in tools.go

## Warning

**tools.go is 1206 lines.** Always use targeted `Edit` tool patches. Never rewrite the whole file.
