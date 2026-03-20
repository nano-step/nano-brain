## ADDED Requirements

### Requirement: Infrastructure endpoint returns symbols grouped by type

The system SHALL expose `GET /api/v1/graph/infrastructure?workspace=<hash>` that returns infrastructure symbols with both flat and grouped representations.

#### Scenario: Successful infrastructure retrieval

- **WHEN** client sends GET request to `/api/v1/graph/infrastructure?workspace=abc123`
- **THEN** server returns 200 with JSON containing `symbols` array and `grouped` object

#### Scenario: Symbol object structure

- **WHEN** response contains symbols
- **THEN** each symbol object SHALL include: `type`, `pattern`, `operation`, `repo`, `filePath`, `lineNumber`

#### Scenario: Grouped structure

- **WHEN** response contains grouped object
- **THEN** grouped SHALL be keyed by symbol type with arrays of pattern objects containing `pattern` and `operations` array

#### Scenario: Operations structure

- **WHEN** grouped pattern contains operations
- **THEN** each operation object SHALL include: `op`, `repo`, `file`

#### Scenario: Empty workspace

- **WHEN** workspace has no infrastructure symbols
- **THEN** server returns 200 with empty data: `{"symbols": [], "grouped": {}}`

#### Scenario: Invalid workspace hash

- **WHEN** workspace parameter is missing or invalid
- **THEN** server returns 400 with error message

#### Scenario: Symbol type values

- **WHEN** response contains symbols
- **THEN** type SHALL be one of: `redis_key`, `mysql_table`, `api_endpoint`, `env_var`, `queue_name`, `s3_bucket`, or other infrastructure types
