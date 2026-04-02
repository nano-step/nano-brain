# RRI-T Phase 2: DISCOVER — nano-brain-web

## Persona 1: End User (Developer using the dashboard)

| # | Scenario | Dim |
|---|----------|-----|
| 1 | Can I search memories and see results instantly? | D1 |
| 2 | Does the knowledge graph render without freezing on 1000+ entities? | D3 |
| 3 | Can I click a graph node and see its details? | D1 |
| 4 | Does workspace selector filter all views correctly? | D2 |
| 5 | Does search handle Vietnamese/Unicode input? | D7 |
| 6 | Can I navigate between all 8 views without errors? | D1 |
| 7 | Is the flow list filterable by type and name? | D1 |
| 8 | Do infrastructure patterns expand/collapse correctly? | D1 |

## Persona 2: Business Analyst

| # | Scenario | Dim |
|---|----------|-----|
| 1 | Does the dashboard show accurate document/embedding counts? | D5 |
| 2 | Are bandit stats visualized correctly for decision making? | D2 |
| 3 | Can I see the expand rate and preference weights? | D2 |
| 4 | Does the uptime display update correctly? | D2 |

## Persona 3: QA Destroyer

| # | Scenario | Dim |
|---|----------|-----|
| 1 | What happens if the API server is down? | D6 |
| 2 | What if the API returns empty data for all endpoints? | D7 |
| 3 | What if graph has 5000+ nodes? | D3 |
| 4 | What if search query is XSS: `<script>alert(1)</script>`? | D4 |
| 5 | What if workspace hash doesn't match any backend data? | D7 |
| 6 | What if the API returns malformed JSON? | D6 |
| 7 | What if I rapidly switch between views? | D3 |
| 8 | What if connection strength is 0 or negative? | D7 |
| 9 | What if flow has 0 steps? | D7 |
| 10 | What if search returns 1000+ results? | D3 |

## Persona 4: DevOps Tester

| # | Scenario | Dim |
|---|----------|-----|
| 1 | Does the web build produce valid static assets? | D6 |
| 2 | Does the Vite proxy correctly route API calls? | D6 |
| 3 | Does the SPA fallback work for direct URL access? | D6 |
| 4 | Are CORS headers correct for localhost origins? | D4 |
| 5 | Does the web UI work behind a reverse proxy? | D6 |

## Persona 5: Security Auditor

| # | Scenario | Dim |
|---|----------|-----|
| 1 | Is user input in search sanitized against XSS? | D4 |
| 2 | Are API responses rendered safely (no innerHTML)? | D4 |
| 3 | Does CORS reject non-localhost origins? | D4 |
| 4 | Are error messages safe (no stack traces to client)? | D4 |
| 5 | Can a malicious node label in the graph execute JS? | D4 |
