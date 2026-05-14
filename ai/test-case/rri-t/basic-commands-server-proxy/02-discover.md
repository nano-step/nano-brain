# RRI-T Phase 2: DISCOVER — basic-commands-server-proxy

**Feature:** basic-commands-server-proxy
**Date:** 2026-05-14

## Interview Summary

| Persona | Questions | Key Concerns |
|---------|-----------|--------------|
| End User (AI Agent) | 10 | Silent failure when server down; output format parity |
| Business Analyst | 8 | Workspace isolation; data correctness vs direct SQLite |
| QA Destroyer | 10 | Server errors, malformed JSON, timeout, wrong port |
| DevOps Tester | 8 | Container detection, host daemon lifecycle, version mismatch |
| Security Auditor | 6 | Cross-workspace leakage, error message exposure |
| **Total** | **42** | |

---

## Persona 1: End User — AI Agent in OpenCode container

### Context
I am the OpenCode agent running inside a Docker container. I call `npx nano-brain tags`,
`npx nano-brain update`, and `npx nano-brain status` as part of my session startup and
memory management routines. I need these commands to work reliably and silently route to
the host daemon.

### Questions

1. What happens when I run `npx nano-brain tags` inside the container and the host daemon IS running?
2. What happens when I run `npx nano-brain tags` and the host daemon is NOT running?
3. What happens when `tags` returns an empty list (no tags indexed yet)?
4. What happens when I run `npx nano-brain update` to trigger a reindex from inside the container?
5. What happens when `update` is called while the server is already reindexing?
6. What happens when `npx nano-brain status` is called — does it show server-side stats or stale local data?
7. What happens when the server responds slowly (5+ seconds) to `tags`?
8. What happens when I run `tags` and `update` simultaneously from two agent processes?
9. What happens when tags contain Vietnamese characters (ký tự đặc biệt)?
10. What happens when I run `npx nano-brain tags` OUTSIDE a container (local dev)?

### Key Concerns
- Silent fallback to local SQLite must NOT happen in container mode
- Output format must be identical to pre-proxy behaviour (agents parse the output)
- `update` must not block the agent if server is busy

---

## Persona 2: Business Analyst

### Context
I need to verify that the proxied commands return data that is semantically identical
to what direct SQLite access returned. Any change in output shape or content would
break downstream agents that depend on consistent CLI output.

### Questions

1. What happens when `tags` proxy returns tags from a different workspace than the current directory?
2. What happens when `update` proxy triggers reindex of the correct workspace (based on cwd)?
3. What happens when `status` no longer shows local DB path — do agents that parse status output break?
4. What happens when the tag list has 0 tags vs null vs empty array from the server?
5. What happens when `update` succeeds on server but the CLI shows no confirmation message?
6. What happens when the server returns tags sorted differently than direct SQLite did?
7. What happens when a workspace has never been indexed and `tags` is called?
8. What happens when `status` used to show local SQLite path but now only shows server data?

### Key Concerns
- Workspace context must be passed correctly in proxy requests
- Output format (text, not JSON) must remain backward-compatible
- `update` acknowledgement must be clear to the calling agent

---

## Persona 3: QA Destroyer

### Context
My job is to break the proxy layer. I will send malformed requests, kill the server
mid-request, overflow inputs, and generally cause chaos to find what breaks.

### Questions

1. What happens when the server returns HTTP 500 for `GET /api/tags`?
2. What happens when the server returns malformed JSON (truncated body)?
3. What happens when the server returns HTTP 200 but with an empty body?
4. What happens when the proxy connects to the wrong port (3101 instead of 3100)?
5. What happens when `POST /api/update` receives a workspace path with path traversal (`../../etc`)?
6. What happens when the server returns a `tags` array with 10,000 entries?
7. What happens when the HTTP request times out (server hangs for 30+ seconds)?
8. What happens when the server returns HTTP 301 redirect for `GET /api/tags`?
9. What happens when `update` is called with `--force` flag that the server doesn't understand?
10. What happens when the server closes the TCP connection mid-response?

### Key Concerns
- Errors must not crash the CLI process (exit code should be non-zero but controlled)
- No silent data corruption — prefer loud failure over silent wrong data
- Timeout must be bounded (CLI must not hang indefinitely)

---

## Persona 4: DevOps Tester

