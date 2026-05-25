## Why

nano-brain accumulates memories without intelligent deduplication — writing "Project uses PostgreSQL" today and "We switched to PostgreSQL for the database" tomorrow creates two separate documents that dilute search relevance. Session transcripts contain valuable facts buried in conversational noise, but these insights are never extracted into searchable, discrete memories. Competitive systems like Mem0 achieve superior recall by using LLM-driven consolidation (ADD/UPDATE/DELETE decisions) and automatic fact extraction. Phase 2 adds these two capabilities as opt-in features that leverage existing LLM infrastructure.

## What Changes

- **LLM-driven memory consolidation**: When `memory_write` is called with consolidation enabled, use LLM to compare new content against similar existing memories (found via embedding search). LLM returns structured decision: ADD (new memory), UPDATE (merge with existing), DELETE (contradicts/supersedes existing), or NOOP (already known). Consolidation runs asynchronously — write returns immediately, consolidation happens in background.
- **Automatic fact extraction from sessions**: During `harvest` command, use LLM to extract discrete facts from session transcripts. Facts are stored as separate memory documents with tag `auto:extracted-fact`, linking back to source session. Extraction is idempotent via content hash — re-harvesting doesn't duplicate facts.
- **Config additions**: New `consolidation` and `extraction` sections in config with provider/model/enabled flags.
- **Background processing**: Both features run async to avoid blocking MCP tool responses.

## Capabilities

### New Capabilities
- `memory-consolidation`: LLM-driven ADD/UPDATE/DELETE/NOOP memory management when writing new memories
- `fact-extraction`: Automatic extraction of discrete facts from session transcripts during harvest

### Modified Capabilities
- `mcp-server`: `memory_write` gains optional consolidation behavior; `harvest` command gains fact extraction
- `storage-limits`: Extracted facts count toward document limits and storage quotas

## Impact

- **Config** (`config.yml`): New `consolidation` and `extraction` sections with provider, model, enabled, maxCandidates/maxFactsPerSession fields
- **Store** (`store.ts`): No schema changes — reuses existing `superseded_by`, `document_tags`, and document insertion
- **Harvester** (`harvester.ts`): Fact extraction integration after session markdown generation
- **Server** (`server.ts`): `memory_write` handler gains async consolidation trigger
- **New module**: `consolidation.ts` for LLM consolidation logic and prompts
- **New module**: `extraction.ts` for LLM fact extraction logic and prompts
- **LLM infrastructure**: Reuses existing Ollama/OpenAI-compatible providers from `embeddings.ts` patterns
- **No new dependencies**: Uses existing node-llama-cpp, Ollama API, or OpenAI-compatible endpoints
