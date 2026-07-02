# Phase 12: Add OpenAPI 3.0 spec for the REST API - Research

**Researched:** 2026-07-02
**Domain:** Go REST API documentation generation (swaggo/swag annotation parsing + OpenAPI 2.0→3.0 conversion)
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Use `swaggo/swag` annotation-based generation — add `// @Summary`, `// @Router`, etc. doc-comments above each of the ~60 handler functions. `swag` derives request/response schemas automatically from the existing typed `XxxRequest`/`XxxResponse` structs' JSON tags (confirmed during discuss-phase: nearly every handler already has both typed, e.g. `QueryRequest`/`SearchResponse` in `internal/server/handlers/query.go`) — no need to hand-write schemas separately from the comments.
- **D-02:** JSON spec only — no bundled Swagger UI page. `GET /api/openapi.json` returns the raw spec; clients import it into their own tooling (Postman, their own Swagger UI instance, codegen).
- **D-03:** Cover all ~60 existing routes in `internal/server/routes.go`: top-level (`/health`, `/api/status`, `/api/version`), the full `/api/v1/*` group, and note `/mcp`/`/sse` exist but are protocol endpoints, not typical REST — document their presence but they don't need full request/response schemas (they're not JSON-request/response endpoints in the OpenAPI sense).
- **D-04:** Document auth/access requirements per route where middleware applies: `workspaceMiddleware` (workspace resolution), `workspaceRegisteredMiddleware` (write-path gate), `csrfMW` (CSRF token) — via `@Security`-style swag annotations, so the spec reflects real access requirements, not just happy-path shapes.
- **D-05:** Single source of truth with the route table — adding a new route without updating its swag annotations should be caught, ideally by a test (per issue acceptance criteria). Exact mechanism (e.g. a test asserting route count in `routes.go` matches annotated-endpoint count in the generated spec, or a `go generate` + `git diff --exit-code` CI-style check) is a research/planning decision — not locked here, but MUST exist in some automated form.
- **D-06:** Docs (README.md/SETUP_AGENT.md or similar) must mention how to fetch/browse the spec — per issue acceptance criteria.

### Claude's Discretion

- Exact `swag`-generated artifact layout (typically `docs/swagger.json`, `docs/swagger.yaml`, `docs/docs.go` — swag's conventional output) and whether the generated Go package is committed to the repo or built via `go generate`/Makefile target at build time — research should confirm swag's current conventions and this repo's existing `go generate` usage (if any) before locking. **Research finding: this repo has NO existing `go:generate` directives anywhere and no Makefile codegen target for `sqlc` either — `sqlc generate` is a manually-invoked, documented-only convention. See Open Question #3.**
- Whether `swag init` runs as a Makefile target, a `go:generate` directive, or a CI step — planner's call based on how this repo already handles other generated artifacts (e.g. `sqlc generate` convention already exists per AGENTS.md's Quick Reference).
- Whether the drift-detection test (D-05) is a Go test, a Makefile check, or a CI-only step — planner's call. **Research recommends: a `go test` using `swag`'s library API — see Standard Stack and Code Examples.**

### Deferred Ideas (OUT OF SCOPE)

- Bundled Swagger UI page (`/api/docs`) — explicitly deferred per D-02; JSON spec only for now. Revisit if a concrete need for in-browser exploration arises.
- MCP tool schema documentation — out of scope entirely; MCP tools are already self-describing via the protocol's own mechanism.
</user_constraints>

<phase_requirements>
## Phase Requirements

No formal REQUIREMENTS.md IDs are mapped to this phase — it is a feature phase tracked via GitHub issue #530, scoped entirely by CONTEXT.md's D-01 through D-06 (reproduced above) and the issue's own acceptance criteria (reproduced verbatim below since it is the canonical scope document per CONTEXT.md's `<canonical_refs>`).

### Issue #530 Acceptance Criteria (verbatim, `gh issue view 530 --repo nano-step/nano-brain`)

