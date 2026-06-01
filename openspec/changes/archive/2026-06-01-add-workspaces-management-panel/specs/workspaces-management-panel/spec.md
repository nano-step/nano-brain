# Spec — Workspaces Management Panel

## ADDED Requirements

### Requirement: UI Route

The web UI SHALL expose a route at `/ui/workspaces` that renders the Workspaces Management Panel. The route SHALL be lazy-loaded.

#### Scenario: User navigates to /ui/workspaces

- **WHEN** the user navigates to `/ui/workspaces`
- **THEN** the panel loads and fetches workspace list via `GET /api/v1/workspaces`
- **AND** displays a table with one row per workspace

### Requirement: Workspace Table Columns

The table SHALL display, for each workspace:
- `name` (workspace human name)
- `hash` (truncated to first 16 chars + ellipsis, with full hash on hover/tooltip)
- `doc_count` (numeric)
- `chunk_count` (numeric)
- Action buttons (`Switch`, `Remove`)

#### Scenario: Workspace has zero docs

- **WHEN** a workspace has `doc_count == 0`
- **THEN** the row still renders with `0` shown in the docs column (not hidden)

### Requirement: Switch Action

Each row SHALL provide a `Switch` button that activates the row's workspace as the current one.

#### Scenario: Switch to a different workspace

- **WHEN** the user clicks `Switch` on a non-active row
- **THEN** the workspace cookie is updated to that row's hash
- **AND** React Query caches are invalidated so subsequent panels show data scoped to the new workspace

#### Scenario: Switch to currently active workspace

- **WHEN** the user clicks `Switch` on the row whose hash equals the currently active workspace
- **THEN** the button is disabled or labeled "Current"

### Requirement: Remove Action with Confirmation

Each row SHALL provide a `Remove` button that opens a confirmation dialog. The dialog SHALL require the user to type the exact workspace `name` to enable the confirm button.

#### Scenario: User clicks Remove

- **WHEN** the user clicks `Remove` on a row
- **THEN** a modal opens showing the workspace name + description "This will permanently delete workspace X and all its data. This cannot be undone."

#### Scenario: User types wrong name

- **WHEN** the typed text does not exactly equal the workspace name
- **THEN** the confirm button is disabled

#### Scenario: User types correct name and confirms

- **WHEN** the typed text equals the workspace name AND the user clicks confirm
- **THEN** the UI calls `DELETE /api/v1/workspaces/:hash`
- **AND** on 2xx response: closes the dialog, refetches the workspace list, removes the row from the table
- **AND** on error response: keeps the dialog open, displays the error message

### Requirement: Removing the Currently-Active Workspace

When the user removes the workspace they are currently viewing, the UI SHALL transition the user to another valid workspace (or an empty state if none remain) so the user is not stranded on a deleted workspace whose API calls would 4xx.

#### Scenario: User removes the currently active workspace

- **WHEN** the deleted workspace's hash equals the currently active workspace
- **AND** at least one other workspace remains
- **THEN** the UI sets the workspace cookie to the first remaining workspace's hash
- **AND** invalidates React Query caches (or reloads) so all panels re-render under the new active workspace

#### Scenario: User removes the last workspace

- **WHEN** the deleted workspace is the only workspace remaining
- **THEN** the UI shows an empty-state message explaining how to register a new workspace (CLI command + API URL)
- **AND** the workspace cookie is cleared

### Requirement: Navigation Entry

A main navigation link labeled `Workspaces` SHALL be added with keyboard shortcut `g w`, placed alongside other panel links (Dashboard, Memory, Graph, Symbols, Harvest, Settings).

#### Scenario: User presses keyboard shortcut

- **WHEN** the user presses `g` then `w` (with no input field focused)
- **THEN** the router navigates to `/ui/workspaces`

### Requirement: Loading and Error States

The panel SHALL surface async state for the workspace list fetch so the user can distinguish "still loading" from "no workspaces" from "fetch failed".

#### Scenario: List fetch in progress

- **WHEN** `GET /api/v1/workspaces` is loading
- **THEN** the table shows a skeleton or spinner placeholder

#### Scenario: List fetch fails

- **WHEN** `GET /api/v1/workspaces` returns non-2xx or network error
- **THEN** the panel shows an inline error message with a `Retry` button that refetches
