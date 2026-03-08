## ADDED Requirements

### Requirement: LLM-driven fact extraction from sessions
During session harvesting, the system SHALL use an LLM to extract discrete facts from session transcripts. Each fact SHALL be a concise, searchable statement capturing a decision, preference, pattern, or technical detail.

#### Scenario: Extract facts from session transcript
- **WHEN** `harvest` command processes a session transcript
- **AND** extraction is enabled
- **THEN** the LLM analyzes the transcript and returns a list of discrete facts
- **AND** each fact is stored as a separate memory document

#### Scenario: Fact content format
- **WHEN** a fact is extracted from a session
- **THEN** the fact content is a single concise statement (1-2 sentences)
- **AND** the fact is self-contained and searchable without additional context

#### Scenario: Example fact extraction
- **WHEN** a session transcript contains discussion about database choice
- **AND** the conversation concludes with "Let's use PostgreSQL for the main database"
- **THEN** a fact is extracted: "Project uses PostgreSQL for the main database"

### Requirement: Extracted facts are tagged and linked to source
Each extracted fact SHALL be tagged with `auto:extracted-fact` and include metadata linking to the source session.

#### Scenario: Fact tagging
- **WHEN** a fact is extracted and stored
- **THEN** the document has tag `auto:extracted-fact`
- **AND** the document metadata includes `source_session_id` field

#### Scenario: Query extracted facts by tag
- **WHEN** `memory_search` is called with tag filter `auto:extracted-fact`
- **THEN** only extracted facts are returned
- **AND** source session information is available in results

### Requirement: Idempotent fact extraction
Re-harvesting the same session SHALL NOT create duplicate facts. The system SHALL use content hashing to detect and skip existing facts.

#### Scenario: Re-harvest same session
- **WHEN** `harvest` command processes a session that was previously harvested
- **AND** the session content has not changed
- **THEN** no new facts are created
- **AND** existing facts remain unchanged

#### Scenario: Re-harvest modified session
- **WHEN** `harvest` command processes a session that was previously harvested
- **AND** the session content has changed (new messages added)
- **THEN** only new facts from the changed content are extracted
- **AND** existing facts remain unchanged

#### Scenario: Duplicate fact detection
- **WHEN** the LLM extracts a fact with identical content to an existing fact
- **THEN** the duplicate fact is not inserted
- **AND** a log entry indicates the duplicate was skipped

### Requirement: Fact extraction configuration
The system SHALL support extraction configuration with provider, model, enabled flag, and maxFactsPerSession.

#### Scenario: Extraction disabled by default
- **WHEN** no `extraction` section exists in config
- **THEN** extraction is disabled
- **AND** `harvest` command does not invoke LLM for fact extraction

#### Scenario: Extraction enabled with limit
- **WHEN** config contains `extraction: { enabled: true, provider: "ollama", model: "llama3.2", maxFactsPerSession: 10 }`
- **THEN** extraction uses Ollama API
- **AND** at most 10 facts are extracted per session

#### Scenario: Extraction with OpenAI-compatible provider
- **WHEN** config contains `extraction: { enabled: true, provider: "openai", url: "https://api.openai.com", apiKey: "sk-...", model: "gpt-4o-mini" }`
- **THEN** extraction uses OpenAI-compatible API
- **AND** the specified model is used for extraction

### Requirement: Fact extraction prompt design
The extraction prompt SHALL guide the LLM to extract specific types of facts: decisions, preferences, patterns, and technical details.

#### Scenario: Extraction prompt categories
- **WHEN** the LLM is invoked for fact extraction
- **THEN** the prompt instructs extraction of: architecture decisions, technology choices, coding patterns, user preferences, debugging insights, and configuration details

#### Scenario: Extraction output format
- **WHEN** the LLM completes fact extraction
- **THEN** the response is a JSON array of fact objects
- **AND** each fact object contains `content` (string) and `category` (string) fields

### Requirement: Graceful degradation on extraction failure
When the LLM provider is unavailable or returns an error, extraction SHALL fail gracefully without affecting session harvesting.

#### Scenario: LLM provider unreachable during harvest
- **WHEN** `harvest` command runs
- **AND** extraction is enabled
- **AND** the LLM provider is unreachable
- **THEN** session markdown is still generated and indexed
- **AND** fact extraction is skipped with a warning log
- **AND** harvest completes successfully

#### Scenario: LLM returns malformed extraction response
- **WHEN** extraction is triggered
- **AND** the LLM returns non-JSON or invalid schema
- **THEN** extraction is skipped for this session
- **AND** a warning is logged with the parse error
- **AND** session markdown is still indexed

### Requirement: Extraction respects workspace boundaries
Extracted facts SHALL inherit the workspace (projectHash) from their source session.

#### Scenario: Facts scoped to workspace
- **WHEN** facts are extracted from a session with projectHash "abc123def456"
- **THEN** each extracted fact document has projectHash "abc123def456"
- **AND** workspace-scoped searches include these facts

#### Scenario: Cross-workspace fact search
- **WHEN** `memory_search` is called with `workspace: "all"`
- **THEN** extracted facts from all workspaces are searchable
