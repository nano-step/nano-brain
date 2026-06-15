## ADDED Requirements

### Requirement: Deduplicate results by content hash
The system SHALL detect and merge duplicate search results that have identical content.

#### Scenario: Exact duplicate files
- **WHEN** two files have identical content but different paths (e.g., `.agent/_flows/index.html` and `.agents/_flows/index.html`)
- **THEN** the system returns only one result with the shorter/cleaner path

### Requirement: Deduplicate results by path normalization
The system SHALL normalize file paths to detect near-duplicates.

#### Scenario: Path variants
- **WHEN** files exist at `src/components/Button.tsx` and `src/components/button.tsx` (case difference)
- **THEN** the system treats them as potential duplicates and returns the most relevant one

### Requirement: Preserve deduplication metadata
The system SHALL indicate when results have been deduplicated.

#### Scenario: Deduplicated result
- **WHEN** a search result has been deduplicated from multiple sources
- **THEN** the response includes a `deduplicated_from` field listing the original paths