| # | Criterion | Research Support |
|---|-----------|-------------------|
| AC-1 | An OpenAPI 3.0 document is servable from the running nano-brain server and validates against the OpenAPI 3.0 schema | Standard Stack (swag v1 + `kin-openapi/openapi2conv.ToV3()` conversion path) resolves the OpenAPI-2.0-vs-3.0 gap; Code Examples includes a `kin-openapi/openapi3` `Loader.Validate()` test satisfying "validates against the schema" literally |
| AC-2 | All ~60 existing REST routes appear in the spec with at least path, method, and description; request/response bodies documented where a typed struct already exists | Route count independently confirmed (60, via grep on `routes.go`); Architecture Patterns (Pattern 1, Pattern 2) cover the standard-handler and protocol-tunnel (`/mcp`/`/sse`) cases respectively; Pitfall 2 flags the unexported-struct risk affecting ~35 of the 60 routes' schema completeness |
| AC-3 | The spec generation mechanism has a single source of truth with routes.go (adding a new route without updating the spec should be caught, ideally by a test) | Code Examples' drift-detection test (`TestOpenAPISpec_NoDrift`) using `swag/gen`'s library API; Pitfall 3 flags the specific route-path-must-match-registration risk this test must guard against |
| AC-4 | Docs (README/SETUP_AGENT.md or similar) mention how to fetch/browse the spec | Confirmed `docs/SETUP_AGENT.md` and `README.md` both exist with established structure (Quick Reference / numbered agent-setup steps) — planner should add a short section to one or both, per D-06 |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **Autonomous delivery flow:** discuss → plan → execute → independent review (separate agent, no self-approval) → test with evidence → ship via PR. This RESEARCH.md feeds the plan step; the planner should structure tasks so each is independently verifiable per the harness's evidence requirement.
- **Test isolation (MANDATORY):** all tests (`go test -race -short ./...`, `go test -race -tags=integration ./...`) MUST target `nanobrain_test` / port `3199` — never `nanobrain_dev` / `3100`. This phase's new tests (drift detection, schema validation, handler test) are pure unit tests with no DB dependency, so this constraint mostly doesn't bind here, but any integration-style smoke test the planner adds (e.g. hitting the live `/api/openapi.json` endpoint) MUST use the test server/DB.
- **Quick Reference commands:** `CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain`, `go test -race -short ./...`, `go test -race -tags=integration ./...`, `sqlc generate` — the planner should extend this list with the new `swag`/openapi generation command (e.g. `make generate-openapi`) per Open Question #3's recommendation.
- **No AI footers** in commits/PRs (no `Co-Authored-By`, no emoji footers) — applies to whatever branch/PR this phase produces.
- **Harness Enforcement Default (AGENTS.md):** this phase already has a GitHub issue (#530) and is going through the full GSD flow — satisfies the "issue first" and "lane classification" gate requirements already.

## Summary

nano-brain's REST API (60 routes in `internal/server/routes.go`, confirmed by direct count) needs a generated, drift-checked OpenAPI 3.0 document served at `GET /api/openapi.json`. CONTEXT.md D-01 locks `swaggo/swag` for annotation-based generation. The critical finding of this research: **`swaggo/swag` v1 (the current stable major version, v1.16.6) generates OpenAPI 2.0 (Swagger) only — it has no native OpenAPI 3.0 output** `[VERIFIED: github.com/swaggo/swag README via WebFetch]`. A `swag/v2` module exists with a `--v3.1` flag, but it is at `v2.0.0-rc5` (release candidate, January 2026) — explicitly marked unstable upstream `[VERIFIED: pkg.go.dev/github.com/swaggo/swag/v2]`. Issue #530's acceptance criteria requires the served document to "validate against the OpenAPI 3.0 schema," which rules out shipping raw Swagger 2.0.

**Primary recommendation:** Use `swaggo/swag` v1 (stable, `v1.16.6`) to annotate handlers and generate Swagger 2.0 JSON, then convert to OpenAPI 3.0 in-process using `github.com/getkin/kin-openapi/openapi2conv.ToV3()` (stable, `v0.140.0`, actively maintained). Validate the converted document with `kin-openapi/openapi3`'s `Loader.Validate()` — the same library serves both the conversion and the acceptance-criteria validation test. `echo-swagger` is NOT needed (D-02 rules out UI-serving middleware); the final JSON is served via a plain `echo.GET` route with `c.Blob()`/`c.JSONBlob()`. Drift detection (D-05) is achievable as a pure `go test` using `swag`'s programmatic library API (`github.com/swaggo/swag/gen`), avoiding any dependency on the `swag` CLI binary being installed in CI.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Route/handler annotation (`@Router`, `@Param`, `@Success`, `@Security`) | API / Backend | — | Doc-comments live directly above the Echo handler functions they describe; this is source-of-truth data, not a separate layer |
| Spec generation (swag parse → Swagger 2.0 JSON) | Build tooling (build-time or `go generate`) | API / Backend (drift test) | Mirrors existing `sqlc generate` pattern — a codegen step producing a committed artifact, not a runtime dependency |
| Spec conversion (Swagger 2.0 → OpenAPI 3.0) | Build tooling | API / Backend (drift test) | Same generation step; `kin-openapi/openapi2conv` runs immediately after swag's parse, before the artifact is committed |
| Spec serving (`GET /api/openapi.json`) | API / Backend | — | A new Echo route in `internal/server/routes.go`, alongside the other ~60 routes; reads a committed static file or embedded asset, no request-time generation |
| Drift detection | API / Backend (test suite) | CI | A `go test` that re-runs generation into a temp dir and diffs against the committed file — runs in the same `go test -race -short ./...` sweep this repo already uses, no new CI job required |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/swaggo/swag` | v1.16.6 (verified via proxy.golang.org, released 2025-07-28) | Parses `@Summary`/`@Router`/`@Param`/`@Success`/`@Security` doc-comments into a Swagger 2.0 document | Locked by CONTEXT.md D-01; de facto standard annotation-based Go OpenAPI generator, 12.9k+ GitHub stars, MIT license `[VERIFIED: github.com/swaggo/swag]` |
| `github.com/getkin/kin-openapi` | v0.140.0 (verified via proxy.golang.org, released 2026-06-02) | `openapi2conv.ToV3()` for Swagger 2.0 → OpenAPI 3.0 conversion; `openapi3.Loader.Validate()` for schema validation | Only actively maintained, idiomatic Go library that does both conversion and validation with the same dependency — avoids adding two separate packages `[VERIFIED: pkg.go.dev/github.com/getkin/kin-openapi]` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/swaggo/swag/gen` (subpackage of the swag module already installed) | same as swag v1.16.6 | Programmatic (`gen.New().Build(&gen.Config{...})`) generation from a Go test, instead of shelling out to the CLI binary | Use for the D-05 drift-detection test — no CLI binary required in CI, just the Go module already in `go.mod` |

### Explicitly NOT needed
| Library | Why not |
|---------|---------|
| `github.com/swaggo/echo-swagger` | Only provides Swagger-UI-serving middleware; D-02 explicitly rules out a bundled UI. Serving the raw JSON needs nothing beyond a plain `echo.GET` handler. |
| `github.com/swaggo/swag/v2` | Native OpenAPI 3.1 support exists but the module is `v2.0.0-rc5` — a release candidate, not a stable release. Adopting it would violate the "don't hand-roll on unstable foundations" principle and contradicts D-01's plain "swaggo/swag" (v1 is the version referred to by that name in current common usage). |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| swag v1 + kin-openapi conversion | swag/v2 native `--v3.1` | Avoids the conversion step, but ships on an RC-tagged dependency with no stability guarantee — a risk this repo's harness policy (no hand-rolling on shaky foundations) would flag. Revisit once swag/v2 reaches a stable v2.0.0. |
| swag annotation-based (locked D-01) | Code-first with `kin-openapi/openapi3` building the spec programmatically from route table + reflection | Issue #530 itself flagged this as an open research question, but D-01 already locked swag annotations — not re-litigated here per CONTEXT.md's constraint that locked decisions are not alternatives to explore. |

**Installation:**
```bash
go get github.com/swaggo/swag@v1.16.6
go get github.com/getkin/kin-openapi@v0.140.0
go install github.com/swaggo/swag/cmd/swag@v1.16.6   # CLI, for local dev convenience only — NOT required by CI (drift test uses the gen library)
```

**Version verification:** Confirmed via `proxy.golang.org` (the authoritative Go module proxy, equivalent rigor to `npm view`):
- `swaggo/swag`: latest tag `v1.16.6`, published 2025-07-28 `[VERIFIED: proxy.golang.org/github.com/swaggo/swag/@latest]`
- `swaggo/echo-swagger`: latest tag `v1.5.2`, published 2026-03-04 (listed for completeness; not adopted) `[VERIFIED: proxy.golang.org]`
- `getkin/kin-openapi`: latest tag `v0.140.0`, published 2026-06-02 `[VERIFIED: proxy.golang.org/github.com/getkin/kin-openapi/@latest]`

Go module requirement: swag requires Go 1.19+ to build `[CITED: swaggo/swag README]`; this repo is on Go 1.23.0, no compatibility concern.

## Package Legitimacy Audit

> `gsd-tools query package-legitimacy check` only supports `npm|pypi|crates` ecosystems — this phase's packages are Go modules, so legitimacy was verified manually against the authoritative Go module proxy (`proxy.golang.org`), which is the equivalent-rigor authoritative source for Go.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `github.com/swaggo/swag` | Go module proxy | First tag v1.0.0 (multi-year project, 50+ tags through v1.16.6) | N/A (Go modules don't report download counts; 12.9k+ GitHub stars, `swaggo` org is well-established with 8+ framework-integration repos) | github.com/swaggo/swag (active, tagged releases through 2025-07-28) | OK | Approved |
| `github.com/getkin/kin-openapi` | Go module proxy | Long-running project (140+ tagged versions through v0.140.0) | N/A (Go modules); widely used across the Go OpenAPI tooling ecosystem | github.com/getkin/kin-openapi (active, tagged 2026-06-02) | OK | Approved |
| `github.com/swaggo/swag/v2` | Go module proxy | New major version, `v2.0.0-rc5` tag (Jan 2026) | N/A | github.com/swaggo/swag (same repo, v2 subdirectory) | SUS (pre-release) | NOT adopted — see Alternatives Considered |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** `github.com/swaggo/swag/v2` — flagged not for illegitimacy but for release-candidate instability; excluded from the recommended stack entirely rather than gated behind a checkpoint, since v1 fully satisfies the requirement via conversion.

