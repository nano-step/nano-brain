## ADDED Requirements

### Requirement: Automatic tag assignment on write

When `memory_write` is called, the system SHALL analyze the content and assign category tags based on keyword/pattern heuristics. Auto-generated tags SHALL be prefixed with `auto:` (e.g., `auto:architecture-decision`).

#### Scenario: Content contains architecture decision keywords

- **WHEN** content contains "decided to use PostgreSQL"
- **THEN** the document is tagged with `auto:architecture-decision`

#### Scenario: Content contains debugging keywords

- **WHEN** content contains "fixed bug" and "stack trace"
- **THEN** the document is tagged with `auto:debugging-insight`

#### Scenario: Content with no matching patterns

- **WHEN** content does not match any category patterns
- **THEN** no auto tags are added to the document

### Requirement: Category definitions

The system SHALL recognize these categories: `architecture-decision` (keywords: decided, chose, architecture, design, tradeoff, approach), `debugging-insight` (keywords: error, fix, bug, stack trace, debug, workaround), `tool-config` (keywords: config, setup, install, environment, .env), `pattern` (keywords: pattern, convention, always, never, rule), `preference` (keywords: prefer, like, dislike, favorite, default), `context` (keywords: context, background, note, remember), `workflow` (keywords: workflow, process, step, pipeline, deploy).

#### Scenario: Architecture decision content

- **WHEN** content is "We chose React over Vue for the frontend"
- **THEN** the document is tagged with `auto:architecture-decision`

#### Scenario: Debugging insight content

- **WHEN** content is "npm install failed, fixed by clearing cache"
- **THEN** the document is tagged with `auto:debugging-insight`

#### Scenario: Content matching multiple categories

- **WHEN** content matches patterns for both `architecture-decision` and `workflow`
- **THEN** all matching auto tags are applied to the document

### Requirement: Additive tagging

Auto-categorization SHALL NOT remove or replace user-provided tags. Auto tags are added alongside any tags the user explicitly provides.

#### Scenario: User provides explicit tags

- **WHEN** user provides tags=["important"] and content matches debugging patterns
- **THEN** the final tags are ["important", "auto:debugging-insight"]

#### Scenario: User provides no tags

- **WHEN** user provides no tags and content matches categorization patterns
- **THEN** only auto tags are applied to the document

### Requirement: Deterministic and fast

Auto-categorization SHALL NOT use LLM calls. It SHALL complete in under 5ms for typical content (under 10KB). The heuristic engine SHALL be pure keyword/regex matching.

#### Scenario: Small document categorization

- **WHEN** a 5KB markdown document is written
- **THEN** auto-categorization completes in under 5ms

#### Scenario: Large document categorization

- **WHEN** a 100KB document is written
- **THEN** auto-categorization completes in under 50ms
