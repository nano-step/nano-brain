---
phase: 12
slug: add-openapi-3-0-spec-for-the-rest-api-issue-530-cover-all-60
status: draft
nyquist_compliant: true
wave_0_complete: false
created: 2026-07-02
---

# Phase 12 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go's built-in `testing` package (this repo's existing convention; no third-party test framework) |
| **Config file** | none — plain `go test` |
| **Quick run command** | `go test -race -short ./...` |
| **Full suite command** | `go test -race -tags=integration ./...` |
| **Estimated runtime** | ~10-20 seconds for the new openapigen tests (no live server/DB needed); unchanged for the rest of the existing suite |

---

## Sampling Rate

- **After every task commit:** `go test -race -short ./...`
- **After every plan wave:** `go test -race -tags=integration ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~20 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 12-01-01 | 01 | 1 | Wave 0 spike (Assumption A1) — swag resolves unexported same-package struct types | — | N/A | manual spike, not a formal test | `swag init` run locally against 2-3 annotated handlers, inspect output | ❌ W0 | ⬜ pending |
| 12-01-02 | 01 | 1 | D-01/D-03 — all 60 routes appear in generated spec with path+method+description | — | N/A | unit | `go test -race -short ./internal/server/... -run TestOpenAPISpec` | ❌ W0 | ⬜ pending |
| 12-01-03 | 01 | 1 | Issue #530 AC-1 — served document validates against OpenAPI 3.0 schema | — | N/A | unit | `go test -race -short ./internal/openapigen/... -run TestOpenAPISpec_ValidatesAgainstOpenAPI3Schema` | ❌ W0 | ⬜ pending |
| 12-01-04 | 01 | 1 | Pitfall 1 — served document root is `"openapi": "3.0.x"`, never `"swagger": "2.0"` | — | N/A | unit | assertion within the schema-validation test above | ❌ W0 | ⬜ pending |
| 12-02-01 | 02 | 2 | D-04 — auth/access requirements documented per route (`@Security`) | V4 Access Control (documented, not new) | `security` array non-empty for routes behind workspaceMiddleware/workspaceRegisteredMiddleware/csrfMW | unit | assertion within the spec-content test (check `security` per matching route) | ❌ W0 | ⬜ pending |
| 12-02-02 | 02 | 2 | D-05 — spec generation has single source of truth with routes.go; drift caught | Pitfall 3 (path-string drift) | `@Router` path strings match actual registered paths, not just a count | unit | `go test -race -short ./internal/openapigen/... -run TestOpenAPISpec_NoDrift` | ❌ W0 | ⬜ pending |
| 12-02-03 | 02 | 2 | New route `GET /api/openapi.json` serves the committed spec | Information Disclosure (accepted per D-02) | Served bytes match the committed `docs/openapi.json` exactly | unit | `go test -race -short ./internal/server/... -run TestOpenAPISpecHandler` | ❌ W0 | ⬜ pending |
| 12-03-01 | 03 | 3 | D-06 — docs mention how to fetch/browse the spec | — | N/A | manual (docs review) | `grep -qi 'openapi' README.md docs/SETUP_AGENT.md` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Spike (not a formal test): manually verify swag resolves unexported same-package struct types (Assumption A1) — annotate 2-3 handlers including one with an unexported struct (e.g. `health.go`'s `healthResponse`), run `swag init` locally, inspect output. Must pass BEFORE scaling annotation work to all 60 routes — if it fails, phase scope grows significantly (structs would need exporting).
- [ ] `go get github.com/swaggo/swag@v1.16.6 github.com/getkin/kin-openapi@v0.140.0`
- [ ] `internal/openapigen/openapi_gen_test.go` (or similar) — drift-detection test (D-05)
- [ ] `internal/openapigen/openapi_validate_test.go` — schema-validation test (issue #530 AC-1)
- [ ] `internal/server/handlers/openapi_test.go` — handler-level test for `GET /api/openapi.json`
- [ ] `internal/server/doc.go` (or similar) — swag's "general API info" file + `@securityDefinitions` block
- [ ] Makefile target `generate-openapi` (or equivalent) running swag gen + openapi2conv, documented like `sqlc generate` in CLAUDE.md's Quick Reference

*Also required: verify `/api/openapi.json`'s auth/bypass treatment is consistent with `/health`/`/api/version` (Security Domain V2 note) — not a new security surface, but must be a deliberate, documented choice.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|--------------------|
| swag resolves unexported same-package struct types | Assumption A1 (gates the whole annotation-only approach) | Empirical behavior of a third-party parser not independently confirmed by research (WebSearch was inconclusive) | Annotate `health.go`'s `healthResponse` (unexported) plus 1-2 exported-struct handlers; run `swag init`; inspect `docs/swagger.json` for a complete, non-empty schema for the unexported type before proceeding |
| Generated spec contains no real workspace hashes/paths as example values | Security Domain — Information Disclosure mitigation | Requires human eyes on the final generated document, not mechanically checkable in general | Review `docs/openapi.json` before it's first committed; confirm all `@Success`/example values use placeholder data, never real captured workspace hashes or file paths |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies (except the two explicitly manual-only items above)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 20s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
