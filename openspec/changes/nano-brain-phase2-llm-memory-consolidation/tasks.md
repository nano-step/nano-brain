## 1. LLM Provider Infrastructure

- [ ] 1.1 Create `src/llm.ts` with `LLMProvider` interface (complete, dispose methods)
- [ ] 1.2 Implement `OllamaLLMProvider` using `/api/generate` endpoint
- [ ] 1.3 Implement `OpenAICompatibleLLMProvider` using `/v1/chat/completions` endpoint
- [ ] 1.4 Add `createLLMProvider` factory function with config-based provider selection
- [ ] 1.5 Add LLM provider health check functions (similar to embedding health checks)

## 2. Configuration Schema

- [ ] 2.1 Add `ConsolidationConfig` type to `src/types.ts` (enabled, provider, model, url, apiKey, maxCandidates)
- [ ] 2.2 Add `ExtractionConfig` type to `src/types.ts` (enabled, provider, model, url, apiKey, maxFactsPerSession)
- [ ] 2.3 Update `CollectionConfig` to include optional `consolidation` and `extraction` sections
- [ ] 2.4 Add config validation for consolidation and extraction sections

## 3. Consolidation Queue

- [ ] 3.1 Add `consolidation_queue` table schema (id, document_id, status, created_at, processed_at, result)
- [ ] 3.2 Add `consolidation_log` table schema (id, document_id, action, reason, target_doc_id, model, created_at)
- [ ] 3.3 Implement queue insertion function in `src/store.ts`
- [ ] 3.4 Implement queue polling function (get next pending job)
- [ ] 3.5 Implement queue status update function (pending → processing → completed/failed)

## 4. Consolidation Prompt Design

- [ ] 4.1 Create `src/prompts/consolidation.ts` with consolidation system prompt
- [ ] 4.2 Define JSON schema for consolidation response (action, reason, mergedContent, targetDocId)
- [ ] 4.3 Create prompt builder function that includes new memory and candidate snippets
- [ ] 4.4 Add prompt examples for each action type (ADD, UPDATE, DELETE, NOOP)

## 5. Consolidation Pipeline

- [ ] 5.1 Create `src/consolidation.ts` module
- [ ] 5.2 Implement `findConsolidationCandidates` using vector search
- [ ] 5.3 Implement `buildConsolidationPrompt` with new memory and candidates
- [ ] 5.4 Implement `parseConsolidationResponse` with JSON validation
- [ ] 5.5 Implement `applyConsolidationDecision` (ADD/UPDATE/DELETE/NOOP handlers)
- [ ] 5.6 Implement `processConsolidationJob` orchestrating the full pipeline
- [ ] 5.7 Add consolidation logging to `consolidation_log` table

## 6. Background Consolidation Worker

- [ ] 6.1 Create `src/consolidation-worker.ts` with background processing loop
- [ ] 6.2 Implement worker start/stop lifecycle management
- [ ] 6.3 Add configurable polling interval (default 1 second)
- [ ] 6.4 Integrate worker startup into MCP server initialization
- [ ] 6.5 Handle graceful shutdown (complete current job, then stop)

## 7. MCP Server Integration (Consolidation)

- [ ] 7.1 Modify `memory_write` handler to enqueue consolidation job when enabled
- [ ] 7.2 Add `consolidation: "pending"` to `memory_write` response when applicable
- [ ] 7.3 Implement `memory_consolidation_status` MCP tool
- [ ] 7.4 Register `memory_consolidation_status` in tool list

## 8. Fact Extraction Prompt Design

- [ ] 8.1 Create `src/prompts/extraction.ts` with extraction system prompt
- [ ] 8.2 Define JSON schema for extraction response (array of {content, category})
- [ ] 8.3 Create prompt builder function for session transcript input
- [ ] 8.4 Add extraction categories: architecture-decision, technology-choice, coding-pattern, preference, debugging-insight, config-detail

## 9. Fact Extraction Pipeline

- [ ] 9.1 Create `src/extraction.ts` module
- [ ] 9.2 Implement `extractFactsFromSession` function
- [ ] 9.3 Implement `parseExtractionResponse` with JSON validation
- [ ] 9.4 Implement `computeFactHash` for idempotency checking
- [ ] 9.5 Implement `storeExtractedFact` with duplicate detection
- [ ] 9.6 Add `auto:extracted-fact` tag and source session metadata

## 10. Harvester Integration

- [ ] 10.1 Modify `harvestSessions` to accept extraction config
- [ ] 10.2 Call fact extraction after session markdown generation
- [ ] 10.3 Track extraction statistics (facts extracted, duplicates skipped)
- [ ] 10.4 Add extraction summary to harvest output
- [ ] 10.5 Handle extraction failures gracefully (log warning, continue harvest)

## 11. Storage Limits Integration

- [ ] 11.1 Add extracted fact count query to `getIndexHealth`
- [ ] 11.2 Add `extractedFacts` section to `memory_status` response
- [ ] 11.3 Check storage limits before inserting extracted facts
- [ ] 11.4 Log warning when extraction stops due to storage limit

## 12. Testing

- [ ] 12.1 Add unit tests for `OllamaLLMProvider` (mock HTTP responses)
- [ ] 12.2 Add unit tests for `OpenAICompatibleLLMProvider` (mock HTTP responses)
- [ ] 12.3 Add unit tests for consolidation prompt building
- [ ] 12.4 Add unit tests for consolidation response parsing
- [ ] 12.5 Add unit tests for consolidation decision application
- [ ] 12.6 Add unit tests for extraction prompt building
- [ ] 12.7 Add unit tests for extraction response parsing
- [ ] 12.8 Add unit tests for fact hash computation and duplicate detection
- [ ] 12.9 Add integration test for full consolidation flow (with mock LLM)
- [ ] 12.10 Add integration test for full extraction flow (with mock LLM)

## 13. Documentation

- [ ] 13.1 Update README with consolidation configuration example
- [ ] 13.2 Update README with extraction configuration example
- [ ] 13.3 Document `memory_consolidation_status` MCP tool
- [ ] 13.4 Add troubleshooting section for LLM provider issues
