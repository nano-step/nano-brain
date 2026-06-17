## REMOVED Requirements

### Requirement: Embedded SPA at /ui
**Reason**: The UI is moving to a separate repository (`nano-brain-dashboard`) and will be served by a standalone local server via `npx nano-brain-dashboard`. The embedded SPA in the Go binary is no longer needed.

**Migration**: Users should install and run `npx @nano-step/nano-brain-dashboard` to access the dashboard. The API at `/api/v1/*` continues to function without the UI.

#### Scenario: /ui no longer served
- **WHEN** a browser issues `GET /ui`
- **THEN** the server returns `404 Not Found`

#### Scenario: API continues working
- **WHEN** a client issues `POST /api/v1/query`
- **THEN** the server processes the request normally
- **AND** the response is returned

### Requirement: Security headers on UI routes
**Reason**: The `/ui` route group is removed. Security headers for the API are out of scope for this change.

**Migration**: No migration needed. The API does not require UI-specific security headers.

#### Scenario: No UI security headers
- **WHEN** a client issues `GET /api/status`
- **THEN** the response does NOT include `Content-Security-Policy`, `X-Frame-Options`, or `Referrer-Policy` headers
