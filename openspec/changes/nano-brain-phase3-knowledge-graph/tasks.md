## 1. Schema and Database

- [ ] 1.1 Create `memory_entities` table with columns: id, name, type, description, project_hash, first_learned_at, last_confirmed_at, contradicted_at, contradicted_by_memory_id
- [ ] 1.2 Create `memory_edges` table with columns: id, source_id, target_id, edge_type, project_hash, created_at
- [ ] 1.3 Add indexes on memory_entities (name, type, project_hash) and memory_edges (source_id, target_id, edge_type)
- [ ] 1.4 Add UNIQUE constraint on memory_entities (LOWER(name), type, project_hash) for deduplication
- [ ] 1.5 Create migration logic for existing databases (add tables if not exist)

## 2. Entity Extraction

- [ ] 2.1 Define TypeScript interfaces for MemoryEntity and MemoryEdge in types.ts
- [ ] 2.2 Create entity extraction prompt template with entity types (tool, service, person, concept, decision, file, library) and relationship types (uses, depends_on, decided_by, related_to, replaces, configured_with)
- [ ] 2.3 Implement extractEntitiesFromMemory() function using Phase 2 LLM infrastructure
- [ ] 2.4 Add JSON schema validation for LLM extraction output
- [ ] 2.5 Integrate entity extraction into memory_write flow (when consolidation enabled)

## 3. Entity Storage and Deduplication

- [ ] 3.1 Implement insertOrUpdateEntity() with case-insensitive deduplication
- [ ] 3.2 Implement insertEdge() for relationship storage
- [ ] 3.3 Add first_learned_at/last_confirmed_at timestamp management
- [ ] 3.4 Create MemoryGraph class following SymbolGraph pattern
- [ ] 3.5 Add getEntityByName() with optional type filter

## 4. Graph Traversal

- [ ] 4.1 Implement BFS traversal in MemoryGraph.traverse() with configurable depth limit
- [ ] 4.2 Add relationship type filtering to traversal
- [ ] 4.3 Implement getEntityEdges() for incoming/outgoing relationships
- [ ] 4.4 Add depth limit enforcement (max 10, default 3)
- [ ] 4.5 Return results ordered by distance from starting entity

## 5. memory_graph_query MCP Tool

- [ ] 5.1 Define tool schema with parameters: entity (required), maxDepth (optional), relationshipTypes (optional)
- [ ] 5.2 Implement tool handler in server.ts
- [ ] 5.3 Add entity-not-found error handling with similar entity suggestions
- [ ] 5.4 Add parameter validation (maxDepth range 1-10)
- [ ] 5.5 Format response with entity details, connected entities, and edges

## 6. Proactive Surfacing Pipeline

- [ ] 6.1 Add proactive config section: enabled (boolean), maxSuggestions (number, default 3)
- [ ] 6.2 Implement findRelatedMemories() using vector similarity search
- [ ] 6.3 Integrate proactive surfacing into memory_write response
- [ ] 6.4 Integrate proactive surfacing into code_detect_changes response
- [ ] 6.5 Add deduplication for suggestions across multiple changed files

## 7. memory_related MCP Tool

- [ ] 7.1 Define tool schema with parameters: topic (required), collection (optional), limit (optional)
- [ ] 7.2 Implement tool handler using vector search
- [ ] 7.3 Add collection filtering support
- [ ] 7.4 Enforce limit maximum of 10
- [ ] 7.5 Return results ordered by relevance score

## 8. Temporal Metadata Tracking

- [ ] 8.1 Update entity insertion to set first_learned_at and last_confirmed_at
- [ ] 8.2 Update entity re-confirmation to only update last_confirmed_at
- [ ] 8.3 Add contradicted_at and contradicted_by_memory_id fields
- [ ] 8.4 Implement markEntityContradicted() function

## 9. Contradiction Detection

- [ ] 9.1 Extend Phase 2 consolidation prompt to detect contradictions
- [ ] 9.2 Add contradiction confidence scoring (0.0-1.0)
- [ ] 9.3 Integrate contradiction detection into consolidation flow
- [ ] 9.4 Store contradiction metadata on affected entities
- [ ] 9.5 Add graceful degradation when Phase 2 consolidation is disabled

## 10. memory_timeline MCP Tool

- [ ] 10.1 Define tool schema with parameters: topic (required), startDate (optional), endDate (optional)
- [ ] 10.2 Implement getTopicTimeline() to fetch chronological memories
- [ ] 10.3 Add change type indicators (new, updated, contradicted)
- [ ] 10.4 Implement date range filtering
- [ ] 10.5 Format timeline entries with timestamp, summary, and change type

## 11. Testing

- [ ] 11.1 Unit tests for entity extraction prompt and parsing
- [ ] 11.2 Unit tests for entity deduplication logic
- [ ] 11.3 Unit tests for BFS graph traversal
- [ ] 11.4 Unit tests for proactive surfacing
- [ ] 11.5 Unit tests for temporal metadata tracking
- [ ] 11.6 Unit tests for contradiction detection
- [ ] 11.7 Integration tests for memory_graph_query tool
- [ ] 11.8 Integration tests for memory_related tool
- [ ] 11.9 Integration tests for memory_timeline tool
- [ ] 11.10 Performance test for proactive surfacing latency (< 100ms)

## 12. Documentation and Config

- [ ] 12.1 Update config.yml schema with proactive section
- [ ] 12.2 Add entity extraction settings under llm section
- [ ] 12.3 Update AGENTS.md with new MCP tools documentation
- [ ] 12.4 Add Phase 3 prerequisites note (requires Phase 1 and Phase 2)
