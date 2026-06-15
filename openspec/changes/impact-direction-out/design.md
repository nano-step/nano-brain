# Design: Add direction="out" to memory_impact

**Tracking:** #419

## Current Behavior

`memory_impact` traverses inbound edges only:
- Input: `node` (file/symbol path)
- Returns: nodes that call/import the input node
- Use case: "What depends on this file?" (impact analysis)

## Proposed Behavior

Add `direction` parameter (optional, default: `"in"`):

| Value | Traversal | Use Case |
|---|---|
| `"in"` | Who calls/imports this node | Existing behavior |
| `"out"` | What this node calls/imports | Review new files |
| `"both"` | Union of both directions | Full graph exploration |

## Implementation Plan

### Step 1: Update memory_graph tool

The `memory_impact` tool uses `memory_graph` internally.

**Current `memory_graph` call:**
```go
edges := memory_graph(node, edge_type="contains|imports|calls", direction="in")
```

**New `memory_graph` call:**
```go
// direction: "in" (default) | "out" | "both"
edges := memory_graph(node, edge_type="contains|imports|calls", direction=direction)
```

### Step 2: Update memory_impact MCP handler

**Location:** `internal/mcp/tools/memory_impact.go`

**Current signature:**
```go
func MemoryImpact(workspace string, node string, maxDepth int) ([]string, error)
```

**New signature:**
```go
func MemoryImpact(workspace string, node string, maxDepth int, direction string) ([]string, error)
```

## Design Decisions

1. **Default: `direction="in"`** - Maintains backward compatibility
2. **Return schema unchanged** - Same string array format
3. **Union for "both"** - Combine inbound + outbound, deduplicate
4. **Single MCP call** - No need for new tool; extend existing

## Edge Cases

1. **Node has no edges in one direction** - Return empty array for that direction
2. **Circular dependencies** - Already handled by existing maxDepth logic
3. **Very deep traversal** - maxDepth applies per direction; "both" may return more results
