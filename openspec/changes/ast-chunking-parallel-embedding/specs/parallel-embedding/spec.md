## ADDED Requirements

### Requirement: Concurrent batch processing
The system SHALL process embedding batches concurrently with EMBEDDING_CONCURRENCY=3 parallel batches.

#### Scenario: Three batches processed in parallel
- **WHEN** there are 6 pending batches to embed
- **THEN** the system SHALL process up to 3 batches concurrently

#### Scenario: Single batch processed when only one pending
- **WHEN** there is only 1 pending batch
- **THEN** the system SHALL process it without waiting for additional batches

### Requirement: Backpressure control
The system SHALL implement backpressure with MAX_PENDING_BATCHES=10, pausing new batch creation when saturated.

#### Scenario: Backpressure applied at limit
- **WHEN** 10 batches are pending/in-progress
- **THEN** the system SHALL wait for batch completion before creating new batches

#### Scenario: Processing resumes after completion
- **WHEN** a batch completes and pending count drops below MAX_PENDING_BATCHES
- **THEN** the system SHALL resume creating new batches

### Requirement: Preserve existing batch parameters
The system SHALL maintain existing batch size (50 documents) and chunk limit (200 chunks per batch).

#### Scenario: Batch size unchanged
- **WHEN** processing a batch of documents
- **THEN** each batch SHALL contain up to 50 documents

### Requirement: Sequential fallback on failure
The system SHALL fall back to sequential processing for a batch if concurrent processing fails.

#### Scenario: Failed batch retried sequentially
- **WHEN** a batch fails during concurrent processing
- **THEN** the system SHALL retry that batch sequentially before continuing

#### Scenario: Partial failure does not block others
- **WHEN** one batch fails while others succeed
- **THEN** successful batches SHALL complete and only the failed batch SHALL retry
