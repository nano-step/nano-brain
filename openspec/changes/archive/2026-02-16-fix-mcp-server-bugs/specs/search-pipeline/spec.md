## MODIFIED Requirements

### Requirement: FTS5 query sanitization
The `searchFTS` function SHALL sanitize user queries before passing them to FTS5 `MATCH`. All user-provided query strings MUST be treated as literal search text, never as FTS5 syntax.

#### Scenario: Query containing hyphenated words
- **WHEN** user searches for `nano-brain`
- **THEN** the search treats the entire hyphenated term as a literal phrase, not as `opencode NOT memory`

#### Scenario: Query containing FTS5 column names
- **WHEN** user searches for `memory architecture`
- **THEN** the search treats `memory` as a search term, not as a column reference
- **THEN** no `no such column` error is thrown

#### Scenario: Query containing FTS5 operators
- **WHEN** user searches for `AND OR NOT NEAR`
- **THEN** the search treats these as literal words, not as FTS5 boolean operators

#### Scenario: Query containing double quotes
- **WHEN** user searches for `he said "hello"`
- **THEN** internal double quotes are escaped and the search completes without SQL error

#### Scenario: Empty or whitespace-only query
- **WHEN** user searches for `   ` or empty string
- **THEN** the search returns an empty result set without error

#### Scenario: Normal multi-word query
- **WHEN** user searches for `sqlite vector search`
- **THEN** the search returns documents containing those terms, ranked by BM25 relevance
