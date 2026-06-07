# flow-enriched-summaries Specification

## Purpose
TBD - created by archiving change enhanced-code-understanding. Update Purpose after archive.
## Requirements
### Requirement: Flow context in summarization prompt
The system SHALL implement the following behavior:
WHEN summarizing a symbol AND graph edges exist for that symbol THEN the LLM prompt includes caller/callee context (max top-10 callers by edge frequency, max depth = 1 hop). When no graph edges exist, summarize without flow context.

#### Scenario: Flow-enriched summary generation
- **GIVEN** symbol "extractAndUpsertSymbols" has callers: [processFile(50), scanCollection(10), processDirty(5)]
- **AND** callees: [symbolRegistry.Extract(), storage.UpsertSymbols()]
- **WHEN** summarization worker processes this symbol
- **THEN** LLM prompt includes "TRIGGERED BY: processFile (50), scanCollection (10), processDirty (5)"
- **AND** prompt includes "CALLS: symbolRegistry.Extract, storage.UpsertSymbols"

### Requirement: Fan-out cap
The system SHALL implement the following behavior:
WHEN a symbol has more than 10 callers THEN only top-10 by call frequency are included. Flow context section capped at 1000 tokens.

#### Scenario: High fan-out capped
- **GIVEN** symbol "logger.Info" has 500 callers
- **WHEN** summarization worker processes this symbol
- **THEN** only top-10 callers by frequency included in prompt
- **AND** prompt includes "and 490 more callers"

### Requirement: Dual-hash invalidation
The system SHALL implement the following behavior:
WHEN a symbol's caller/callee list changes (graph topology shift) THEN the symbol's `graph_context_hash` differs from stored value, triggering re-summarization.

#### Scenario: Graph topology change triggers re-summarization
- **GIVEN** symbol A summarized with graph_context_hash = "hash1" (callers: B, C)
- **WHEN** new function D starts calling A (graph edge added)
- **THEN** graph_context_hash recomputed = "hash2" (callers: B, C, D)
- **AND** hash differs from stored, symbol A re-enqueued for summarization

### Requirement: Cascade invalidation cap
The system SHALL implement the following behavior:
WHEN a widely-called symbol changes THEN max 20 caller-summaries are marked stale (sorted by PageRank importance if available, else by recency). When queue exceeds 1000 items, drop lowest-priority.

#### Scenario: Cascade limited to 20 callers
- **GIVEN** symbol "utils.FormatError" changes (content hash differs)
- **AND** 200 symbols have summaries mentioning "utils.FormatError" as caller
- **WHEN** invalidation runs
- **THEN** only top-20 caller-summaries marked stale (by importance_score DESC)
- **AND** remaining 180 re-summarized eventually on their own file change

#### Scenario: Queue overflow protection
- **GIVEN** queue has 950 items AND cascade invalidation adds 200 more (total 1150)
- **WHEN** queue exceeds 1000
- **THEN** 150 lowest-priority items dropped (fewest graph references)
- **AND** dropped items logged to zerolog with warning level

