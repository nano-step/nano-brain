## Why

Agents that install nano-brain through MCP do not automatically read README files or project docs. Their primary decision surface is the MCP tool listing: tool names, descriptions, and input schema descriptions. Without guidance there, agents may overuse generic search, skip symbol/impact/flow tools, or ask broad framework questions instead of using the right code-intelligence primitive.

## What Changes

- Add agent-facing tool-selection guidance directly to MCP tool descriptions.
- Make search tools explain when to use hybrid, BM25, or vector search.
- Make code-intelligence tools explain when to use symbols, graph, impact, trace, flow, and flowchart.
- Keep behavior and response schemas unchanged.

## Impact

- `internal/mcp/tools.go` — update descriptions and schema descriptions only.
- `internal/mcp/tools_test.go` or existing MCP tests — assert important guidance appears in tool listings.
- `openspec/specs/mcp-server/spec.md` — document the requirement after implementation/archive.

No API break: tool names, parameters, handlers, and response shapes remain unchanged.
