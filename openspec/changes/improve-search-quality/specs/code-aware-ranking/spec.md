## ADDED Requirements

### Requirement: Boost code files over documentation
The search pipeline SHALL apply a ranking boost to code files when query tokens match file paths or symbol names.

#### Scenario: Query matches file path
- **WHEN** user queries "monnectPaymentService" and results include `src/services/monnectPaymentService.js`
- **THEN** the code file appears before documentation files that mention the same term

#### Scenario: Query matches symbol name
- **WHEN** user queries "processPayout" and results include both the function definition and a comment mentioning it
- **THEN** the function definition appears first

### Requirement: Preserve cross-domain results
The search pipeline SHALL return both code and documentation results, with code boosted but not replacing docs.

#### Scenario: Mixed results
- **WHEN** user queries "payment workflow"
- **THEN** results include both workflow documentation AND implementation code, with code files ranked higher when they match symbol names
