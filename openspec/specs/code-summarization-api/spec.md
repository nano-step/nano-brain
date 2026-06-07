# code-summarization-api Specification

## Purpose
TBD - created by archiving change code-symbol-summarization. Update Purpose after archive.
## Requirements
### Requirement: Daily budget counter (DB-persisted)

The system SHALL track daily LLM request count per workspace in PostgreSQL to survive server restarts.

#### Scenario: Budget counter incremented on each batch request

- **WHEN** a batch LLM request is made for workspace W
- **THEN** the system SHALL increment `code_summarization_usage.request_count` for `(workspace_hash=W, usage_date=CURRENT_DATE)` in UTC

#### Scenario: Budget exhausted stops summarization

- **WHEN** `request_count >= max_requests_per_day` for today (and `max_requests_per_day > 0`)
- **THEN** the system SHALL skip summarization for this workspace until the next UTC day
- **AND** log an INFO message: "Daily code summarization budget exhausted for workspace W"

#### Scenario: Unlimited budget (max_requests_per_day = 0)

- **WHEN** `max_requests_per_day` is configured as 0
- **THEN** the system SHALL NOT enforce any daily limit

#### Scenario: Server restart mid-day preserves counter

- **WHEN** server has made 1500 requests today and restarts
- **THEN** on restart the system SHALL read the existing counter (1500) from DB
- **AND** continue counting from 1500 (not reset to 0)

### Requirement: Embedding queue depth protection

The system SHALL pause summarization when the embedding queue is saturated.

#### Scenario: Queue depth exceeds threshold

- **WHEN** embedding queue has >1000 pending items
- **THEN** the system SHALL skip the current poll cycle
- **AND** log a WARNING: "Pausing code summarization: embedding queue depth exceeds 1000"

#### Scenario: Queue drains below threshold

- **WHEN** embedding queue pending count drops below 500
- **THEN** the system SHALL resume summarization on the next poll cycle

### Requirement: Polling lifecycle

The system SHALL run as an independent background goroutine started at server boot.

#### Scenario: Service starts when enabled

- **WHEN** server boots with `code_summarization.enabled=true`
- **THEN** a background goroutine SHALL start polling every `poll_interval_seconds` (default 60)

#### Scenario: Service does not start when disabled

- **WHEN** `code_summarization.enabled=false`
- **THEN** no background goroutine SHALL be started
- **AND** no LLM calls SHALL be made

#### Scenario: Graceful shutdown

- **WHEN** server receives shutdown signal (context cancelled)
- **THEN** the summarizer SHALL finish any in-progress batch
- **AND** stop polling
- **AND** NOT start new batches

#### Scenario: No unsummarized symbols found

- **WHEN** poll cycle finds 0 unsummarized symbol chunks
- **THEN** the system SHALL sleep until next poll interval without making any LLM calls

#### Scenario: Max summaries per cycle cap

- **WHEN** there are 5000 unsummarized symbols but `max_summaries_per_cycle=300`
- **THEN** only 300 symbols (10 batches of 30) SHALL be processed this cycle
- **AND** remaining symbols SHALL be processed in subsequent cycles

### Requirement: Orphan summary cleanup

The system SHALL periodically remove summary documents whose source symbol no longer exists.

#### Scenario: Symbol deleted triggers orphan cleanup

- **WHEN** hourly GC runs
- **AND** a summary document's `metadata.source_content_hash` does not match any current chunk's `content_hash` in the same workspace
- **THEN** the summary document SHALL be deleted

#### Scenario: GC frequency

- **WHEN** the summarizer is running
- **THEN** orphan cleanup SHALL execute once per hour (not every poll cycle)

### Requirement: Configuration

The system SHALL be configured via the `code_summarization` section in config.yml.

#### Scenario: All config fields respected

- **WHEN** config contains:
  ```yaml
  code_summarization:
    enabled: true
    provider_url: "https://ai-proxy.example.com/v1"
    model: "gemini/gemini-2.5-flash"
    batch_size: 30
    max_output_tokens: 8000
    concurrency: 2
    max_requests_per_day: 3000
    max_symbol_lines: 500
    poll_interval_seconds: 60
    max_summaries_per_cycle: 300
  ```
- **THEN** the system SHALL use these values for all summarization behavior

#### Scenario: Defaults when fields omitted

- **WHEN** `code_summarization.enabled=true` but other fields omitted
- **THEN** the system SHALL use defaults: `batch_size=30`, `max_output_tokens=8000`, `concurrency=2`, `max_requests_per_day=0`, `max_symbol_lines=500`, `poll_interval_seconds=60`, `max_summaries_per_cycle=300`

#### Scenario: API key from environment variable

- **WHEN** `code_summarization.api_key` is empty
- **AND** environment variable `NANO_BRAIN_CODE_SUMMARIZE_API_KEY` is set
- **THEN** the system SHALL use the environment variable value

### Requirement: Manual trigger API endpoint

The system SHALL expose `POST /api/v1/code/summarize` to trigger summarization manually.

#### Scenario: Manual trigger processes symbols

- **WHEN** `POST /api/v1/code/summarize` is called with `{"workspace": "<hash>"}`
- **THEN** the system SHALL immediately process up to `max_summaries_per_cycle` unsummarized symbols
- **AND** return `{"processed": N, "skipped": M, "errors": E}`

#### Scenario: Manual trigger respects daily budget

- **WHEN** daily budget is exhausted
- **AND** manual trigger is called
- **THEN** the system SHALL return `{"processed": 0, "skipped": 0, "errors": 0, "message": "daily budget exhausted"}`
- **AND** HTTP status SHALL be 200 (not an error — informational)

#### Scenario: Feature disabled

- **WHEN** `code_summarization.enabled=false`
- **AND** manual trigger is called
- **THEN** the system SHALL return HTTP 400 with `{"error": "code summarization is disabled"}`

