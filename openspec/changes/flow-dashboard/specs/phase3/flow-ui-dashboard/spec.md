## ADDED Requirements

### Requirement: Serve a web dashboard for flow visualization
The system SHALL expose a `GET /api/v1/graph/flow/ui` endpoint that serves an HTML page for browsing and viewing execution flows.

#### Scenario: Dashboard loads
- **WHEN** a browser navigates to `GET /api/v1/graph/flow/ui?workspace=<hash>`
- **THEN** the response is an HTML page with embedded CSS/JS that renders a flow dashboard

#### Scenario: Pre-select a flow
- **WHEN** the URL includes `?entry=POST /api/v1/query`
- **THEN** the dashboard loads with that flow already selected and rendered

#### Scenario: Mobile responsive
- **WHEN** the page is viewed on a mobile device
- **THEN** the layout adapts to a single-column view with collapsible sidebar

### Requirement: Render Mermaid diagrams inline
The dashboard SHALL render Mermaid flowcharts directly in the browser using Mermaid.js loaded from CDN.

#### Scenario: Diagram renders
- **WHEN** a user selects an endpoint from the list
- **THEN** the Mermaid diagram for that flow renders in the main content area

#### Scenario: Diagram updates on selection
- **WHEN** a user clicks a different endpoint
- **THEN** the diagram updates to show the newly selected flow

### Requirement: Client-side search and filter
The dashboard SHALL support client-side search to filter the endpoint list.

#### Scenario: Search by path
- **WHEN** the user types "query" in the search box
- **THEN** the endpoint list filters to show only endpoints containing "query" in the path

#### Scenario: Search by handler
- **WHEN** the user types "WriteDocument" in the search box
- **THEN** the endpoint list filters to show only endpoints with "WriteDocument" as the handler

#### Scenario: Clear search
- **WHEN** the user clears the search box
- **THEN** all endpoints are shown again
