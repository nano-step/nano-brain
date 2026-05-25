## Context

nano-brain currently stores memories as isolated documents with vector embeddings and BM25 indexing. Phase 1 adds relevance decay and auto-categorization. Phase 2 adds LLM-powered consolidation and fact extraction. Phase 3 builds on these foundations to create a knowledge graph layer that understands relationships between entities mentioned in memories.

The existing codebase already has a graph infrastructure for code symbols (`code_symbols`, `symbol_edges` tables) with BFS traversal in `SymbolGraph.handleImpact()`. Phase 3 extends this pattern to memory entities while keeping the two graphs separate (code vs. memory entities serve different purposes).

Constraints:
- Zero external dependencies (no Neo4j, no external graph databases)
- Must reuse Phase 2 LLM infrastructure for entity extraction
- Performance-sensitive: proactive surfacing runs on every write
- SQLite-only storage using existing better-sqlite3 setup

## Goals / Non-Goals

**Goals:**
- Extract entities and relationships from memories using LLM
- Store entity graph in SQLite following existing `code_symbols`/`symbol_edges` pattern
- Provide graph traversal queries via `memory_graph_query` MCP tool
- Automatically surface related memories on write and change detection
- Track temporal metadata (first learned, last confirmed) for facts
- Detect contradictions when new memories conflict with existing ones
- Provide timeline views of knowledge evolution

**Non-Goals:**
- External graph database integration (Neo4j, etc.)
- Real-time entity extraction during search (extraction happens at write time)
- Cross-project entity linking (entities are scoped to project_hash)
- Natural language graph queries (structured queries only)
- Automatic contradiction resolution (detection only, user decides)

## Decisions

### Decision 1: Separate tables for memory entities (not extending code_symbols)

**Choice**: Create new `memory_entities` and `memory_edges` tables separate from `code_symbols`/`symbol_edges`.

**Rationale**: Memory entities (tools, services, decisions, people) are conceptually different from code symbols (functions, classes, variables). Mixing them would:
- Complicate queries that need only code or only memory entities
- Create confusion about entity types and edge semantics
- Make it harder to evolve the schemas independently

**Alternatives considered**:
- Extend `code_symbols` with a `source` column: Rejected due to semantic confusion and query complexity
- Single unified entity table: Rejected because code symbols have different metadata (line numbers, exports) than memory entities

### Decision 2: Entity extraction reuses Phase 2 LLM infrastructure

**Choice**: Use the same LLM provider/model configuration from Phase 2 consolidation for entity extraction.

**Rationale**: 
- Consistent configuration (one place to set model, temperature, etc.)
- Reuse existing LLM client code and error handling
- Users already configured LLM for Phase 2

**Implementation**: Entity extraction prompt is separate from consolidation prompt but uses same `llm.provider` and `llm.model` config.

### Decision 3: BFS traversal with configurable depth limit

**Choice**: Graph traversal uses breadth-first search with default depth limit of 3, matching `code_impact` pattern.

**Rationale**:
- BFS ensures closest relationships are found first
- Depth limit prevents runaway queries on densely connected graphs
- Consistent with existing `SymbolGraph.handleImpact()` implementation

**Implementation**: `memory_graph_query` accepts optional `maxDepth` parameter (default 3, max 10).

### Decision 4: Proactive surfacing uses vector search only (no LLM)

**Choice**: Related memory suggestions use lightweight vector similarity search, not LLM-based relevance scoring.

**Rationale**:
- Speed: Vector search is milliseconds, LLM calls are seconds
- Cost: No additional LLM tokens consumed per write
- Sufficient quality: Vector similarity captures semantic relatedness well enough for suggestions

**Implementation**: After `memory_write` or `code_detect_changes`, run vector search against memory collection, return top N results (configurable, default 3).

### Decision 5: Entity deduplication by normalized name + type

**Choice**: Entities are deduplicated by case-insensitive normalized name combined with entity type.

**Rationale**:
- "Redis", "redis", "REDIS" should all map to the same entity
- But "Redis" (tool) and "Redis" (person name) should be different entities
- Simple and predictable matching logic

**Implementation**: `UNIQUE(LOWER(name), type, project_hash)` constraint on `memory_entities` table.

### Decision 6: Temporal metadata on memory_entities, not documents

**Choice**: Store `first_learned_at` and `last_confirmed_at` on extracted entities/facts, not on source documents.

**Rationale**:
- Documents already have `created_at` and `modified_at`
- Temporal reasoning is about facts/entities, not documents
- Same fact can appear in multiple documents; we want the earliest mention

**Implementation**: `memory_entities` table includes `first_learned_at TEXT` and `last_confirmed_at TEXT` columns.

### Decision 7: Contradiction detection integrates with Phase 2 consolidation

**Choice**: Contradiction detection runs during Phase 2 consolidation when UPDATE/DELETE decisions are made.

**Rationale**:
- Consolidation already compares new content with existing memories
- Natural place to detect "this contradicts what we knew before"
- Avoids duplicate LLM calls for comparison

**Implementation**: Consolidation prompt extended to flag contradictions. Contradicted facts get `contradicted_at` timestamp and reference to contradicting memory.

## Risks / Trade-offs

### Risk 1: Entity extraction quality depends on LLM
**Risk**: Poor entity extraction leads to sparse or incorrect graph.
**Mitigation**: 
- Use structured output format (JSON) with clear entity/relationship schema
- Include examples in prompt for common entity types
- Allow manual entity tagging as fallback (future enhancement)

### Risk 2: Graph can grow large over time
**Risk**: Unbounded entity/edge growth impacts query performance.
**Mitigation**:
- Index on `project_hash` for scoped queries
- Depth limits on traversal
- Consider entity pruning based on access patterns (future enhancement)

### Risk 3: Proactive surfacing adds latency to writes
**Risk**: Vector search on every write slows down `memory_write`.
**Mitigation**:
- Vector search is fast (< 50ms for typical collections)
- Make proactive surfacing configurable (`proactive.enabled: false` to disable)
- Return suggestions asynchronously if needed (future enhancement)

### Risk 4: Contradiction detection may have false positives
**Risk**: LLM incorrectly flags non-contradictions as contradictions.
**Mitigation**:
- Detection only, no automatic resolution
- Include confidence score with contradiction flags
- User can dismiss false positives

### Risk 5: Phase 2 dependency creates coupling
**Risk**: Phase 3 features require Phase 2 to be fully implemented.
**Mitigation**:
- Entity extraction can work standalone (without consolidation)
- Temporal reasoning degrades gracefully without consolidation (no contradiction detection)
- Clear prerequisite documentation
