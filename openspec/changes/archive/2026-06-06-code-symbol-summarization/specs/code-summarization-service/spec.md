## ADDED Requirements

### Requirement: Batch LLM summarization of code symbols

The system SHALL generate 2-4 sentence summaries for code symbols (functions, types, interfaces, structs, methods) by sending batches of symbols to an OpenAI-compatible LLM provider.

#### Scenario: Successful batch summarization

- **WHEN** the CodeSummarizer polls and finds 30 unsummarized symbol chunks in workspace W
- **THEN** it SHALL build a single prompt containing all 30 symbols with their source code
- **AND** send one HTTP request to the configured LLM provider
- **AND** parse the structured JSON array response
- **AND** create one summary document per symbol with tag `symbol-summary`
- **AND** enqueue each summary's chunk_id for embedding

#### Scenario: Partial LLM response (count mismatch)

- **WHEN** the LLM returns summaries for only 25 of 30 sent symbols
- **THEN** the system SHALL store the 25 matched summaries
- **AND** the 5 unmatched symbols SHALL be retried on the next poll cycle (implicit via re-query)

#### Scenario: Invalid JSON response

- **WHEN** the LLM returns invalid JSON
- **THEN** the system SHALL retry the same batch once
- **AND** if retry also fails, skip the batch and log a WARNING

#### Scenario: LLM provider timeout or error

- **WHEN** the LLM provider returns an HTTP error or times out
- **THEN** the system SHALL log a WARNING and skip the batch
- **AND** the symbols SHALL be retried on the next poll cycle

#### Scenario: Symbol exceeds max_symbol_lines

- **WHEN** a symbol's source code exceeds `max_symbol_lines` (default 500)
- **THEN** the system SHALL skip that symbol with a WARNING log
- **AND** it SHALL NOT be included in any batch prompt

### Requirement: Response matching by composite key

The system SHALL match LLM responses to input symbols using the composite key `(name, file, kind)` with strict equality.

#### Scenario: Same-name symbols in different files

- **WHEN** batch contains `func Parse()` from `a.go` and `func Parse()` from `b.go`
- **AND** LLM returns `[{"name":"Parse","file":"a.go","summary":"..."},{"name":"Parse","file":"b.go","summary":"..."}]`
- **THEN** each summary SHALL be matched to the correct symbol by `(name, file)` pair

#### Scenario: LLM returns symbols in different order

- **WHEN** symbols are sent as `[A, B, C]` and LLM returns `[C, A, B]`
- **THEN** matching SHALL succeed (order-independent, key-based)

### Requirement: Summary document storage format

Each summary SHALL be stored as a document with the following properties:

- `collection`: same as source symbol's collection (typically `code`)
- `source_path`: `<file_path>?symbol=<name>&kind=<kind>&hash=<content_hash[:8]>&summary=true`
- `title`: `Summary: <name> (<kind>)`
- `content`: the 2-4 sentence summary text
- `tags`: `["symbol-summary"]`
- `metadata`: `{"symbol_name": "<name>", "symbol_kind": "<kind>", "source_file": "<path>", "source_content_hash": "<full_hash>", "model_version": "<model_name>"}`
- `embedding_strategy`: `summary`

#### Scenario: Summary document created for a function

- **WHEN** symbol `gitignoreStack` (struct) in `internal/watcher/filter.go` is summarized
- **THEN** a document SHALL be created with `source_path = "internal/watcher/filter.go?symbol=gitignoreStack&kind=struct&hash=a1b2c3d4&summary=true"`
- **AND** content SHALL contain 2-4 sentences describing the struct's behavior

#### Scenario: Summary superseded on code change

- **WHEN** symbol `gitignoreStack` is modified (content_hash changes)
- **THEN** the old summary's source_path no longer matches any current chunk
- **AND** a new summary SHALL be generated on the next poll cycle
- **AND** old summary SHALL be cleaned up by periodic GC

### Requirement: Incremental summarization via content-hash

The system SHALL skip symbols that already have a matching summary document (same `content_hash[:8]` in source_path).

#### Scenario: Unchanged symbol skipped

- **WHEN** symbol chunk has content_hash `abc123def456`
- **AND** a summary document exists with source_path containing `hash=abc123de`
- **THEN** the symbol SHALL NOT be included in any batch

#### Scenario: Changed symbol re-summarized

- **WHEN** symbol chunk content_hash changes from `abc123def456` to `fff999aaa111`
- **THEN** no summary matches the new hash
- **AND** the symbol SHALL be included in the next batch
