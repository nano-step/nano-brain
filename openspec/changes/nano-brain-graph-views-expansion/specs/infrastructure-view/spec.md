## ADDED Requirements

### Requirement: Infrastructure view renders at /web/infrastructure

The system SHALL render an infrastructure symbols view at the `/web/infrastructure` route as a grouped table.

#### Scenario: Route accessible

- **WHEN** user navigates to `/web/infrastructure`
- **THEN** system displays the infrastructure view with navigation item highlighted

#### Scenario: Symbols grouped by type

- **WHEN** workspace has infrastructure symbols
- **THEN** view displays collapsible sections grouped by symbol type (redis_key, mysql_table, api_endpoint, etc.)

### Requirement: Symbol row display

The system SHALL display each symbol pattern with operations and locations.

#### Scenario: Row content

- **WHEN** symbol group is expanded
- **THEN** each row shows: pattern, operation badges (read/write/define), repo list, file count

#### Scenario: Expand to file details

- **WHEN** user clicks a pattern row
- **THEN** row expands to show all file paths with line numbers

### Requirement: Infrastructure filtering

The system SHALL allow filtering symbols by type, repo, and operation.

#### Scenario: Filter by type

- **WHEN** user selects type filter
- **THEN** view shows only symbols of selected type

#### Scenario: Filter by repo

- **WHEN** user enters repo filter
- **THEN** view shows only symbols from matching repositories

#### Scenario: Filter by operation

- **WHEN** user selects operation filter (read/write/define)
- **THEN** view shows only symbols with matching operations

### Requirement: Virtual scrolling

The system SHALL use virtual scrolling for large symbol lists.

#### Scenario: Large list performance

- **WHEN** symbol type has 1000+ patterns
- **THEN** list uses virtual scrolling to maintain smooth performance

### Requirement: Empty state handling

The system SHALL display appropriate messaging when no infrastructure symbols exist.

#### Scenario: No infrastructure symbols

- **WHEN** workspace has no infrastructure symbols
- **THEN** view displays empty state message
