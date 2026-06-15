## Why

nano-brain agents currently see a flat graph — nodes and edges without structure, trust signals, or priority ranking. This limits agent decision-making: they can't tell which edges are extracted from code vs inferred by heuristic, can't identify architectural hotspots, and can't surface hidden cross-module coupling.

Adding community detection, confidence tags, god nodes, and surprise scoring transforms the graph from a flat edge list into an *architecture* — structured, trustworthy, with clear priority signals.

**Reference:** Graphify (github.com/safishamsi/graphify, 65k+ stars) validates these patterns at scale. We adopt the same approach, adapted for Go + PostgreSQL.

## What Changes

- **Confidence tags on edges**: Every edge gets an `EXTRACTED`, `INFERRED`, or `AMBIGUOUS` label. Agents can weight trust accordingly.
- **Leiden community detection**: Using `gonum.org/v1/gonum/graph/community`, detect natural groupings in the codebase. Expose community IDs per symbol.
- **God node ranking**: Identify the most-connected real entities (filtering file hubs, built-in noise, concept nodes). Agents know the architectural linchpins.
- **Surprise scoring**: Composite score (confidence weight + cross file-type + cross-repo + cross-community + peripheral→hub) surfaces hidden coupling and tech debt.
- **Suggested questions**: Auto-generate questions from AMBIGUOUS edges, bridge nodes, isolated functions, low-cohesion communities. Agents know what to ask.
- **MCP tools**: Expose `memory_communities`, `memory_surprises`, `memory_god_nodes` for agent access.

## Capabilities

### New Capabilities

- `community-detection`: Leiden algorithm integration, hub exclusion, cohesion scoring, oversized community splitting
- `confidence-tags`: EXTRACTED/INFERRED/AMBIGUOUS classification on every edge, with source attribution
- `graph-analysis`: God node ranking, surprise scoring, suggested questions, import cycle detection

### Modified Capabilities

- `code-intelligence`: Add community ID to symbol queries, add confidence filtering to graph traversal

## Impact

- `internal/storage/` — new column on `edges` table, new SQL queries for community/analysis
- `internal/graph/` — community detection, god node ranking, surprise scoring
- `internal/mcp/` — new MCP tools (`memory_communities`, `memory_surprises`, `memory_god_nodes`)
- `migrations/` — add `confidence` column to edges, add `community_id` to symbols
- Dependencies — add `gonum.org/v1/gonum` for Leiden algorithm