### Context
I am responsible for the container lifecycle. I start and stop the host daemon, update
nano-brain versions, and need to verify the CLI handles all deployment states correctly.

### Questions

1. What happens when `nano-brain docker start` is run, then `tags` is immediately called (server still booting)?
2. What happens when the host daemon is stopped mid-flight (`docker stop nano-brain`) while `update` is running?
3. What happens when the CLI version inside the container is newer than the server version on host?
4. What happens when `/.dockerenv` exists but `host.docker.internal` DNS resolution fails?
5. What happens when the server is on a non-default port (`NANO_BRAIN_PORT=3200`) and the CLI uses default 3100?
6. What happens when the bind mount (`/home/agent/.nano-brain`) is missing (misconfigured container)?
7. What happens when `status` is called right after `docker restart nano-brain` (server in warm-up)?
8. What happens when both `NANO_BRAIN_DIRECT=1` and `isInsideContainer()=true` — which wins?

### Key Concerns
- CLI must give actionable error message when server is unreachable ("run nano-brain docker start on host")
- Port configuration must be consistent between server and CLI
- `NANO_BRAIN_DIRECT=1` override must still work for debugging

---

## Persona 5: Security Auditor

### Context
I need to verify that the new HTTP endpoints don't expose data beyond what the CLI
already exposed, and that workspace isolation is maintained across the proxy boundary.

### Questions

1. What happens when `GET /api/tags` is called without a workspace header — does it return all tags from all workspaces?
2. What happens when a crafted request sends a workspace path pointing to another user's project?
3. What happens when `POST /api/update` is called repeatedly (50x/sec) — is there rate limiting or resource exhaustion?
4. What happens when the server error response includes internal file paths or stack traces?
5. What happens when the proxy forwards unexpected headers that leak environment details?
6. What happens when `GET /api/tags` response is cached by an intermediate proxy and served stale?

### Key Concerns
- Workspace context in HTTP requests must be validated server-side
- Error responses must not expose internal paths
- No unintended DoS vector via repeated `update` calls

---

## Raw Test Ideas (Consolidated)

| # | Idea | Source Persona | Dimension | Priority Estimate |
|---|------|---------------|-----------|-------------------|
| 1 | `tags` proxy returns correct tag list when server running | End User | D2: API | P0 |
| 2 | `tags` fails gracefully when server not running | End User | D6: Infra | P0 |
| 3 | `update` proxy triggers server-side reindex for correct workspace | Business Analyst | D2: API | P0 |
| 4 | `status` returns server data only (no local DB fallback) | End User | D5: Data | P0 |
| 5 | Server HTTP 500 on `tags` → CLI exits non-zero with message | QA Destroyer | D7: Edge | P0 |
| 6 | Malformed JSON response → CLI exits non-zero, no panic | QA Destroyer | D7: Edge | P1 |
| 7 | `tags` data parity: proxy vs direct SQLite returns same tags | Business Analyst | D5: Data | P0 |
| 8 | `update` with no collections → server handles gracefully | Business Analyst | D7: Edge | P1 |
| 9 | Server not yet ready (booting) → CLI retries or clear error | DevOps | D6: Infra | P1 |
| 10 | `tags` outside container → local SQLite path unchanged | End User | D6: Infra | P0 |
| 11 | Timeout on `tags` → CLI exits after bounded wait | QA Destroyer | D3: Perf | P1 |
| 12 | `GET /api/tags` no cross-workspace leakage | Security | D4: Security | P1 |
| 13 | Vietnamese tag names preserved through proxy round-trip | End User | D5: Data | P1 |
| 14 | `update` repeated calls → server not overwhelmed | Security | D4: Security | P2 |
| 15 | `status` output format backward-compatible after removing local fallback | Business Analyst | D5: Data | P1 |
| 16 | Server returns empty tags → CLI shows "(no tags)" not error | End User | D7: Edge | P1 |
| 17 | `POST /api/update` workspace path validation (no traversal) | Security | D4: Security | P1 |
| 18 | CLI version mismatch vs server → commands still work | DevOps | D6: Infra | P2 |
| 19 | `/.dockerenv` present but host.docker.internal unreachable → clear error | DevOps | D6: Infra | P1 |
| 20 | HTTP redirect on `GET /api/tags` → CLI does not follow blindly | QA Destroyer | D7: Edge | P2 |
