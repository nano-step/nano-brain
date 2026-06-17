## ADDED Requirements

### Requirement: Dashboard repo structure
The `nano-brain-dashboard` repository SHALL contain a Vite 5 + React 18 + TypeScript project with the following structure: `src/api/` (client + connection/auth store), `src/components/graph/` (GraphCanvas + adapters), `src/panels/*` (rebuilt panels), `src/app/` (router/layout/theme), and design tokens.

#### Scenario: Project builds and typechecks
- **WHEN** a developer runs `npm install && npm run build`
- **THEN** the build completes with zero TypeScript errors
- **AND** the output is a static SPA in `dist/`

#### Scenario: Dev server starts with proxy
- **WHEN** a developer runs `npm run dev`
- **THEN** the app is served on `http://localhost:5173`
- **AND** requests to `/api/*` are proxied to `http://localhost:3100`

### Requirement: API client ported from nano-brain
The dashboard SHALL use API client code ported from `web/src/api/client.ts` and `web/src/api/types.ts` with the base URL changed to relative `/api` (proxy handles routing).

#### Scenario: Client fetches data through proxy
- **WHEN** the dashboard calls `GET /api/status`
- **THEN** the request goes through the local proxy
- **AND** the response is rendered in the UI
- **AND** no CORS errors appear in the console

### Requirement: Connection and version banner
The dashboard SHALL display a connection status banner on first load that calls `/api/version`, compares the response against a `SUPPORTED_API_RANGE` constant, and shows a warning if the API version is outside the supported range.

#### Scenario: Version compatible
- **WHEN** the API version is within `SUPPORTED_API_RANGE`
- **THEN** the banner shows "Connected to API vX.Y.Z" in green

#### Scenario: Version incompatible
- **WHEN** the API version is outside `SUPPORTED_API_RANGE`
- **THEN** the banner shows a yellow warning with the detected version and supported range

#### Scenario: API unreachable
- **WHEN** the API is not running or unreachable
- **THEN** the banner shows a red error with "Cannot connect to nano-brain API"

### Requirement: Panel parity with /ui
The dashboard SHALL implement all panels from the current `/ui` with functional parity. Each panel scenario below defines the minimum acceptance criteria.

#### Scenario: Dashboard panel — stats and sparklines
- **WHEN** user navigates to Dashboard
- **THEN** workspace stats are displayed (doc count, chunk count, embedding status)
- **AND** sparkline charts show recent activity trends
- **AND** data is fetched from `GET /api/v1/stats`

#### Scenario: Memory panel — search and document drawer
- **WHEN** user enters a search query in Memory panel
- **THEN** results are returned via hybrid search (`POST /api/v1/query`)
- **AND** each result shows title, snippet, score, and tags
- **AND** clicking a result opens the document drawer with full content
- **AND** the document drawer shows backlinks and wikilink resolution

#### Scenario: Graph panel — knowledge graph visualization
- **WHEN** user navigates to Graph panel
- **THEN** the knowledge graph overview is rendered (`POST /api/v1/graph/overview`)
- **AND** nodes are colored by role (function, type, interface, etc.)
- **AND** a legend shows node role colors
- **AND** clicking a node opens the document drawer
- **AND** neighborhood expansion works (`POST /api/v1/graph/neighborhood`)

#### Scenario: Flow panel — endpoint flow visualization
- **WHEN** user selects an endpoint in Flow panel
- **THEN** the flow is rendered from `nodes[]`/`edges[]` (not Mermaid string)
- **AND** nodes show roles (handler, middleware, service)
- **AND** edges show kind (calls, conditional, middleware)
- **AND** conditional edges are displayed as dashed lines
- **AND** a "Copy Mermaid" button copies the `mermaid` field to clipboard

#### Scenario: Symbols panel — symbol search and display
- **WHEN** user searches for a symbol in Symbols panel
- **THEN** matching symbols are listed with kind (function, type, method)
- **AND** clicking a symbol shows its definition location and call graph

#### Scenario: Harvest panel — session list with live updates
- **WHEN** user navigates to Harvest panel
- **THEN** harvested sessions are listed with source, title, and timestamp
- **AND** SSE events (`/api/v1/events`) update the list in real-time
- **AND** clicking a session shows its summary or raw content

#### Scenario: Settings panel — configuration editing
- **WHEN** user navigates to Settings
- **THEN** all configuration options are displayed (read from `GET /api/v1/config`)
- **AND** secret fields show `"<redacted>"` placeholder
- **AND** changes persist via `POST /api/v1/config`
- **AND** invalid patches show validation errors

#### Scenario: Workspaces panel — workspace management
- **WHEN** user navigates to Workspaces
- **THEN** all registered workspaces are listed with document counts
- **AND** the current workspace is highlighted
- **AND** clicking a workspace switches the active workspace
- **AND** workspace deletion is available with confirmation

#### Scenario: CodeSummarize panel — code summarization trigger
- **WHEN** user navigates to CodeSummarize
- **THEN** the panel shows summarization status and controls
- **AND** triggering summarization sends `POST /api/v1/summarize`
- **AND** progress is displayed via SSE events
