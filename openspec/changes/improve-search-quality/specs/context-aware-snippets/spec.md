## ADDED Requirements

### Requirement: Generate snippets based on function boundaries
The system SHALL extract snippets that correspond to complete function or class definitions.

#### Scenario: Function snippet
- **WHEN** a search result matches a function `processPayout()`
- **THEN** the snippet contains the entire function body plus any preceding comments

### Requirement: Include relevant context in snippets
The system SHALL include surrounding context that helps understand the matched code.

#### Scenario: Class method with context
- **WHEN** a search result matches a method `validatePayout()` inside class `PaymentService`
- **THEN** the snippet includes the class name and method signature

### Requirement: Handle large functions gracefully
The system SHALL limit snippet size while preserving completeness.

#### Scenario: Oversized function
- **WHEN** a function exceeds 1000 characters
- **THEN** the snippet contains the first 1000 characters with an ellipsis indicator
