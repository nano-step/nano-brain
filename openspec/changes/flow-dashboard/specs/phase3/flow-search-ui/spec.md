## ADDED Requirements

### Requirement: Search flow summaries
The dashboard SHALL allow users to search flow summaries by endpoint path, handler name, or middleware name.

#### Scenario: Filter by path substring
- **WHEN** the user enters "write" in the search field
- **THEN** the list updates to show only flows whose path contains "write" (case-insensitive)

#### Scenario: Filter by handler name
- **WHEN** the user enters "Query" in the search field
- **THEN** the list updates to show only flows whose handler contains "Query"

#### Scenario: No matches
- **WHEN** the search term matches no flows
- **THEN** the list shows "No matching endpoints" message

#### Scenario: Debounced input
- **WHEN** the user types quickly
- **THEN** the filter updates after a 300ms debounce to avoid excessive DOM updates
