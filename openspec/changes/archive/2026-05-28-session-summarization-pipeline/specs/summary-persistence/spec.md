## ADDED Requirements

### Requirement: Save summary as physical markdown file
The system SHALL write the final summary to `{output_dir}/{source}_{title_slug}_{date}.md` where source is `opencode` or `claude`, title_slug is the slugified session title (lowercase, alphanumeric + dashes, max 80 chars), and date is `YYYY-MM-DD`.

#### Scenario: OpenCode session summary saved
- **WHEN** session title is "nano-brain workspace registration issue" and date is 2026-05-26
- **THEN** the file SHALL be saved as `{output_dir}/opencode_nano-brain-workspace-registration-issue_2026-05-26.md`

#### Scenario: Output directory does not exist
- **WHEN** the configured output_dir does not exist
- **THEN** the system SHALL create it with `0755` permissions before writing

#### Scenario: Duplicate filename
- **WHEN** a file with the same name already exists (re-summarization)
- **THEN** the system SHALL overwrite the existing file

### Requirement: Store summary in document DB with idempotent upsert
The system SHALL store the summary in the documents table using `UpsertDocumentBySourcePath` with `source_path = "summary://{source}/{session_id}"` and `collection = "session-summary"`.

#### Scenario: First-time summary storage
- **WHEN** no document exists with this source_path
- **THEN** the system SHALL insert a new document, chunk it, and enqueue chunks for embedding

#### Scenario: Re-summarization with changed content
- **WHEN** a document exists with same source_path but content_hash differs
- **THEN** the system SHALL update the document, delete old chunks (cascade delete old embeddings), insert new chunks, and enqueue for embedding

#### Scenario: Re-summarization with unchanged content
- **WHEN** a document exists with same source_path and same content_hash
- **THEN** the system SHALL skip the update entirely

### Requirement: Chunk and embed summaries for vector search
After storing the summary document, the system SHALL chunk it using default chunk config and enqueue all chunks for embedding via the existing embed queue.

#### Scenario: Summary embedded for semantic search
- **WHEN** a summary is stored and chunked
- **THEN** the chunks SHALL appear in vector search results when querying for topics discussed in the session

### Requirement: Summarization triggered by harvest
The system SHALL trigger summarization for each session that was newly harvested or re-harvested (content_hash changed) during the harvest cycle. Sessions with unchanged content_hash SHALL be skipped.

#### Scenario: New session harvested
- **WHEN** the harvester processes a session for the first time
- **THEN** the summarizer SHALL be called with the session content and metadata

#### Scenario: Session content unchanged
- **WHEN** the harvester detects a session with unchanged content_hash
- **THEN** the summarizer SHALL NOT be called for that session

#### Scenario: LLM provider unavailable
- **WHEN** the LLM provider is unreachable during summarization
- **THEN** the system SHALL log a warning and skip summarization for this cycle; the session's raw content SHALL still be harvested normally (chunked + embedded as fallback)

### Requirement: LLM provider documented in setup guide
The setup/onboarding documentation SHALL include instructions for configuring the `summarization` section in config.yaml, including provider URL, API key, model selection, and output directory.

#### Scenario: User configures ai-proxy
- **WHEN** a user follows the setup guide to configure ai-proxy
- **THEN** the guide SHALL provide the exact config.yaml snippet with ai-proxy URL, model name, and note about API key sourcing (config or environment variable)
