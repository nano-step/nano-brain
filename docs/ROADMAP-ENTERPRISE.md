# Enterprise roadmap: nano-brain + nano-brain-observe

> Status: proposal, 2026-11-28. Nothing here is built unless marked ✅. This file replaces the auth parts of [ROADMAP.md](ROADMAP.md) (Pillar 4, Phase 7, Phase 10 "Authentication & authorization").

## Summary
- **Goal:** one nano-brain deployment serves a whole company. Every engineer's agent signs in as a person, sees only the workspaces that person has been granted, and every write, delete and admin action is audited.
- **Today:** auth is optional: Basic users and static bearer tokens set in config. The identity a request carries is only a username or the literal `"token"`. There are no roles, no workspace scoping and no audit log.
- **Who does what:** nano-brain owns identity, authorization and audit, and is the system of record. nano-brain-observe stays an opt-in tool that captures payloads for quality work. A proxy can be bypassed, so it can never be the audit log.
- **New API endpoints:** about 15 are needed (see [New API endpoints](#new-api-endpoints)): auth discovery, `/me`, tokens, workspace members, audit, health and metrics. Search and graph need no new endpoints; they need permission checks.
- **Order:** E0 hardening → E1 identity → E2 authorization → E3 SSO and E4 audit (in parallel) → E5 provisioning. E6 deployment can start after E1. Work on the two blockers (workspace identity, push ingestion) runs alongside E1 and E2.

## Current state (measured 2026-11-28)
| Area | Today | Evidence |
|---|---|---|
| Authentication | Basic auth (bcrypt users in config) and static bearer tokens in config. Off by default. | `internal/server/middleware/auth.go`, `internal/config/template.go` |
| Identity on a request | The username, or the literal `"token"` for every bearer token. Set as `auth.principal` but never passed to handlers or MCP tools. | `auth.go` `validateBearer` |
| Authorization | None. Routes are already split into `api` / `data` / `write` groups, but one gate covers all of them. | `internal/server/routes.go` |
| Route grouping bugs | `POST /graph/neighborhood` and `GET /code/summarize/status` are reads that sit in the `write` group. | `routes.go:80,117` |
| Workspace identity | SHA-256 of an absolute path. The same repo cloned on two laptops gives two workspaces. | `memory_workspaces_resolve` |
| Ingestion | The watcher and the harvester read the server's own disk. | `internal/watcher`, `internal/harvest` |
| Audit | None. `telemetry_logs` records only 3 REST search handlers; the 19 MCP tools record nothing. | migration 00001, observe `docs/DESIGN.md` |
| Users, orgs, tokens in the DB | No tables. | `migrations/*.sql` |
| Known holes | #735: any 64-hex `workspace` value skips the DB check. #723: `reload-config` ignores `server.auth`. #673 and #674: open follow-ups on the HTTP surface and on auth. | GitHub issues |
| MCP OAuth discovery | Clients already probe `/.well-known/oauth-*` and get 404. Before observe PR #7 stopped logging these probes, they made up about 49% of its records. | observe logs |

## Blockers to decide before building auth
1. **Workspace identity is tied to a path on one machine.** A team needs a logical id: the normalized git remote URL, plus a subpath for monorepos. Each user's local path becomes an alias of that id. Without this, a permission is granted on a hash that nobody else's machine produces. (Q3)
2. **Ingestion reads the server's own disk.** A shared server cannot read engineers' laptops. Team mode needs push ingestion: a thin client (`nano-brain agent`) on each machine watches and harvests locally, then pushes to the server with that user's token. Single-user mode keeps today's in-process watcher.
3. **Tenancy model.** Recommended: one tenant per deployment, meaning one company runs one server with one Postgres. That avoids an `org_id` column on every table and avoids row-level security. Multi-tenant SaaS comes later, with one database per tenant. (Q1)

## Phases
### E0 — Hardening prerequisites (no new features)
- Close #735: a `workspace` must exist in the DB, and from E2 on it must also be granted to the caller. Close #723 (reloading auth config), #673 and #674.
- Body size limit and server timeouts on every route.
- `/healthz` (liveness), `/readyz` (checks Postgres and the embedder), Prometheus `/metrics`.
- Move all routes under `/api/v1`. `/api/status`, `/api/version`, `/api/harvest` and `/api/reload-config` are outside it today. Keep the old paths as aliases for one release.
- **Done when:** these security issues are closed and Kubernetes probes work.

### E1 — Identity and tokens
- New tables: `users`, and `api_tokens` (id, user_id, name, sha256 of the secret, scopes, expires_at, last_used_at, revoked_at).
- Token format `nbt_<public-id>_<secret>`. Only the hash of the secret is stored. The public id lets logs, including observe's, name the token without ever seeing the secret.
- A `Principal{UserID, TokenID, Scopes, Via}` value on the request context, passed through to MCP tool handlers.
- Users and tokens in the config file become bootstrap and break-glass credentials only.
- CLI: `nano-brain token create|list|revoke`, `nano-brain user add|disable`.
- **Done when:** every request is tied to a user id, and observe logs show the token id.

### E2 — Authorization (roles + workspace grants)
- Org-level roles: `owner`, `admin`, `member`, `viewer`. Workspace grants: a user or group gets `reader`, `writer` or `maintainer` on a workspace.
- What a token can do = the user's role, limited further by the token's scopes.
- Checks run in three places only: the REST route groups, a per-tool check in the MCP layer, and one workspace check where the `workspace` argument is resolved. No checks scattered through handlers.
- MCP `tools/list` hides tools the caller cannot call. `memory_workspaces_list` returns only granted workspaces.
- Move the misgrouped read routes out of the `write` group.
- **Done when:** integration tests show a viewer cannot write, and a user without a grant gets 404 rather than 403, so the workspace's existence does not leak.

### E3 — SSO via OAuth 2.1 (MCP authorization spec)
- nano-brain is an OAuth **resource server**, not an authorization server. Sign-in goes to the company IdP (Okta, Entra ID, Google Workspace, Keycloak, Auth0) over OIDC.
- Follow the MCP authorization spec:
  - serve `GET /.well-known/oauth-protected-resource` (RFC 9728), naming the IdP;
  - on 401, return `WWW-Authenticate: Bearer resource_metadata="…"`;
  - validate each JWT against the IdP's JWKS: `iss`, `exp`, scopes, and an `aud` equal to this server's resource URI (RFC 8707). Never pass tokens through to anything else.
- Map `sub` or email to a `users` row, creating it on first login when the email domain is on an allow-list.
- MCP clients that support OAuth sign in through the browser. Headless agents and CI keep using personal access tokens.
- The web UI, if it stays, signs in with the authorization-code flow plus PKCE and keeps a server-side session cookie (HttpOnly, SameSite=Lax).
- **Done when:** an agent connects to a remote nano-brain with only its URL plus a browser login.

### E4 — Audit and compliance
- An append-only `audit_events` table: ts, user_id, token_id, action (tool or route), workspace, target ids, result, ip, user_agent, request_id. No payloads.
- Always audited: writes, updates, deletes, admin actions, sign-ins and failed auth, token create and revoke, grant changes. Reads are audited only when `audit.reads: true`, because of volume. (Q5)
- `GET /api/v1/audit` with filters, an NDJSON export, a retention setting, and an optional stream to a SIEM (stdout JSON or a webhook).
- Record who wrote what: `created_by` and `updated_by` on documents (#381).
- Data governance: export a workspace, delete everything authored by a user, set retention per collection (for example, sessions).

### E5 — Org admin and provisioning
- Groups: the IdP's `groups` claim maps to nano-brain groups, so access is granted to teams rather than to individuals.
- SCIM 2.0 (`/scim/v2/Users`, `/scim/v2/Groups`). When someone leaves the company, their tokens must be revoked automatically.
- Admin UI pages: users, tokens, grants, audit.
- Usage per user, built from `code_summarization_usage` (which already exists) and embedding counts. Quotas only if a customer asks.

### E6 — Deployment and operations (can start after E1)
- Helm chart, a published Docker image, and a guide for managed Postgres (pgvector must be available).
- TLS terminates at the ingress; no native TLS.
- Horizontal scaling: N replicas with `serve_only: true`, plus one worker replica. `serve_only` already exists.
- OpenTelemetry traces, and a documented backup and restore procedure.

## Permission model
Roles map to permissions as follows: `viewer` = read; `member` = read + write; a `maintainer` grant adds maintain on that workspace; `admin` and `owner` = everything.

| Permission | MCP tools | REST routes |
|---|---|---|
| `memory:read` | query, search, vsearch, get, wake_up, tags, ticket, graph, impact, trace, symbols, flow, flowchart, workspaces_list, workspaces_resolve, status | `data` group reads: `/query`, `/search`, `/vsearch`, `/get`, `/multi-get`, `/graph/*`, `/symbols`, `/links/*`, `/wake-up`, `/events` |
| `memory:write` | write, update, delete | `/write`, `DELETE /documents/:id` |
| `workspace:maintain` | — | `/reindex`, `/reindex-cfg`, `/update`, `/embed`, `/summarize`, `/code/summarize*`, `/flow/materialize`, `/graph/pagerank/compute`, collections POST/PUT/DELETE, `/harvest` |
| `admin` | — | `/init`, `DELETE /workspaces/:hash`, `/reset-workspace`, `/config`, `/reload-config`, `/doctor`, and the users, tokens, grants and audit APIs |

All 19 MCP tools are in the table. No MCP tool needs `workspace:maintain` or `admin`, so agents with a normal token never see admin operations.

## New API endpoints
**Yes, new endpoints are needed.** None of them are for search or graph; those get permission checks instead.

| Phase | Endpoint | Purpose |
|---|---|---|
| E0 | `GET /healthz`, `GET /readyz` | Liveness and readiness for Kubernetes and load balancers |
| E0 | `GET /metrics` | Prometheus: request rate, latency per tool, embed queue depth |
| E1 | `GET /api/v1/me` | Who the caller is, plus their role, scopes and workspaces. Agents and the UI use it to check access. |
| E1 | `GET/POST /api/v1/tokens`, `DELETE /api/v1/tokens/:id` | Personal access tokens, self-service. The secret is shown once, at creation. |
| E1 | `GET/POST /api/v1/users`, `PATCH/DELETE /api/v1/users/:id` | Admin: invite, change role, disable |
| E2 | `GET/PUT/DELETE /api/v1/workspaces/:hash/members` | Workspace grants for users and groups |
| E3 | `GET /.well-known/oauth-protected-resource` | MCP OAuth discovery (RFC 9728). Clients already probe this path today. |
| E3 | `GET /auth/login`, `GET /auth/callback`, `POST /auth/logout` | Web UI sign-in only. Agents never use these. |
| E4 | `GET /api/v1/audit`, `GET /api/v1/audit/export` | Query the audit log; NDJSON export for a SIEM |
| E4 | `POST /api/v1/workspaces/:hash/export` | Export one workspace's data |
| E5 | `/scim/v2/Users`, `/scim/v2/Groups` | Provisioning and deprovisioning from the IdP |
| E5 | `GET /api/v1/usage` | Usage per user and per workspace |
| Blocker 2 | `POST /api/v1/ingest/batch` | Push ingestion from `nano-brain agent`: documents plus deletions, idempotent by content hash. Session summaries reuse it with `collection=sessions`. |

**Not adding:** a role-CRUD API (roles are fixed), GraphQL, or new MCP tools. `/api/v1/me` is enough for an agent to check its own access.

## nano-brain-observe in an enterprise deployment
**Role:** an opt-in tool for quality R&D. It is never the system of record. Audit and metrics live in nano-brain (E4, E0), because a client can bypass a proxy by connecting to nano-brain directly. Details: the nano-brain-observe repo's `docs/ROADMAP.md`.

| Phase | Work | Depends on |
|---|---|---|
| O1 | Open issues: #3 (redact credentials in args and responses, never log the `Authorization` header, retention, private file permissions) and #5 (installer) | — |
| O2 | Log who made each call: the token's public id from `nbt_<id>_…`, or the `sub` of a JWT, marked as *claimed* because the proxy does not validate it. Tests that OAuth discovery and `WWW-Authenticate` pass through unchanged. Keep dropping only 404 probes. | E1, E3 |
| O3 | Deployable: a stdout sink for Kubernetes log collectors, sampling, metadata only by default with payload capture opt-in per workspace, and a sidecar option in the Helm chart | E6 |
| O4 | Quality reports: `stats` grouped by caller and by workspace, plus a report of empty-result queries, failing arguments and slow tools to feed nano-brain issues | O2 |

Overlap: #728 moves User-Agent attribution and a usage report into nano-brain itself. Once E0 metrics and E4 audit ship, observe's metadata-only mode duplicates them; what stays unique to observe is capturing payloads. (Q4)

## Out of scope
- **Running our own OAuth authorization server, or hosting dynamic client registration.** The company IdP does this better, and doing it ourselves is a large security surface.
- **Native SAML.** Customers who only have SAML put an IdP broker (Keycloak) in front.
- **Row-level multi-tenancy in v1.** See blocker 3.
- **Access control per collection or per document.** The workspace is the unit of access. (Q7 in ROADMAP.md is answered: workspace level is enough to start.)
- **Native TLS.** TLS ends at the ingress or a reverse proxy (Q8 in ROADMAP.md is answered).

## Open questions (owner decision)
- **Q1 Tenancy:** one tenant per self-hosted deployment first (recommended), or multi-tenant SaaS from the start?
- **Q2 Licensing:** do the enterprise features stay MIT, or go open-core with a separate license?
- **Q3 Workspace identity:** is a normalized git remote URL acceptable as the key? How should monorepo subpaths and repos with no remote be handled?
- **Q4 Observe long term:** keep it as a separate repo, or fold payload capture into nano-brain behind a flag?
- **Q5 Read auditing:** off by default (less volume) or on by default (stricter compliance)?
- **Q6 First IdPs:** which ones to support and test first: Google Workspace, Entra ID, Okta?
