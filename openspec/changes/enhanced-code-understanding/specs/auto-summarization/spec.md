## ADDED Requirements

### Requirement: Watcher auto-enqueue
The system SHALL implement the following behavior:
WHEN watcher completes `extractAndUpsertSymbols()` for a file THEN all extracted symbols are enqueued for summarization (non-blocking, <1ms overhead on watcher).

#### Scenario: Single file change triggers summarization
- **GIVEN** code_summarization.enabled = true AND budget remaining > 0
- **WHEN** user edits src/auth/handler.go (contains 3 functions)
- **THEN** watcher indexes file, extracts 3 symbols, enqueues 3 items
- **AND** within 10-30s, worker dequeues batch, calls LLM, stores 3 summaries
- **AND** chunks for those symbols are re-embedded with summary text
- **AND** memory_query("how does AuthHandler work?") returns summary in results

### Requirement: Background worker processing
The system SHALL implement the following behavior:
WHEN summarization queue has pending items AND daily budget not exhausted THEN background workers dequeue batches of up to 30 symbols, call LLM provider, and store summaries in `symbol_documents.summary`.

#### Scenario: Batch processing
- **GIVEN** 45 symbols pending in queue
- **WHEN** worker polls
- **THEN** dequeues first 30 as batch 1, calls LLM, stores summaries
- **AND** on next poll, dequeues remaining 15 as batch 2

### Requirement: Content-hash deduplication with optimistic locking
The system SHALL implement the following behavior:
WHEN a symbol is already summarized AND its content hash matches `symbol_documents.summary_hash` THEN it is NOT re-enqueued. WHEN a symbol's source content changes (hash differs) THEN existing summary is invalidated and symbol is re-enqueued. WHEN storing a summary, the system MUST verify that the symbol's content hash has not changed since enqueue time (optimistic lock). If hash differs at store time, the summary is discarded and the symbol is re-enqueued with the new hash.

#### Scenario: Unchanged symbol skipped
- **GIVEN** symbol "ProcessFile" already summarized with hash "abc123"
- **WHEN** file is re-indexed but ProcessFile content unchanged (hash still "abc123")
- **THEN** symbol is NOT enqueued for summarization (skip)

#### Scenario: Race condition — file changes during summarization
- **GIVEN** symbol "ProcessFile" enqueued with content_hash "abc123"
- **AND** while worker is calling LLM, user edits ProcessFile (new hash "def456")
- **WHEN** worker attempts to store summary
- **THEN** system checks current content_hash in symbol_documents = "def456" (differs from enqueued "abc123")
- **AND** summary is discarded (not stored)
- **AND** symbol is re-enqueued with new hash "def456" for fresh summarization

### Requirement: Budget exhaustion handling
The system SHALL implement the following behavior:
WHEN daily budget is exhausted THEN workers pause (no new dequeues) until daily reset at 00:00 UTC. In-flight batches complete. Queue items remain pending. Budget counts MUST be persisted in PostgreSQL (`code_summarization_usage` table with columns `date DATE`, `workspace_hash TEXT`, `calls_used INT`). On worker startup, today's count is loaded from DB. Server restart does not reset budget.

#### Scenario: Budget cap reached
- **GIVEN** daily budget = 100 calls AND 99 calls made today
- **WHEN** worker dequeues batch of 30 symbols (1 LLM call)
- **THEN** call succeeds (100th call), summaries stored
- **AND** worker attempts next dequeue, budget check fails, worker pauses
- **AND** queue items remain pending until tomorrow's 00:00 UTC reset

#### Scenario: Budget survives server restart
- **GIVEN** 80 calls made today, server restarts
- **WHEN** worker starts up
- **THEN** loads today's count (80) from code_summarization_usage table
- **AND** remaining budget = 20 calls (not reset to 100)

