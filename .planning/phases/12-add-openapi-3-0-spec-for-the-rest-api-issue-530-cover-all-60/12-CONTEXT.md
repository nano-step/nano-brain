# Phase 12: Add OpenAPI 3.0 spec for the REST API - Context

**Gathered:** 2026-07-02
**Status:** Ready for planning

<domain>
## Phase Boundary

nano-brain already exposes a full REST API (~60 endpoints under `/api/v1/*` plus a few top-level routes, see `internal/server/routes.go`) independent of the MCP server. This phase makes that API self-describing: an OpenAPI 3.0 document, generated from the existing route/handler code (not hand-maintained), served at runtime so any non-MCP HTTP client can discover the full endpoint surface without reading Go source.

Tracked via issue #530 (full acceptance criteria there).

Out of scope: MCP tool schemas (already self-describing via the MCP protocol's own `toolSchema` mechanism in `internal/mcp/tools.go`) — this phase is specifically about REST API discoverability for non-MCP clients.

</domain>

<decisions>
## Implementation Decisions

### Generation mechanism (locked by Tâm)
- **D-01:** Use `swaggo/swag` annotation-based generation — add `// @Summary`, `// @Router`, etc. doc-comments above each of the ~60 handler functions. `swag` derives request/response schemas automatically from the existing typed `XxxRequest`/`XxxResponse` structs' JSON tags (confirmed during discuss-phase: nearly every handler already has both typed, e.g. `QueryRequest`/`SearchResponse` in `internal/server/handlers/query.go`) — no need to hand-write schemas separately from the comments.
- **D-02:** JSON spec only — no bundled Swagger UI page. `GET /api/openapi.json` returns the raw spec; clients import it into their own tooling (Postman, their own Swagger UI instance, codegen).

### Scope (from issue #530)
- **D-03:** Cover all ~60 existing routes in `internal/server/routes.go`: top-level (`/health`, `/api/status`, `/api/version`), the full `/api/v1/*` group, and note `/mcp`/`/sse` exist but are protocol endpoints, not typical REST — document their presence but they don't need full request/response schemas (they're not JSON-request/response endpoints in the OpenAPI sense).
- **D-04:** Document auth/access requirements per route where middleware applies: `workspaceMiddleware` (workspace resolution), `workspaceRegisteredMiddleware` (write-path gate), `csrfMW` (CSRF token) — via `@Security`-style swag annotations, so the spec reflects real access requirements, not just happy-path shapes.
- **D-05:** Single source of truth with the route table — adding a new route without updating its swag annotations should be caught, ideally by a test (per issue acceptance criteria). Exact mechanism (e.g. a test asserting route count in `routes.go` matches annotated-endpoint count in the generated spec, or a `go generate` + `git diff --exit-code` CI-style check) is a research/planning decision — not locked here, but MUST exist in some automated form.
- **D-06:** Docs (README.md/SETUP_AGENT.md or similar) must mention how to fetch/browse the spec — per issue acceptance criteria.

### Claude's Discretion
- Exact `swag`-generated artifact layout (typically `docs/swagger.json`, `docs/swagger.yaml`, `docs/docs.go` — swag's conventional output) and whether the generated Go package is committed to the repo or built via `go generate`/Makefile target at build time — research should confirm swag's current conventions and this repo's existing `go generate` usage (if any) before locking.
- Whether `swag init` runs as a Makefile target, a `go:generate` directive, or a CI step — planner's call based on how this repo already handles other generated artifacts (e.g. `sqlc generate` convention already exists per AGENTS.md's Quick Reference).
- Whether the drift-detection test (D-05) is a Go test, a Makefile check, or a CI-only step — planner's call.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### This phase's tracking issue
- `https://github.com/nano-step/nano-brain/issues/530` — full intent, scope, acceptance criteria (verbatim source for D-03 through D-06)

### Existing route/handler surface to document
- `internal/server/routes.go` — the full route table (~60 routes) this phase must cover
- `internal/server/handlers/*.go` — existing typed `XxxRequest`/`XxxResponse` structs with JSON tags that `swag` will derive schemas from
- `internal/server/middleware/` (or wherever `workspaceMiddleware`/`workspaceRegisteredMiddleware`/`csrfMW` are defined) — the auth/access requirements to document per D-04

### Existing generated-artifact conventions to mirror
- `AGENTS.md` Quick Reference — `sqlc generate` is this repo's existing precedent for a committed, regenerable artifact; mirror its conventions (regenerate command documented, artifact committed) for `swag init`'s output unless research finds a stronger reason to diverge

### Harness policy
- `AGENTS.md` § "Harness Enforcement Default" — this phase follows the full GSD flow by default (already in motion)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- Nearly all ~60 handlers already have typed `Request`/`Response` structs with clean JSON tags (confirmed via `grep -n "type.*Response struct" internal/server/handlers/*.go` — 20+ matches, representative sample) — this is exactly what `swag` needs to auto-derive schemas, minimizing new type-writing work.
- `go.mod` currently has no OpenAPI-adjacent dependency (`swaggo/swag`, `kin-openapi`, etc.) — clean slate, no conflicting prior art to reconcile.
- Echo v4.12.0 (already the framework in use) has mature `swaggo/echo-swagger` middleware support if a UI were ever added later (deferred per D-02, but worth knowing the option exists).

### Established Patterns
- `sqlc generate` is this repo's existing precedent for a build-time code-generation step producing committed output — `swag init` should follow the same shape (documented regenerate command, generated artifact committed to the repo, not gitignored) unless research finds a reason to diverge.

### Integration Points
- The new `GET /api/openapi.json` route needs to be registered in `internal/server/routes.go` alongside the existing ~60 routes it describes — likely near the other top-level routes (`/health`, `/api/status`, `/api/version`).

</code_context>

<specifics>
## Specific Ideas

No specific UI/format mockups requested (D-02 explicitly rules out a UI). The spec itself should be a standard OpenAPI 3.0 JSON document — no custom extensions requested beyond what `swag`/`@Security` annotations naturally produce.

</specifics>

<deferred>
## Deferred Ideas

- Bundled Swagger UI page (`/api/docs`) — explicitly deferred per D-02; JSON spec only for now. Revisit if a concrete need for in-browser exploration arises.
- MCP tool schema documentation — out of scope entirely; MCP tools are already self-describing via the protocol's own mechanism.

</deferred>

---

*Phase: 12-add-openapi-3-0-spec-for-the-rest-api-issue-530-cover-all-60*
*Context gathered: 2026-07-02*
