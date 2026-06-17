## ADDED Requirements

### Requirement: npx entry point
The `@nano-step/nano-brain-dashboard` npm package SHALL expose a `bin` entry that starts a local Node.js server serving the built SPA and proxying `/api/*` requests to the nano-brain backend.

#### Scenario: npx starts server on default port
- **WHEN** user runs `npx nano-brain-dashboard`
- **THEN** a server starts on port 4321
- **AND** the SPA is served at `http://localhost:4321`
- **AND** `/api/*` requests are proxied to `http://localhost:3100`

#### Scenario: npx accepts custom port
- **WHEN** user runs `npx nano-brain-dashboard --port 8080`
- **THEN** the server starts on port 8080

#### Scenario: npx accepts custom API base
- **WHEN** user runs `npx nano-brain-dashboard --api-base http://192.168.1.100:3100`
- **THEN** `/api/*` requests are proxied to the specified address

#### Scenario: npx accepts API token
- **WHEN** user runs `npx nano-brain-dashboard --api-token nbt_xxxx`
- **THEN** all proxied requests include `Authorization: Bearer nbt_xxxx` header
- **AND** the token is NOT logged or exposed in the browser

#### Scenario: Port conflict falls back gracefully
- **WHEN** the default port 4321 is already in use
- **THEN** the server tries ports 4322, 4323, ... up to 4326
- **AND** if all fail, prints a clear error: "Ports 4321-4326 are in use. Use --port to specify a different port."
- **AND** exits with code 1

#### Scenario: Backend unreachable on startup
- **WHEN** the server starts but `http://localhost:3100/health` is unreachable
- **THEN** the server still starts and serves the SPA
- **AND** a banner is injected showing "nano-brain API is not running at localhost:3100. Start it with `nano-brain serve`."
- **AND** the banner auto-hides when the backend comes online (poll every 30s)

### Requirement: Proxy routing
The local server SHALL proxy all requests matching `/api/*` to the configured backend URL, preserving headers, method, and body. The proxy SHALL handle both regular HTTP requests and SSE (Server-Sent Events) streams.

#### Scenario: API request proxied correctly
- **WHEN** the browser sends `POST /api/v1/query` with JSON body
- **THEN** the server forwards the request to `http://localhost:3100/api/v1/query`
- **AND** the response is returned to the browser

#### Scenario: SSE stream proxied correctly
- **WHEN** the browser opens `GET /api/v1/events` (SSE)
- **THEN** the server establishes a streaming connection to the backend
- **AND** events are forwarded to the browser in real-time
- **AND** the connection stays open for at least 60 seconds without timeout

#### Scenario: SSE reconnects after proxy restart
- **WHEN** the proxy server restarts while an SSE connection is active
- **THEN** the browser's EventSource automatically reconnects
- **AND** new events resume streaming within 5 seconds

#### Scenario: Auth header forwarded through proxy
- **WHEN** the browser sends a request with `Authorization: Bearer <token>` header
- **THEN** the proxy forwards the header to the backend unchanged
- **AND** the backend processes the request with the provided credentials

### Requirement: SPA fallback routing
The local server SHALL serve `index.html` for any path under `/` that does not match a static asset or `/api/*`, enabling client-side routing.

#### Scenario: Client route loads index
- **WHEN** user navigates to `http://localhost:4321/memory/abc-123`
- **AND** no static file matches that path
- **THEN** the server returns `index.html`
- **AND** the client router resolves `/memory/abc-123`

### Requirement: Security headers
The local server SHALL set security headers on all responses: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: same-origin`. CSP SHALL be set via `<meta>` tag in `index.html`.

#### Scenario: Static assets have security headers
- **WHEN** the browser requests a static asset (JS, CSS, image)
- **THEN** the response includes `X-Content-Type-Options: nosniff`
- **AND** `X-Frame-Options: DENY`
- **AND** `Cache-Control: public, max-age=31536000, immutable` for hashed assets

### Requirement: Build output
The npm package SHALL include a pre-built `dist/` directory with the production SPA. The `bin` entry SHALL serve this `dist/` directory.

#### Scenario: Package installs and runs
- **WHEN** user runs `npm install -g @nano-step/nano-brain-dashboard`
- **THEN** the `nano-brain-dashboard` command is available
- **AND** running it serves the pre-built SPA

### Requirement: Vite dev server proxy
The Vite development configuration SHALL proxy `/api/*` to `VITE_API_BASE` (default `http://localhost:3100`), matching the production proxy behavior.

#### Scenario: Dev mode proxy works
- **WHEN** developer runs `npm run dev`
- **THEN** `/api/status` requests are proxied to the backend
- **AND** the app renders the status data

### Requirement: Offline and fallback documentation
The README SHALL document fallback paths for users without Node.js or behind corporate proxies blocking npm.

#### Scenario: Docker alternative documented
- **WHEN** a user cannot run `npx nano-brain-dashboard` (no Node.js or npm blocked)
- **THEN** the README provides a Docker alternative: `docker run -p 4321:4321 nano-step/nano-brain-dashboard`
- **AND** documents building from source: `git clone && npm install && npm run build && node server.js`

#### Scenario: Known limitations documented
- **WHEN** a user reads the README
- **THEN** it states that the dashboard requires Node.js 18+ or Docker
- **AND** it lists the minimum system requirements
