# token-retry Specification

## Purpose
TBD - created by archiving change codesummarize-token-retry. Update Purpose after archive.
## Requirements
### Requirement: Token estimation before batch send

The system SHALL estimate token count for each batch before sending to the LLM provider.

#### Scenario: Batch within token limit

- **WHEN** a batch of 30 symbols has estimated tokens = 12,000
- **AND** `max_batch_tokens` = 100,000
- **THEN** the batch SHALL be sent as-is (no split)

#### Scenario: Batch exceeds token limit — auto-split

- **WHEN** a batch of 30 symbols has estimated tokens = 150,000
- **AND** `max_batch_tokens` = 100,000
- **THEN** the batch SHALL be recursively split in half (15 + 15)
- **AND** each sub-batch sent as a separate LLM request

#### Scenario: Single symbol exceeds limit

- **WHEN** a batch contains 1 symbol with estimated tokens = 120,000
- **AND** `max_batch_tokens` = 100,000
- **THEN** the symbol SHALL still be sent (cannot split further, base case)
- **AND** if LLM returns error, mark as permanently failed

#### Scenario: Token estimation formula

- **WHEN** estimating tokens for a batch
- **THEN** formula SHALL be: `150 + sum(len(symbol.Code)/4 + 25)` where 150 = prompt overhead, 25 = per-symbol header, chars/4 = token approximation

### Requirement: Retry with exponential backoff

The system SHALL retry failed batches up to `max_retries` times with exponential backoff.

#### Scenario: Transient error triggers retry

- **WHEN** LLM returns HTTP 429 (rate limit)
- **THEN** the system SHALL wait `retry_backoff_seconds * attempt^2` seconds
- **AND** retry the same batch
- **AND** log: level=warn, attempt number, backoff duration, error message

#### Scenario: Max retries exhausted

- **WHEN** a batch fails 3 times with transient errors
- **THEN** the system SHALL mark all symbols in the batch as failed
- **AND** record error_type = 'transient_exhausted'
- **AND** log: level=error with symbol names and final error

#### Scenario: Permanent error skips retry

- **WHEN** LLM returns HTTP 400 (bad request)
- **THEN** the system SHALL NOT retry
- **AND** immediately mark symbols as failed with error_type = 'permanent'

### Requirement: Error classification

The system SHALL classify LLM provider errors as transient or permanent to determine retry behavior.

#### Scenario: Transient errors trigger retry

- **WHEN** LLM returns HTTP 429, 408, 500, 502, 503, or 504
- **OR** request times out (context deadline exceeded)
- **THEN** the error SHALL be classified as transient
- **AND** the batch SHALL be retried according to retry policy

#### Scenario: Permanent errors skip retry

- **WHEN** LLM returns HTTP 400, 401, or 403
- **OR** JSON parsing fails after max retries
- **THEN** the error SHALL be classified as permanent
- **AND** the batch SHALL NOT be retried
- **AND** symbols SHALL be marked as permanently failed

### Requirement: Failed symbol tracking

The system SHALL persist failed symbols in `code_summarization_failures` table.

#### Scenario: Failure recorded

- **WHEN** a symbol fails summarization (permanent or retries exhausted)
- **THEN** a row SHALL be inserted with: workspace_hash, symbol_name, symbol_kind, source_file, content_hash, error_reason, error_type, attempts, last_attempt_at

#### Scenario: Retry resolves failure

- **WHEN** a previously failed symbol is successfully summarized (manual retry)
- **THEN** `resolved_at` SHALL be set to current timestamp
- **AND** the symbol SHALL no longer appear in unresolved failures list

#### Scenario: Duplicate prevention

- **WHEN** a symbol already has an unresolved failure row
- **AND** it fails again
- **THEN** the existing row SHALL be updated (attempts++, last_attempt_at, error_reason) instead of creating a duplicate

### Requirement: Status endpoint

`GET /api/v1/code/summarize/status?workspace=X` SHALL return counts.

#### Scenario: Status response

- **WHEN** workspace has 500 symbol chunks, 300 summaries, 10 unresolved failures
- **THEN** response SHALL be:
  ```json
  {"total_symbols": 500, "summarized": 300, "pending": 190, "failed": 10}
  ```

### Requirement: Failures list endpoint

`GET /api/v1/code/summarize/failures?workspace=X` SHALL return unresolved failures.

#### Scenario: List failures

- **WHEN** workspace has 3 unresolved failures
- **THEN** response SHALL be array of `{id, symbol_name, symbol_kind, source_file, error_reason, error_type, attempts, last_attempt_at}`

### Requirement: Retry endpoint

`POST /api/v1/code/summarize/retry` SHALL re-attempt failed symbols.

#### Scenario: Retry specific symbols

- **WHEN** called with `{"workspace":"X","failure_ids":["uuid1","uuid2"]}`
- **THEN** the system SHALL re-run summarization for those symbols only
- **AND** on success: set `resolved_at`, create summary document
- **AND** on failure: update attempts count and error_reason
- **AND** return `{"retried": 2, "succeeded": 1, "failed": 1}`

#### Scenario: Retry all

- **WHEN** `POST /api/v1/code/summarize/retry-all` called with `{"workspace":"X"}`
- **THEN** the system SHALL retry ALL unresolved failures for that workspace

### Requirement: Configuration

The system SHALL support configurable retry and token limit settings.

#### Scenario: Defaults when fields omitted

- **WHEN** config fields `max_batch_tokens`, `max_retries`, `retry_backoff_seconds` are omitted
- **THEN** defaults SHALL be: max_batch_tokens=100000, max_retries=3, retry_backoff_seconds=1

#### Scenario: Custom configuration applied

- **WHEN** config contains `max_batch_tokens: 50000`, `max_retries: 5`, `retry_backoff_seconds: 2`
- **THEN** the system SHALL use these values for token splitting threshold, retry count, and backoff base

