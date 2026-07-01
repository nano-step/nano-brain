# Phase 9: MCP workspace config binding - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-01
**Phase:** 9-MCP workspace config binding
**Areas discussed:** Config surface, Resolution precedence, Schema visibility, Integration point

Mode: `--auto` (autonomous, per project's Autonomous Delivery Protocol) — no interactive prompts; recommended options selected and logged inline based on prior direct codebase/SDK research already performed in-session.

---

## Config surface — how does `.mcp.json` carry the default workspace?

| Option | Description | Selected |
|--------|-------------|----------|
| URL query param (`?workspace=<name>`) | Works with any HTTP-based MCP client, zero client-schema support needed, survives stateless transport since client resends the full URL every request | ✓ |
| Custom HTTP header | Requires the MCP client's config schema to support arbitrary headers — not verified as universally supported | |
| Env var | HTTP transport has no client-supplied env passthrough (unlike stdio) | |

**Selected:** URL query param, param name `workspace` (matches the existing tool-call arg name), accepting the same name-or-hash values `requireWorkspace` already accepts. `"all"` is not accepted as a connection default.
**Notes:** [auto] Config surface — Q: "How should `.mcp.json` carry a default workspace?" → Selected: "URL query param" (recommended default; only option requiring zero client-side schema support).

---

## Resolution precedence — what wins when both a connection default and a per-call arg exist?

| Option | Description | Selected |
|--------|-------------|----------|
| Per-call arg always wins | Connection default is a pure fallback for omitted args | ✓ |
| Connection default always wins | Locks the connection to one workspace, ignoring per-call overrides | |

**Selected:** Per-call arg always wins; connection default only fills in when the arg is omitted. Omitted arg + no default = today's exact `"workspace is required"` error, unchanged.
**Notes:** [auto] Resolution precedence — Q: "Does an explicit per-call workspace arg override the connection default?" → Selected: "Yes, always" (recommended; preserves 100% backward compatibility and still allows explicit cross-workspace `"all"` queries on a bound connection).

---

## Write-tool scope — does the default apply to `memory_write`/`memory_update` too, or reads only?

| Option | Description | Selected |
|--------|-------------|----------|
| Apply to both read and write tools | A `.mcp.json` entry is inherently pinned to one project already | ✓ |
| Reads only, writes always require explicit arg | Extra safety margin against mis-writes | |

**Selected:** Apply uniformly to both `requireWorkspace` and `requireRegisteredWorkspace`. A per-project `.mcp.json` binding isn't an ambiguous multi-tenant default — it's already scoped to one project by whoever wrote that config — so there's no added mis-write risk versus today.
**Notes:** [auto] Write-tool scope — Q: "Should memory_write/memory_update also honor the connection default?" → Selected: "Yes, both" (recommended; consistent behavior across all tools, no safety regression given the binding is per-project).

---

## Schema visibility — does the JSON schema still list `workspace` as required?

| Option | Description | Selected |
|--------|-------------|----------|
| Drop from required list, update description | Lets the LLM legally omit it when it knows/assumes a default may be configured | ✓ |
| Keep required in schema, only relax at runtime | Agent would always pass something even when unnecessary, defeating the point | |

**Selected:** Drop `"workspace"` from the required-fields list in `toolSchema(...)` across all ~18 `memory_*` tool registrations; update each description to explain the optional-with-fallback behavior.
**Notes:** [auto] Schema visibility — Q: "Should `workspace` remain required in the tool schema?" → Selected: "No, make optional" (recommended; required-in-schema would keep forcing the exact friction this phase exists to remove).

---

## Claude's Discretion

- Exact Go context-key type/name for the injected default-workspace value (implementation detail).
- Eager vs. lazy resolution of the query-param value to a workspace hash inside the middleware vs. inside `requireWorkspace` (either is correct; left to planner/executor to pick the smaller diff).

## Deferred Ideas

- Proactive `UserPromptSubmit`-style auto-injection hook (removes the "agent forgets to call wake_up at all" failure mode — separate, larger capability from this phase's "agent doesn't need to discover the workspace hash" scope).
- Deprecating/consolidating low-usage tools (`memory_ticket`, `memory_workspaces_list`, `memory_tags`) — identified from the same real-usage-data analysis this phase is motivated by, but unrelated capability.
- New language/framework extractors (PHP, Java) — unrelated capability.