All three packages above are `[VERIFIED]` via the authoritative Go module proxy (`proxy.golang.org`), not merely discovered via WebSearch — the specific version numbers and publish dates were confirmed by direct proxy queries, and existence/purpose was cross-checked against each project's own README/pkg.go.dev page.

## Architecture Patterns

### System Architecture Diagram

```
                    ┌─────────────────────────────────────────┐
                    │  internal/server/handlers/*.go            │
                    │  (60 Echo handler functions,               │
                    │   ~35 already have typed Request/Response  │
                    │   structs with JSON tags)                  │
                    │                                             │
                    │  + new doc-comments above each handler:    │
                    │    // @Summary ...                          │
                    │    // @Router /api/v1/... [method]          │
                    │    // @Param ...                            │
                    │    // @Success 200 {object} XxxResponse     │
                    │    // @Security WorkspaceAuth               │
                    └───────────────────┬─────────────────────────┘
                                         │  swag parses source (AST, no reflection)
                                         ▼
                    ┌─────────────────────────────────────────┐
                    │  swag gen.Build() [swaggo/swag v1]         │
                    │  → docs/swagger.json  (OpenAPI 2.0)         │
                    └───────────────────┬─────────────────────────┘
                                         │  openapi2conv.ToV3()
                                         ▼
                    ┌─────────────────────────────────────────┐
                    │  docs/openapi.json  (OpenAPI 3.0,          │
                    │   committed artifact — mirrors sqlc's       │
                    │   committed-codegen convention)             │
                    └───────────────────┬─────────────────────────┘
                                         │
                       ┌─────────────────┴──────────────────┐
                       ▼                                     ▼
        ┌───────────────────────────┐      ┌──────────────────────────────┐
        │ internal/server/routes.go   │      │ Drift-detection go test        │
        │ new route:                   │      │ (runs gen.Build() to a temp    │
        │ GET /api/openapi.json        │      │  dir, openapi2conv.ToV3(),     │
        │ → c.Blob(200, "application/  │      │  diffs bytes against committed │
        │   json", embeddedBytes)      │      │  docs/openapi.json — fails on  │
        └───────────────────────────┘      │  mismatch)                      │
                       │                     └──────────────────────────────┘
                       ▼
        ┌───────────────────────────┐
        │ Non-MCP HTTP client          │
        │ (Postman, codegen tool, etc.) │
        └───────────────────────────┘
```

