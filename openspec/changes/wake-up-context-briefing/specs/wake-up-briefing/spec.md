# Spec: wake-up-briefing

Core briefing generation logic and all three access surfaces (CLI, MCP, HTTP).

## ADDED Requirements

### Requirement: generateBriefing returns structured BriefingResult
The system SHALL return a structured BriefingResult with l0 and l1 sections when generateBriefing is called.

#### Scenario: Basic briefing generation
- **WHEN** `generateBriefing(store, configPath, projectHash)` is called
- **THEN** it returns a `BriefingResult` object with `l0` (workspace identity) and `l1` (critical facts) sections
- **AND** the formatted text output is a string

### Requirement: L0 includes collection names and top topics
The L0 section SHALL include workspace identity from collection config and workspace profile topics.

#### Scenario: Workspace with collections
- **WHEN** a workspace has collections configured
- **THEN** L0 section includes collection names from `loadCollectionConfig()`
- **AND** L0 section includes top topics from `WorkspaceProfile.loadProfile()`

#### Scenario: Workspace without collections
- **WHEN** a workspace has no collections configured
- **THEN** L0 section shows only the workspace path

### Requirement: L1 includes top-accessed documents
The L1 section SHALL include up to 10 most-accessed non-superseded documents.

#### Scenario: Top-accessed documents in L1
- **WHEN** `generateBriefing()` is called
- **THEN** L1 includes up to 10 documents ordered by `access_count DESC`
- **AND** each entry shows title, collection, and a 1-line snippet
- **AND** superseded documents are excluded

### Requirement: L1 includes recent decision-tagged documents
The L1 section SHALL include up to 5 recent documents tagged with "decision".

#### Scenario: Recent decisions in L1
- **WHEN** `generateBriefing()` is called
- **THEN** L1 includes up to 5 documents tagged with "decision" ordered by `modified_at DESC`
- **AND** each entry shows title, date, and a 1-line snippet
- **AND** superseded documents are excluded

### Requirement: Output is capped at 2000 characters
The formatted briefing output SHALL NOT exceed 2000 characters.

#### Scenario: Briefing exceeds character limit
- **WHEN** the composed briefing exceeds 2000 characters
- **THEN** sections are truncated to fit within the budget
- **AND** L0 gets ~400 chars, L1 key memories ~800 chars, L1 decisions ~600 chars

### Requirement: Empty workspace returns minimal briefing
The system SHALL return a non-empty briefing with workspace path even when no documents exist.

#### Scenario: Empty workspace
- **WHEN** the workspace has no indexed documents
- **THEN** `generateBriefing()` returns a briefing with workspace path and "no memories yet" message
- **AND** the return value is not an empty string

### Requirement: CLI command
The system SHALL expose a `nano-brain wake-up` CLI command with --json and --workspace flags.

#### Scenario: CLI default output
- **WHEN** `nano-brain wake-up` is run
- **THEN** the briefing text is printed to stdout

#### Scenario: CLI JSON output
- **WHEN** `nano-brain wake-up --json` is run
- **THEN** the structured BriefingResult is printed as JSON

#### Scenario: CLI workspace override
- **WHEN** `nano-brain wake-up --workspace=<path>` is run
- **THEN** the briefing is generated for the specified workspace

### Requirement: MCP tool
The system SHALL expose a `memory_wake_up` MCP tool with optional workspace parameter.

#### Scenario: MCP tool invocation
- **WHEN** the `memory_wake_up` MCP tool is called
- **THEN** it returns the briefing text
- **AND** it accepts an optional `workspace` parameter

### Requirement: HTTP endpoint
The system SHALL expose GET and POST /api/wake-up HTTP routes with optional workspace parameter.

#### Scenario: HTTP GET wake-up
- **WHEN** `GET /api/wake-up` is requested
- **THEN** it returns the briefing as JSON response
- **AND** it accepts an optional `workspace` query parameter

#### Scenario: HTTP POST wake-up
- **WHEN** `POST /api/wake-up` is requested with body `{"workspace": "<path>"}`
- **THEN** it returns the briefing for the specified workspace