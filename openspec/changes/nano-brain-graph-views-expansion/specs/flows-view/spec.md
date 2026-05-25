## ADDED Requirements

### Requirement: Flows view renders at /web/flows

The system SHALL render an execution flows view at the `/web/flows` route with list and detail layout.

#### Scenario: Route accessible

- **WHEN** user navigates to `/web/flows`
- **THEN** system displays the flows view with navigation item highlighted

#### Scenario: Flow list displays

- **WHEN** workspace has execution flows
- **THEN** list shows each flow with entry→terminal label, flow_type badge, and step count

### Requirement: Flow detail visualization

The system SHALL display flow steps as a horizontal chain when a flow is selected.

#### Scenario: Click expands flow

- **WHEN** user clicks a flow in the list
- **THEN** view expands to show step-by-step call chain

#### Scenario: Step chain visualization

- **WHEN** flow detail is expanded
- **THEN** steps render as horizontal boxes connected by arrows in stepIndex order

#### Scenario: Step information

- **WHEN** step boxes render
- **THEN** each box displays symbolName and filePath

### Requirement: Flow filtering

The system SHALL allow filtering flows by type and symbol.

#### Scenario: Filter by flow type

- **WHEN** user selects flow_type filter (intra_community or cross_community)
- **THEN** list shows only flows matching selected type

#### Scenario: Search by symbol name

- **WHEN** user enters text in symbol search
- **THEN** list shows only flows containing matching entry or terminal symbol

#### Scenario: Filter by file path

- **WHEN** user enters file path filter
- **THEN** list shows only flows with steps in matching files

### Requirement: Pagination

The system SHALL paginate flows for performance.

#### Scenario: Page size

- **WHEN** workspace has more than 20 flows
- **THEN** list displays 20 flows per page with pagination controls

#### Scenario: Lazy load steps

- **WHEN** flow is expanded
- **THEN** steps load on demand (not pre-loaded for all flows)

### Requirement: Empty state handling

The system SHALL display appropriate messaging when no flows exist.

#### Scenario: No flows detected

- **WHEN** workspace has no execution flows
- **THEN** view displays empty state message