### Requirement: Rate limit and retry handling
The system SHALL implement the following behavior:
WHEN LLM provider returns HTTP 429 THEN workers pause for 60s with exponential backoff (60s, 120s, 240s, max 900s). WHEN a symbol fails summarization 3 times THEN mark as failed. Do not retry until manual trigger or file re-indexing.

#### Scenario: Provider failure with retry
- **GIVEN** LLM provider returns 500 on batch request
- **WHEN** worker processes batch
- **THEN** marks batch items as attempt+1
- **AND** retries with exponential backoff
- **AND** after 3 failures, items marked in code_summarization_failures table
- **AND** watcher continues indexing unaffected

### Requirement: Summary injection into search
The system SHALL implement the following behavior:
WHEN summary is stored THEN the chunk content for that symbol is updated to include the summary text, and the chunk is re-enqueued for embedding with PRIORITY=HIGH (ahead of new file chunks). If embedding queue exceeds 5000 items, the system SHALL log WARNING but continue enqueuing summary chunks at high priority.

#### Scenario: Summary appears in search
- **GIVEN** symbol "AuthHandler" summarized as "Validates JWT tokens from Authorization header"
- **WHEN** agent queries memory_query("how does auth work?")
- **THEN** chunk containing AuthHandler + its summary ranks high via BM25 and vector match

#### Scenario: Summary embedding has priority over file chunks
- **GIVEN** embedding queue has 200 pending file chunks
- **WHEN** summary stored for symbol "ProcessFile"
- **THEN** summary chunk enqueued at HEAD of embedding queue (priority=HIGH)
- **AND** summary embedding completes before the 200 pending file chunks

### Requirement: Feature flag
The system SHALL implement the following behavior:
WHEN `code_summarization.enabled = false` in config THEN watcher hook is no-op. WHEN config is hot-reloaded AND `enabled` changes THEN workers start/stop accordingly without server restart.

#### Scenario: Disabled by config
- **GIVEN** code_summarization.enabled = false
- **WHEN** file is indexed
- **THEN** no symbols enqueued for summarization
- **AND** no worker goroutines running

### Requirement: LLM output validation
The system SHALL validate LLM responses before storing summaries. WHEN LLM returns a response that is empty, exceeds 2000 characters, or does not contain readable prose (detected via simple heuristic: must contain at least 3 words and no HTML/XML tags) THEN the response MUST be rejected, logged as a failed attempt, and the symbol retried.

#### Scenario: LLM returns garbage
- **GIVEN** LLM provider returns HTML error page instead of summary text
- **WHEN** worker validates response
- **THEN** response rejected (contains HTML tags)
- **AND** attempt count incremented
- **AND** symbol retried on next poll cycle

#### Scenario: LLM returns empty response
- **GIVEN** LLM provider returns empty string or whitespace-only
- **WHEN** worker validates response
- **THEN** response rejected (fewer than 3 words)
- **AND** logged as validation failure

### Requirement: Prompt template
The system SHALL use the following prompt structure for code summarization:

```
You are a code documentation generator. Analyze the following code symbol and produce a concise summary (2-4 sentences) describing:
1. What this function/type does (purpose)
2. Key behavior and side effects

Symbol: {symbol_name} ({symbol_kind})
Language: {language}
File: {file_path}

Code:
{symbol_source_code}

{flow_context_if_available}

Respond with ONLY the summary text. No markdown, no bullet points, no code blocks.
```

When flow context is available (Phase 3), the `{flow_context_if_available}` section SHALL contain:
```
Context:
- TRIGGERED BY: {top_10_callers_with_frequency}
- CALLS: {direct_callees}
```

#### Scenario: Prompt produces structured summary
- **GIVEN** symbol "ProcessFile" with kind "function" in file "internal/watcher/watcher.go"
- **WHEN** prompt sent to LLM with symbol source code
- **THEN** LLM returns prose like "ProcessFile reads a source file, checks content hash against DB, chunks the content, upserts document and chunks, enqueues embeddings, and conditionally extracts symbols and graph edges."
