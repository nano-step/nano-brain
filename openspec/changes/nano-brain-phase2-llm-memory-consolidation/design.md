## Context

nano-brain currently stores memories without deduplication or intelligent lifecycle management. Phase 1 introduced relevance decay, auto-categorization, and usage-based boosting using heuristics. Phase 2 adds LLM-powered intelligence for two specific use cases:

1. **Memory consolidation**: When new memories are written, compare against existing similar memories and decide whether to add, update, delete, or skip.
2. **Fact extraction**: During session harvesting, extract discrete facts from conversational transcripts.

Both features are opt-in (require LLM configuration) and run asynchronously to avoid blocking MCP tool responses. The existing LLM infrastructure (Ollama API, OpenAI-compatible endpoints) from `embeddings.ts` provides the foundation.

**Current state**:
- `memory_write` inserts documents directly without comparison
- `harvest` converts sessions to markdown but doesn't extract structured facts
- LLM providers exist for embeddings but not for text generation/completion

**Constraints**:
- MCP server runs as background process — blocking operations degrade agent experience
- LLM calls are expensive (latency + cost) — must be optional and batched where possible
- No new runtime dependencies — reuse existing Ollama/OpenAI patterns

## Goals / Non-Goals

**Goals:**
- Enable LLM-driven memory consolidation with ADD/UPDATE/DELETE/NOOP decisions
- Extract discrete, searchable facts from session transcripts during harvest
- Run both features asynchronously without blocking MCP responses
- Provide structured JSON output from LLM for reliable parsing
- Make both features opt-in via config with sensible defaults (disabled)

**Non-Goals:**
- Real-time consolidation (blocking `memory_write` until LLM responds)
- Graph-based memory relationships (Phase 3 consideration)
- Multi-turn LLM conversations for clarification
- Automatic conflict resolution without LLM (heuristic-only consolidation)
- Fact extraction from non-session sources (codebase, external docs)

## Decisions

### Decision 1: Background consolidation queue

**Choice**: Consolidation runs in a background queue, not inline with `memory_write`.

**Rationale**: LLM calls take 500ms-5s depending on model and provider. Blocking `memory_write` would make the MCP tool unusable. A background queue allows immediate response while consolidation happens asynchronously.

**Alternatives considered**:
- Inline consolidation: Rejected due to latency impact on MCP tools
- Scheduled batch consolidation: Rejected because stale memories accumulate between batches

**Implementation**: 
- `memory_write` inserts document immediately, then enqueues consolidation job
- Background worker processes queue, applies LLM decisions
- Queue persists in SQLite to survive restarts

### Decision 2: Structured JSON output from LLM

**Choice**: LLM returns structured JSON with action, reason, and optional mergedContent.

**Rationale**: Structured output enables reliable parsing and logging. The action enum (ADD/UPDATE/DELETE/NOOP) maps directly to store operations.

**Schema**:
```json
{
  "action": "ADD" | "UPDATE" | "DELETE" | "NOOP",
  "reason": "string explaining decision",
  "mergedContent": "string (only for UPDATE)",
  "targetDocId": "number (only for UPDATE/DELETE)"
}
```

**Alternatives considered**:
- Free-form text parsing: Rejected due to unreliable extraction
- Multiple LLM calls (one per decision type): Rejected due to cost/latency

### Decision 3: Embedding-based candidate selection

**Choice**: Use existing vector search to find top-N similar memories as consolidation candidates.

**Rationale**: Reuses existing embedding infrastructure. Limits LLM context to relevant memories only, reducing token usage and improving decision quality.

**Implementation**:
- Before LLM call, search for top `maxCandidates` (default 5) similar documents
- Include candidate snippets in LLM prompt
- LLM decides which candidate (if any) to UPDATE/DELETE

### Decision 4: Fact extraction during harvest, not real-time

**Choice**: Extract facts during `harvest` command, not during session recording.

**Rationale**: Sessions are complete at harvest time, providing full context for extraction. Real-time extraction would require streaming LLM calls and partial context handling.

**Implementation**:
- After `sessionToMarkdown`, pass content to fact extraction LLM
- Store extracted facts as separate documents with tag `auto:extracted-fact`
- Link facts to source session via metadata

### Decision 5: Idempotency via content hash

**Choice**: Use content hash to prevent duplicate fact extraction on re-harvest.

**Rationale**: Users may re-run harvest (e.g., after config change). Without idempotency, facts would duplicate.

**Implementation**:
- Compute hash of extracted fact content
- Check if document with same hash exists before inserting
- Store extraction source session ID in metadata for traceability

### Decision 6: Reuse embedding provider patterns for LLM

**Choice**: Create `LLMProvider` interface mirroring `EmbeddingProvider` patterns.

**Rationale**: Consistent API across providers. Reuses Ollama/OpenAI connection logic.

**Interface**:
```typescript
interface LLMProvider {
  complete(prompt: string, options?: { maxTokens?: number; temperature?: number }): Promise<string>;
  dispose(): void;
}
```

**Providers**:
- `OllamaLLMProvider`: Uses `/api/generate` endpoint
- `OpenAICompatibleLLMProvider`: Uses `/v1/chat/completions` endpoint

## Risks / Trade-offs

**[Risk] LLM hallucination in consolidation decisions** → Mitigation: Log all decisions with full context. Provide `memory_consolidation_log` MCP tool for debugging. Conservative default: prefer ADD over UPDATE/DELETE when uncertain.

**[Risk] Fact extraction produces low-quality facts** → Mitigation: Limit `maxFactsPerSession` (default 20). Use structured prompt with examples. Tag facts for easy filtering/deletion.

**[Risk] Background queue grows unbounded** → Mitigation: Queue size limit with oldest-first eviction. Log warnings when queue exceeds threshold.

**[Risk] LLM provider unavailable** → Mitigation: Graceful degradation — consolidation/extraction silently skipped when LLM unreachable. Log warning, don't fail harvest/write.

**[Trade-off] Async consolidation means temporary duplicates** → Accepted: Brief window where both old and new memory exist. Consolidation resolves within seconds/minutes.

**[Trade-off] Fact extraction increases storage** → Accepted: Facts are small (1-2 sentences each). Storage limits from Phase 1 apply. Users can disable extraction.
