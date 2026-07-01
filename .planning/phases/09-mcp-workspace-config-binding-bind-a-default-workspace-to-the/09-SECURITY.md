---
phase: 09
slug: mcp-workspace-config-binding-bind-a-default-workspace-to-the
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-07-01
---

# Phase 09 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| MCP client (`.mcp.json`) → HTTP server | The `?workspace=` value on the connection URL is attacker-controlled-if-the-config-file-is; `.mcp.json` is a locally-trusted file per this project's existing localhost/PNA threat model | Workspace name/hash string |
| HTTP middleware → SDK tool dispatch | The injected context value crosses into the SDK's `req.Context()`, then into every tool handler's `ctx` | Workspace name/hash string (raw, unresolved) |
| Tool handler → storage resolution | `requireWorkspace`/`requireRegisteredWorkspace` pass the value (arg OR context default) into `storage.ResolveWorkspaceParam` and, for writes, the `GetWorkspaceByHash` registration check | Resolved workspace hash |
| MCP server (tools/list) → MCP client (agent) | The tool schema is served to the agent as API-contract metadata; dropping required-ness of `workspace` changes what the agent is compelled to send | JSON schema (no sensitive data) |
| Real HTTP client → Echo `/mcp` route → SDK | The integration test exercises the true production boundary: query string on the raw `*http.Request` must survive into the SDK's `req.Context()` | Workspace name/hash string over HTTP |
| Human operator → `.mcp.json` config | Docs instruct the operator on the `?workspace=` value; the value is locally-trusted config per the existing threat model | Documentation only |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-09-01 | Elevation of Privilege | `requireRegisteredWorkspace` (write path) | high | mitigate | Connection-default value flows through the SAME `requireWorkspace` → `ResolveWorkspaceParam` → `GetWorkspaceByHash` registration path as an explicit arg (no laxer path). Verified: `internal/mcp/tools.go:187-204` (delegation), `TestRequireRegisteredWorkspace_UsesConnectionDefault` — PASS | closed |
| T-09-02 | Spoofing / wrong-workspace resolution | `requireWorkspace` fallback | high | mitigate | The `?workspace=` value is resolved by the identical `storage.ResolveWorkspaceParam` used for explicit args — no second resolution path. Verified: `internal/mcp/tools.go:168` (single call site) | closed |
| T-09-03 | Information Disclosure | `requireWorkspace` fallback (D-03 precedence) | high | mitigate | Explicit per-call `workspace` arg is read FIRST and always wins; a compromised connection default can never override an explicit arg. Verified: `internal/mcp/tools.go:155-161`, `TestRequireWorkspace_ExplicitArgWins` — PASS | closed |
| T-09-04 | Tampering (schema drift) | `toolSchema` required-fields lists | medium | mitigate | Negative-grep verify gate proves no required list still carries `"workspace"`; schema-assertion test proves the parameter is still PRESENT (only required-ness drops). Verified: `TestToolSchema_WorkspaceNotRequired` — PASS | closed |
| T-09-05 | Denial of Service / broken tool | 4 excluded tools (Pitfall 3) | medium | mitigate | `TestToolSchema_WorkspaceNotRequired` explicitly asserts `memory_workspaces_resolve` keeps `required:[path]` and `memory_ticket` keeps `required:[ticket]` | closed |
| T-09-06 | Elevation of Privilege (schema-only) | Dropping required `workspace` | low | accept | Dropping schema required-ness cannot itself grant access — the runtime gate (`requireWorkspace` error on no-arg-no-default, D-04) is enforced independent of schema. Verified: `TestRequireWorkspace_NoArgNoDefaultErrors`, `TestStreamableHTTP_ConnectionDefaultWorkspace/no_query_param_and_no_arg_still_requires_workspace` — both PASS | closed |
| T-09-07 | Spoofing / wiring regression | `WrapStreamableHandler` HTTP wiring | high | mitigate | Full-HTTP integration test drives a real query string through the real `echo.WrapHandler(WrapStreamableHandler(...))` path — the only test that would catch a re-wiring as `echo.MiddlewareFunc`. Verified: `TestStreamableHTTP_ConnectionDefaultWorkspace` — PASS | closed |
| T-09-08 | Elevation of Privilege | `requireRegisteredWorkspace` write path over connection default | high | mitigate | Write-path test proves the connection default still passes through the registration DB check (`GetWorkspaceByHash`) and that no-default+no-arg still errors. Verified: `TestRequireRegisteredWorkspace_UsesConnectionDefault` — PASS | closed |
| T-09-09 | Information Disclosure (docs) | `.mcp.json` `?workspace=` documentation | low | accept | Docs describe an additive, locally-trusted config surface; the `?workspace=` value is no more attacker-reachable than the existing explicit `workspace` arg — same trust boundary, no new enumeration surface | closed |
| T-09-SC | Tampering | npm/pip/cargo installs | n/a | accept | No package installs across all 3 plans in this phase (zero new dependencies; `git diff go.mod` empty for the phase) | closed |

*Status: open · closed · open — below {block_on} threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-09-01 | T-09-06 | Elevation-of-privilege risk from schema-only change is fully mitigated by the independent runtime gate (D-04); no schema-level control needed on top of it | Planning-time disposition (09-02-PLAN.md), re-confirmed at security audit | 2026-07-01 |
| AR-09-02 | T-09-09 | Docs describing an additive, locally-trusted `.mcp.json` config surface carry no new attacker-reachable surface beyond the existing explicit `workspace` arg | Planning-time disposition (09-03-PLAN.md), re-confirmed at security audit | 2026-07-01 |
| AR-09-03 | T-09-SC | No package installs in this phase across all 3 plans | Planning-time disposition (all 3 plans), re-confirmed at security audit | 2026-07-01 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-07-01 | 10 | 10 | 0 | Claude (gsd-secure-phase, short-circuit path — threats_open: 0, register_authored_at_plan_time: true, asvs_level: 1) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-01