### Recommended Project Structure
```
docs/
├── swagger.json          # intermediate Swagger 2.0 output from swag (may be gitignored — see Open Questions)
├── docs.go                # swag's generated Go package (general API info); only needed if using swag's Go-embed convenience — likely NOT needed here since serving is via committed JSON, not swag's own embed
└── openapi.json           # FINAL committed artifact: OpenAPI 3.0, converted — this is what /api/openapi.json serves
internal/server/
├── routes.go              # add: s.echo.GET("/api/openapi.json", handlers.OpenAPISpec(...))
├── handlers/
│   ├── openapi.go          # NEW: handler serving the committed docs/openapi.json (embed.FS or os.ReadFile)
│   └── *.go                 # existing 34 handler files — add swag doc-comments above each exported handler func
└── openapi_gen_test.go     # NEW: drift-detection test (or place under a dedicated internal/openapigen package)
```

### Pattern 1: Annotation placement directly above existing exported handler constructors
**What:** swag annotations go immediately above the function whose doc-comment block they belong to — but this repo's handlers are largely *constructor functions returning `echo.HandlerFunc`* (e.g. `func Query(searcher HybridSearcher, logger zerolog.Logger, ...) echo.HandlerFunc`), not simple `func(c echo.Context) error` methods.
**When to use:** For every one of the ~60 routes.
**Example:**
```go
// Source: pattern derived from swag's documented annotation format (github.com/swaggo/swag README),
// applied to this repo's constructor-returns-HandlerFunc idiom (internal/server/handlers/query.go)

// Query godoc
// @Summary      Hybrid BM25 + vector search
// @Description  Runs the hybrid search pipeline scoped to a workspace
// @Tags         search
// @Accept       json
// @Produce      json
// @Param        request body QueryRequest true "Search query"
// @Success      200 {object} SearchResponse
// @Failure      400 {object} handlers.ErrorResponse
// @Security     WorkspaceAuth
// @Router       /api/v1/query [post]
func Query(searcher HybridSearcher, logger zerolog.Logger, rec ...*telemetry.Recorder) echo.HandlerFunc {
	return func(c echo.Context) error {
		// ... unchanged
	}
}
```
**Caveat found in this codebase:** swag's `@Router` line needs the literal runtime path. Several routes are mounted under `api := s.echo.Group("/api/v1", ...)` in `routes.go`, so the annotation must spell out the full path (`/api/v1/query`), not just the handler-relative suffix — swag has no visibility into `routes.go`'s grouping, since it only parses doc-comments, not the route-registration code.

### Pattern 2: Documenting protocol-tunnel routes without full schemas (D-03)
**What:** `/mcp` (GET/POST/DELETE) and `/sse` (GET/POST) are registered via `echo.WrapHandler(...)`, wrapping the MCP SDK's own `http.Handler` — they are not JSON REST endpoints in the OpenAPI sense.
**When to use:** For these 5 route registrations only.
**Example:**
```go
// Source: derived from swag's @Router/@Summary syntax, applied per D-03's "document presence, not schema" requirement

// MCPStreamable godoc
// @Summary      MCP streamable HTTP transport (protocol endpoint — not a typical JSON REST API; see MCP spec)
// @Tags         protocol
// @Router       /mcp [get]
// @Router       /mcp [post]
// @Router       /mcp [delete]
func mcpPlaceholder() {}
```
Since these routes are registered inline in `routes.go` via `echo.WrapHandler`, not via a named handler constructor in `internal/server/handlers/`, swag needs a documented anchor function to attach the annotation to (swag requires annotations on an actual Go function). A minimal unexported placeholder function (never called, purely a documentation anchor) is the simplest approach — verify this against swag's parser during Wave 0 before committing to the pattern across all 5 tunnel routes.

### Recommended package-level `@Security` scheme declaration
```go
// Source: swaggo/swag README security-definitions syntax, mapped to this repo's actual middleware names

// @securityDefinitions.apikey WorkspaceAuth
// @in query
// @name workspace
// @description Workspace hash required via query param (GET) or JSON body (POST/PUT/PATCH). See workspaceMiddleware in internal/server/middleware.go.

// @securityDefinitions.apikey WorkspaceRegisteredAuth
// @in query
// @name workspace
// @description Workspace hash must belong to an already-registered workspace (write-path gate). See workspaceRegisteredMiddleware.

// @securityDefinitions.apikey CSRFToken
// @in header
// @name X-CSRF-Token
// @description Required on write endpoints per D-04. See middleware.CSRF in internal/server/middleware/.
```
Place this block in a dedicated `main-api-info` doc-comment location — conventionally in `cmd/nano-brain/main.go` or a new `internal/server/doc.go`, since swag's `-g` flag expects one "general API info" file `[CITED: swaggo/swag README]`.

