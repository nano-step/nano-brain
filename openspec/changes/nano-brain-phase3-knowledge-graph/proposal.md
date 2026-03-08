## Why

nano-brain stores memories as isolated documents without understanding relationships between entities (tools, services, decisions, people). Competitive memory systems (Mem0, memU) achieve higher accuracy through entity-relationship graphs, proactive memory surfacing, and temporal reasoning. Phase 3 adds knowledge graph capabilities that transform flat memory storage into an interconnected knowledge base with automatic relationship discovery, proactive context suggestions, and timeline-aware fact tracking.

## What Changes

- **Entity-Relationship Memory Graph**: Extract entities (tools, services, people, concepts, decisions, files, libraries) and relationships from memories using LLM. Store in SQLite graph tables (`memory_entities`, `memory_edges`) following the existing `code_symbols`/`symbol_edges` pattern. Support graph traversal queries via BFS with depth limits.
- **Proactive Memory Surfacing**: Automatically surface related past memories when `code_detect_changes` finds changed symbols or when `memory_write` adds new content. Lightweight vector search (no LLM call) returns top related memories as supplementary context.
- **Temporal Reasoning**: Track when facts were first learned and last confirmed. Detect contradictions when new memories conflict with existing ones (integrates with Phase 2 consolidation). Provide timeline views showing how knowledge about a topic evolved over time.
- **New MCP Tools**: `memory_graph_query` (traverse entity relationships), `memory_related` (find related memories across collections), `memory_timeline` (show knowledge evolution over time).

## Capabilities

### New Capabilities
- `memory-entity-graph`: Entity extraction from memories, relationship storage in SQLite graph, BFS traversal with depth limits, entity deduplication by normalized name + type
- `proactive-surfacing`: Automatic related memory suggestions on write and change detection, configurable max suggestions, lightweight vector-based matching
- `temporal-reasoning`: First-learned/last-confirmed timestamps on extracted facts, contradiction detection integrated with Phase 2 consolidation, chronological timeline output

### Modified Capabilities
- `mcp-server`: New tools `memory_graph_query`, `memory_related`, `memory_timeline` added to MCP interface

## Impact

- **Schema**: New tables `memory_entities` and `memory_edges` (separate from code_symbols to avoid confusion). New columns for temporal metadata on extracted facts.
- **LLM Integration**: Entity extraction reuses Phase 2 LLM infrastructure (same provider/model config, different prompts)
- **Store** (`store.ts`): New graph storage and traversal methods following `SymbolGraph` pattern
- **MCP Server** (`server.ts`): Three new tool handlers for graph query, related memories, and timeline
- **Config** (`config.yml`): New `proactive` section with `enabled`, `maxSuggestions` fields; entity extraction settings under existing `llm` section
- **Dependencies**: No new external dependencies. SQLite-only graph storage (no Neo4j).
- **Prerequisites**: Phase 1 (memory decay, auto-categorization) and Phase 2 (LLM consolidation, fact extraction) must be implemented first
