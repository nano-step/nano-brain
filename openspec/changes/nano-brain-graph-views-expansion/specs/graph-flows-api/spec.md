## ADDED Requirements

### Requirement: Flows endpoint returns execution flows with steps

The system SHALL expose `GET /api/v1/graph/flows?workspace=<hash>` that returns all execution flows with their step details for the specified workspace.

#### Scenario: Successful flows retrieval

- **WHEN** client sends GET request to `/api/v1/graph/flows?workspace=abc123`
- **THEN** server returns 200 with JSON containing `flows` array

#### Scenario: Flow object structure

- **WHEN** response contains flows
- **THEN** each flow object SHALL include: `id`, `label`, `flowType`, `entrySymbol`, `terminalSymbol`, `stepCount`, `steps`

#### Scenario: Flow step structure

- **WHEN** flow contains steps
- **THEN** each step object SHALL include: `symbolId`, `symbolName`, `filePath`, `stepIndex`

#### Scenario: Empty workspace

- **WHEN** workspace has no execution flows
- **THEN** server returns 200 with empty array: `{"flows": []}`

#### Scenario: Invalid workspace hash

- **WHEN** workspace parameter is missing or invalid
- **THEN** server returns 400 with error message

#### Scenario: Flow type values

- **WHEN** response contains flows
- **THEN** flowType SHALL be one of: `intra_community`, `cross_community`