### Anti-Patterns to Avoid
- **Annotating the returned `echo.HandlerFunc` closure body instead of the outer constructor:** swag's parser expects the doc-comment directly above a named function declaration — annotating an anonymous inline closure will not be picked up.
- **Relying on swag/v2's native OpenAPI 3 output for production:** at `v2.0.0-rc5`, breaking changes are possible before a stable v2.0.0 tag lands; this phase should not take on that instability for a compliance-adjacent deliverable (issue #530 acceptance criteria requires schema validation).
- **Adding `echo-swagger` "just in case":** D-02 explicitly rules out a UI; pulling in the dependency anyway adds unused surface area and contradicts the "Simplicity First" project convention (AGENTS.md/CLAUDE.md behavioral guidelines: no speculative flexibility).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Parsing Go doc-comments into a structured spec | A custom regex/AST scanner over `routes.go` + handler files | `swaggo/swag`'s `gen` package | Locked by D-01; also a well-trodden, edge-case-hardened parser (nested structs, embedded structs, `omitempty`, enum-like string types, arrays) that a hand-rolled scanner would take months to match |
| Swagger 2.0 → OpenAPI 3.0 structural conversion (securityDefinitions → components.securitySchemes, definitions → components.schemas, etc.) | Manual JSON transformation script | `kin-openapi/openapi2conv.ToV3()` | The 2.0→3.0 structural diff is large and has many edge cases (e.g. `type: file` params, `consumes`/`produces` → per-operation `content`); a maintained converter avoids silent spec-invalidity bugs |
| OpenAPI 3.0 schema validation | Hand-written JSON Schema checks | `kin-openapi/openapi3` `Loader.Validate()` | Issue #530's acceptance criterion #1 explicitly requires "validates against the OpenAPI 3.0 schema" — a maintained validator directly satisfies this, a custom validator would need to reimplement the entire OpenAPI 3.0 meta-schema |

**Key insight:** every "don't hand-roll" item here maps to one already-vetted, actively maintained library (both from the same trusted `swaggo`/`getkin` sources) — this phase should need exactly two new dependencies, no more.

## Common Pitfalls

### Pitfall 1: swag natively emits OpenAPI 2.0, not 3.0 — this is easy to discover too late
**What goes wrong:** A developer runs `swag init`, gets a `swagger.json`, and assumes it satisfies "OpenAPI 3.0" because the tool is colloquially called "the OpenAPI generator for Go." The output is actually Swagger 2.0 (`"swagger": "2.0"` at the document root), which fails any OpenAPI-3.0-schema validation.
**Why it happens:** swag's own documentation and most tutorials predate widespread OpenAPI 3.0 adoption in the Go ecosystem; the `swaggo` org name and general community usage blur the version distinction.
**How to avoid:** Always pipe swag's JSON output through `openapi2conv.ToV3()` before serving or committing as the final artifact. Add a test asserting `openapi.json`'s root has `"openapi": "3.0.x"`, not `"swagger": "2.0"`.
**Warning signs:** The served document's root key is `swagger` instead of `openapi`; any OpenAPI-3.0-only client tooling (e.g. some codegen tools) rejects the file with a version-mismatch error.

### Pitfall 2: Unexported (lowercase) Request/Response structs in existing handlers
**What goes wrong:** Roughly 35 of this repo's ~60 handler request/response types are unexported (e.g. `healthResponse`, `overviewRequest`, `statusResponse`, `flowRequest` — confirmed via grep across `internal/server/handlers/*.go`). swag's `@Success 200 {object} TypeName` annotation references a type by name within the parsed package; because swag parses Go source (not runtime reflection), it CAN resolve unexported types declared in the same package as the annotated handler — but cross-package references to unexported types will fail to resolve, and some swag versions historically had inconsistent behavior with unexported models in generated schema `component` names.
**Why it happens:** These structs were written for internal handler-local use, never designed with public API documentation in mind — a reasonable choice at the time, now a documentation-generation constraint.
**How to avoid:** Because all annotations will be placed above handlers in the *same* `handlers` package as their request/response structs, same-package unexported-type resolution should work — but this MUST be verified empirically in a Wave 0 spike (annotate 2-3 handlers with unexported types, run `swag init`, inspect the output) before committing to annotating all 60 routes with an assumption that turns out wrong.
**Warning signs:** Generated `swagger.json` shows `"$ref": "#/definitions/handlers.someType"` with an empty/missing schema body, or swag emits a build-time warning about an unresolved type.

