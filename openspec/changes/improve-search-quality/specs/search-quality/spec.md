## MODIFIED Requirements

### Requirement: Search results SHALL include both code and documentation
The search pipeline SHALL return relevant results from both code files and documentation files, prioritizing actual implementations over documentation when both are relevant.

#### Scenario: Code and documentation both relevant
- **WHEN** user queries "payment processing workflow"
- **THEN** results include both the documentation file describing the workflow AND the actual code files implementing it

### Requirement: Search results SHALL prioritize symbol matches
The search pipeline SHALL rank symbol matches (function names, class names) higher than text matches in comments or strings.

#### Scenario: Symbol query
- **WHEN** user queries "processPayout function"
- **THEN** the actual `processPayout()` function appears in results before comments mentioning "processPayout"

### Requirement: Search results SHALL be deduplicated
The search pipeline SHALL remove duplicate results before returning to the user.

#### Scenario: Multiple paths to same content
- **WHEN** the same file exists at multiple paths
- **THEN** only one result is returned with the most relevant path