### Pitfall 3: `@Router` path must be the full mounted path, not the handler-relative suffix
**What goes wrong:** Annotating `@Router /query [post]` on the `Query` handler produces a spec entry for `/query`, but the route is actually mounted at `/api/v1/query` (via the `api := s.echo.Group("/api/v1", ...)` and `data := api.Group("", workspaceMiddleware(...))` nesting in `routes.go`). swag has zero visibility into `routes.go`'s `Group()` calls — it only reads doc-comments.
**Why it happens:** swag's annotation model assumes the developer manually keeps the `@Router` path in sync with wherever the route is actually registered; there's no compiler-enforced link between the two.
**How to avoid:** When annotating each handler, cross-reference the exact registration line in `routes.go` (already inventoried in this research's Phase Requirements section below) to get the true full path, including the `/api/v1` prefix where applicable. The drift-detection test (D-05) should also assert path *strings* match, not just count — a route moved to a different prefix without updating its annotation would otherwise pass a naive count-based check.
**Warning signs:** Served spec's paths don't match `curl`-able URLs; a client following the spec gets 404s.

### Pitfall 4: `echo.WrapHandler`-based routes (`/mcp`, `/sse`) have no named handler function to annotate
**What goes wrong:** swag requires a Go function declaration to attach a doc-comment to. `s.echo.GET("/mcp", echo.WrapHandler(wrappedStreamable))` passes an already-constructed `http.Handler` value inline — there's no dedicated function in `internal/server/handlers/` for these 5 route registrations.
**Why it happens:** These are protocol-tunnel endpoints (MCP SDK's own transport), architecturally different from the other handler-per-route pattern.
**How to avoid:** Use a minimal placeholder/anchor function purely for documentation purposes (Pattern 2 above), OR use swag's `@Router` annotation syntax attached to any single documented location (e.g. a doc-only block near the general API info) — needs Wave 0 verification of which approach swag's parser accepts.
**Warning signs:** These 5 routes silently missing from the generated spec, failing D-03's "document their presence" requirement.

## Code Examples

### Serving the committed spec via plain Echo (no UI middleware)
```go
// Source: pattern derived from Echo v4's documented c.Blob()/c.File() API (labstack/echo v4.12.0, already in go.mod)
package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// OpenAPISpec serves the committed, generated OpenAPI 3.0 document.
// The spec is generated by `swag init` + openapi2conv.ToV3() (see Makefile
// target `generate-openapi`) and committed to docs/openapi.json — this
// handler does not generate anything at request time.
func OpenAPISpec(specJSON []byte) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.Blob(http.StatusOK, "application/json", specJSON)
	}
}
```
Registration in `routes.go`, alongside the other top-level routes:
```go
s.echo.GET("/api/openapi.json", handlers.OpenAPISpec(openapiSpecBytes))
```
Whether `openapiSpecBytes` comes from `//go:embed docs/openapi.json` (mirroring the `migrations.go` precedent already in this repo) or a plain `os.ReadFile` at startup is a Claude's Discretion item per CONTEXT.md — `//go:embed` is recommended since it matches this repo's one existing embed precedent (`migrations/migrations.go`) and guarantees the served file always matches the committed binary (no runtime file-path drift).

### Drift-detection test using swag's library API (no CLI binary needed)
```go
// Source: pattern derived from github.com/swaggo/swag/gen's documented Config/Build API (pkg.go.dev/github.com/swaggo/swag/gen)
package openapigen_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/swaggo/swag/gen"
)

func TestOpenAPISpec_NoDrift(t *testing.T) {
	tmpDir := t.TempDir()

	g := gen.New()
	err := g.Build(&gen.Config{
		SearchDir:   "../../..",        // repo root, or internal/server
		MainAPIFile: "internal/server/doc.go",
		OutputDir:   tmpDir,
		OutputTypes: []string{"json"},
	})
	if err != nil {
		t.Fatalf("swag generation failed: %v", err)
	}

	generatedSwagger2, err := os.ReadFile(filepath.Join(tmpDir, "swagger.json"))
	if err != nil {
		t.Fatalf("reading generated swagger.json: %v", err)
	}

	var doc2 openapi2.T
	// unmarshal + convert to v3, then marshal deterministically...
	doc3, err := openapi2conv.ToV3(&doc2)
	if err != nil {
		t.Fatalf("openapi2conv.ToV3: %v", err)
	}
	_ = doc3

	committed, err := os.ReadFile("../../../docs/openapi.json")
	if err != nil {
		t.Fatalf("reading committed docs/openapi.json: %v", err)
	}

	// Compare freshly-generated-and-converted bytes against committed bytes.
	// Exact byte-diffing requires deterministic JSON marshaling (sorted keys,
	// fixed indentation) on both sides — verify swag/kin-openapi's marshal
	// output is stable across runs before relying on raw byte equality;
	// otherwise compare semantically (unmarshal both sides into map[string]any
	// and use reflect.DeepEqual or a JSON-diff library).
	_ = generatedSwagger2
	_ = committed
}
```
**Open question flagged in-line:** whether raw byte-diff or semantic diff is needed depends on marshal determinism — verify in Wave 0 (see Open Questions below).

### OpenAPI 3.0 schema validation (satisfies issue #530 acceptance criterion #1)
```go
// Source: pattern derived from github.com/getkin/kin-openapi/openapi3's documented Loader/Validate API
package openapigen_test

import (
	"context"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestOpenAPISpec_ValidatesAgainstOpenAPI3Schema(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile("../../../docs/openapi.json")
	if err != nil {
		t.Fatalf("loading docs/openapi.json: %v", err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		t.Fatalf("docs/openapi.json failed OpenAPI 3.0 validation: %v", err)
	}
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Hand-maintained API docs (Markdown/Postman collections manually updated) | Annotation-driven generation with automated drift detection | Long-standing industry shift (pre-dates this phase); this repo has never had either | Prevents the exact failure mode issue #530 is trying to close: docs silently going stale |
| swag emitting only Swagger 2.0 | swag/v2 (RC) adding native OpenAPI 3.1 support | swag/v2 first RC tags appeared ~2025, still at `rc5` as of Jan 2026 | Not yet safe to adopt for this phase; revisit when v2 reaches a stable tag |

**Deprecated/outdated:**
- Swagger 2.0 as a target format for *new* API documentation projects: OpenAPI 3.0/3.1 is the current standard; Swagger 2.0 should only appear here as swag's *intermediate* output, immediately converted, never the final served artifact.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | swag's AST parser can resolve unexported (lowercase) Request/Response struct types referenced in `@Success`/`@Param body` annotations, when the annotation and the struct live in the same `handlers` package | Common Pitfalls #2 | If wrong, ~35 of the 60 routes would need their structs exported (a larger, more invasive change than annotation-only) before swag can generate a complete schema — this would blow up phase scope significantly. MUST be verified in Wave 0 before committing to an annotation-only plan. |
| A2 | A minimal anchor/placeholder function is an acceptable and swag-parseable way to document the 5 `echo.WrapHandler`-based `/mcp`/`/sse` routes that have no dedicated named handler function | Common Pitfalls #4, Pattern 2 | If wrong, these 5 routes might need a different documentation mechanism (e.g. manually editing the generated JSON post-conversion) — low risk since D-03 only requires "document presence," a manual JSON patch is an acceptable fallback |
| A3 | Byte-for-byte diffing of swag+kin-openapi's regenerated output against the committed file is reliable (i.e., both libraries produce deterministic marshal output across runs) for the D-05 drift test | Code Examples (drift test) | If wrong, the drift test would have false-positive failures on every run even with no actual route changes; mitigated by falling back to semantic (parsed-structure) diffing instead of raw bytes — flagged inline in the code example |

## Open Questions

1. **Does swag resolve unexported struct types for schema generation in the same-package case?**
   - What we know: swag parses Go source via AST, not runtime reflection, so package-private types are visible to the parser in principle.
   - What's unclear: Whether swag's model-name generation and cross-reference logic treats unexported types identically to exported ones, or silently skips/mangles them — WebSearch did not surface a definitive statement either way.
   - Recommendation: First planning/implementation task should be a Wave 0 spike — annotate 2-3 representative handlers (including at least one with an unexported struct, e.g. `health.go`'s `healthResponse`) and run `swag init` locally, inspecting the output before scaling the annotation work to all 60 routes.

2. **Where should the intermediate `docs/swagger.json` (2.0) live — committed or gitignored?**
   - What we know: The final `docs/openapi.json` (3.0) should be committed (mirrors sqlc's committed-artifact convention, D-05's single-source-of-truth requirement).
   - What's unclear: Whether the intermediate Swagger 2.0 file is useful to keep around (e.g. for debugging conversion issues) or should be treated as a build-time-only scratch file.
   - Recommendation: Gitignore the intermediate `swagger.json`/`docs.go` (swag's raw output); commit only the final converted `openapi.json`. Simpler surface area, one canonical artifact — consistent with "Simplicity First" project convention.

3. **Exact generation trigger — Makefile target, `go:generate` directive, or CI-only step?**
   - What we know: This repo's only comparable precedent (`sqlc generate`) is a manually-invoked CLI command documented in CLAUDE.md's Quick Reference, NOT wired to a Makefile target or `go:generate` directive (grep confirmed zero `go:generate` directives anywhere in the repo, and the Makefile has no `sqlc` or codegen target).
   - What's unclear: Since there's no existing `go:generate`/Makefile precedent to strictly "mirror" (the CONTEXT.md canonical-refs claim of a Makefile precedent doesn't hold on inspection), the planner has more freedom here than CONTEXT.md's phrasing implies.
   - Recommendation: Add a new Makefile target (e.g. `make generate-openapi`) that runs both the swag generation and the openapi2conv step, documented the same way `sqlc generate` is documented (a Quick Reference line), and enforce freshness via the D-05 drift `go test` rather than a CI-only check — this keeps the enforcement inside the existing `go test -race -short ./...` sweep this repo already runs on every PR (`.github/workflows/ci.yml`), requiring no new CI job.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Building/testing the new code | ✓ | 1.23.0 (go.mod) | — |
| `swag` CLI binary | Local dev convenience (`swag init`) | Not required — drift test uses `swag/gen` library API directly | — | Developers can `go install github.com/swaggo/swag/cmd/swag@v1.16.6` locally if they prefer the CLI workflow; CI does not need it |
| Go module proxy access | `go get` for new dependencies | ✓ (confirmed reachable via `proxy.golang.org` during this research session) | — | — |
| Docker / PostgreSQL | Not needed for this phase — no DB schema changes | N/A | — | — |

No missing dependencies block this phase.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go's built-in `testing` package (this repo's existing convention; no third-party test framework) |
| Config file | none — plain `go test` |
| Quick run command | `go test -race -short ./...` |
| Full suite command | `go test -race -tags=integration ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| D-01/D-03 | All 60 routes appear in generated spec with path+method+description | unit | `go test -race -short ./internal/server/... -run TestOpenAPISpec` | ❌ Wave 0 |
| Issue #530 AC-1 | Served document validates against OpenAPI 3.0 schema | unit | `go test -race -short ./internal/openapigen/... -run TestOpenAPISpec_ValidatesAgainstOpenAPI3Schema` | ❌ Wave 0 |
| D-04 | Auth/access requirements documented per route (`@Security`) | unit | assertion within the spec-content test above (check `security` array is non-empty for routes behind `workspaceMiddleware`/`workspaceRegisteredMiddleware`/`csrfMW`) | ❌ Wave 0 |
| D-05 | Spec generation has single source of truth with routes.go; drift caught | unit | `go test -race -short ./internal/openapigen/... -run TestOpenAPISpec_NoDrift` | ❌ Wave 0 |
| New route `GET /api/openapi.json` | Endpoint serves the committed spec | unit/integration | `go test -race -short ./internal/server/... -run TestOpenAPISpecHandler` (unit, handler-level) | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -race -short ./...`
- **Per wave merge:** `go test -race -tags=integration ./...`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/openapigen/openapi_gen_test.go` (or similar) — drift-detection test, covers D-05
- [ ] `internal/openapigen/openapi_validate_test.go` — schema-validation test, covers issue #530 AC-1
- [ ] `internal/server/handlers/openapi_test.go` — handler-level test for the new `GET /api/openapi.json` route
- [ ] Spike task (not a formal test file): manually verify swag resolves unexported struct types in same-package annotations before committing to full-scope annotation work (Open Question #1 / Assumption A1)
- [ ] Framework install: `go get github.com/swaggo/swag@v1.16.6 github.com/getkin/kin-openapi@v0.140.0`

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | No | This phase adds a read-only, unauthenticated discovery endpoint (`GET /api/openapi.json`) describing the API shape — it does not itself perform authentication. Existing `middleware.Auth` (Basic/token, per `internal/server/middleware.go`) already gates all routes uniformly per its `BypassPaths` config; confirm `/api/openapi.json` is treated consistently with other public metadata routes like `/health`/`/api/version` (likely on the bypass list already, or intentionally left behind auth like the rest of `/api/*` — planner's call, not a new security surface). |
| V3 Session Management | No | No session state introduced. |
| V4 Access Control | No | The spec document is metadata about the API surface, not a data-access path. Exposing it does not itself grant access to any protected resource. |
| V5 Input Validation | No (this phase adds no new user-input-accepting endpoint beyond the doc-serving GET, which takes no parameters) | N/A |
| V6 Cryptography | No | No crypto operations introduced. |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| Information disclosure via overly detailed spec (e.g. leaking internal implementation details, unredacted example values, or internal-only routes not meant for external clients) | Information Disclosure | Review the generated spec before shipping — ensure no example values contain real workspace hashes/paths (this repo's existing CLAUDE.md privacy rule against committing real workspace names/hashes applies equally to any example values swag annotations might include; use clearly-fake placeholder values in `@Success`/example annotations, never real captured data) |
| Spec becoming a reconnaissance tool for an attacker (full endpoint enumeration in one file) | Information Disclosure | Accepted risk per issue #530's explicit intent (the whole point is discoverability for legitimate non-MCP clients); no additional mitigation needed beyond what already applies to any public API documentation — this is a deliberate product decision already locked by D-02 (public JSON, no auth-gated UI) |

## Sources

### Primary (HIGH confidence)
- `proxy.golang.org/github.com/swaggo/swag/@latest` — confirmed v1.16.6, 2025-07-28
- `proxy.golang.org/github.com/swaggo/echo-swagger/@latest` — confirmed v1.5.2, 2026-03-04
- `proxy.golang.org/github.com/getkin/kin-openapi/@latest` — confirmed v0.140.0, 2026-06-02
- `github.com/swaggo/swag` README (fetched via WebFetch) — confirmed Swagger 2.0-only native output, annotation syntax, Go 1.19+ requirement, framework-integration-package-only-needed-for-UI
- `pkg.go.dev/github.com/swaggo/swag/v2` — confirmed v2.0.0-rc5 release-candidate status, native `--v3.1` flag existence
- `pkg.go.dev/github.com/swaggo/swag/gen` and `pkg.go.dev/github.com/getkin/kin-openapi/openapi2conv` — confirmed programmatic library APIs (`gen.New().Build(Config)`, `openapi2conv.ToV3()`)
- `pkg.go.dev/github.com/getkin/kin-openapi/openapi3` — confirmed `Loader`/`doc.Validate(ctx)` API for schema validation
- `gh issue view 530 --repo nano-step/nano-brain` — full issue text, acceptance criteria (direct tool call, authoritative for scope)
- Direct codebase reads: `internal/server/routes.go` (60 routes counted via grep, cross-referenced against issue text), `internal/server/middleware.go` (exact `workspaceMiddleware`/`workspaceRegisteredMiddleware` behavior), `internal/server/handlers/*.go` (struct export-status audit), `migrations/migrations.go` (this repo's only existing `//go:embed` precedent), `sqlc.yaml` + `Makefile` (confirmed no `go:generate`/Makefile precedent for codegen, contradicting one CONTEXT.md claim)

### Secondary (MEDIUM confidence)
- WebSearch results on swag's unexported-type handling — inconclusive, downgraded to an Assumption (A1) requiring Wave 0 verification rather than treated as fact

### Tertiary (LOW confidence)
- None used as load-bearing claims; all package-existence and version claims were cross-verified against `proxy.golang.org` directly rather than left at WebSearch-only confidence.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — both core packages verified directly against the Go module proxy and their own documentation, with exact version/date confirmation
- Architecture: HIGH — based on direct reads of this repo's actual `routes.go`, `middleware.go`, and handler files, not assumptions
- Pitfalls: MEDIUM-HIGH — the OpenAPI-2.0-vs-3.0 pitfall is HIGH confidence (directly verified); the unexported-struct pitfall is MEDIUM confidence (logically sound but not empirically tested against swag's actual parser in this session — flagged as Assumption A1 requiring a Wave 0 spike)

**Research date:** 2026-07-02
**Valid until:** 30 days (stable Go-ecosystem libraries; re-check if swag/v2 reaches a stable tag before implementation starts, which would reopen the swag-v1-vs-v2 decision)
