---
phase: 09-mcp-workspace-config-binding-bind-a-default-workspace-to-the
plan: 02
subsystem: api
tags: [mcp, tool-schema, json-schema, workspace-resolution]

# Dependency graph
requires: ["09-01"]
provides:
  - "14 memory_* tool schemas no longer require \"workspace\" (D-06) while keeping the property present and overridable"
  - "TestToolSchema_WorkspaceNotRequired schema-assertion test (internal/mcp/tools_schema_test.go)"
affects: [09-03]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "toolSchema(...) required-fields removal via targeted []string{...} literal edits, verified by negative grep + JSON-schema-decoding unit test"

key-files:
  created:
    - internal/mcp/tools_schema_test.go
  modified:
    - internal/mcp/tools.go
    - internal/mcp/flowchart.go

key-decisions:
  - "Descriptions for all 14 edited workspace properties append the same D-06 optional-note verbatim: 'Optional if the MCP connection was configured with a default workspace via the ?workspace= URL query param; otherwise required.'"
  - "toolSchema(props, required) only sets the JSON 'required' key when len(required) > 0, so []string{} and nil are equivalent no-required-fields representations — used []string{} for the sites that previously had only {\"workspace\"} required, matching the plan's literal instruction"
  - "Schema-assertion test decodes Tool.InputSchema by re-marshaling to JSON and unmarshaling into a minimal {Properties, Required} struct, since the SDK's ClientSession.ListTools returns InputSchema as `any` over the wire (no exported jsonschema.Schema struct to unmarshal into directly)"

patterns-established: []

requirements-completed: []

coverage:
  - id: D6-tools
    description: "13 tools.go toolSchema required-fields lists no longer contain \"workspace\"; property + updated description retained"
    verification:
      - kind: unit
        ref: "go build ./internal/mcp/... && negative grep '\\}, \\[\\]string\\{[^}]*\"workspace\"' internal/mcp/tools.go (no matches)"
        status: pass
      - kind: unit
        ref: "internal/mcp/tools_schema_test.go#TestToolSchema_WorkspaceNotRequired"
        status: pass
    human_judgment: false
  - id: D6-flowchart
    description: "memory_flowchart (flowchart.go) required-fields list no longer contains \"workspace\"; property + updated description retained"
    verification:
      - kind: unit
        ref: "go build ./internal/mcp/... && negative grep on internal/mcp/flowchart.go (no matches)"
        status: pass
      - kind: unit
        ref: "internal/mcp/tools_schema_test.go#TestToolSchema_WorkspaceNotRequired"
        status: pass
    human_judgment: false
  - id: D6-excluded
    description: "memory_status/memory_workspaces_resolve/memory_workspaces_list/memory_ticket required-fields contracts unchanged (RESEARCH Pitfall 3)"
    verification:
      - kind: unit
        ref: "internal/mcp/tools_schema_test.go#TestToolSchema_WorkspaceNotRequired (asserts path/ticket required, no workspace regression)"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-07-01
status: complete
---

# Phase 9 Plan 2: MCP workspace config binding — schema visibility Summary

**Dropped `"workspace"` from the required-fields list of the 14 `memory_*` tools (13 in tools.go + memory_flowchart in flowchart.go), keeping the parameter itself present and overridable, so an LLM agent is no longer schema-forced to supply it when the connection has a bound default (D-06).**

## Performance

- **Duration:** ~12 min (commit-to-commit, 0af00e7 → 972091b)
- **Tasks:** 3
- **Files modified:** 3 (2 modified, 1 created)

## Accomplishments
- Removed `"workspace"` from the `toolSchema(...)` required-fields list at all 13 enumerated sites in `internal/mcp/tools.go` (`memory_query`, `memory_search`, `memory_vsearch`, `memory_get`, `memory_write`, `memory_tags`, `memory_update`, `memory_wake_up`, `memory_graph`, `memory_trace`, `memory_impact`, `memory_symbols`, `memory_flow`), appending the D-06 optional-note to each tool's `workspace` property description
- Applied the identical mechanical edit to `memory_flowchart` in `internal/mcp/flowchart.go` (the 14th site, in a separate file)
- Left the 4 excluded tools (`memory_status`, `memory_workspaces_resolve`, `memory_workspaces_list`, `memory_ticket`) completely untouched — none of them ever required `"workspace"`, and none do now
- Added `internal/mcp/tools_schema_test.go` with `TestToolSchema_WorkspaceNotRequired`, which decodes each tool's live `InputSchema` (via the in-memory MCP transport, no Postgres needed) and asserts: the 14 edited tools have `workspace` in `properties` but not in `required`; `memory_workspaces_resolve` still requires `path`; `memory_ticket` still requires `ticket`; `memory_workspaces_list`/`memory_status` show no regression

## Task Commits

Each task was committed atomically:

1. **Task 1: Remove "workspace" from required-fields + update descriptions across 13 tools.go sites** - `0af00e7` (feat)
2. **Task 2: Remove "workspace" from memory_flowchart required-fields + update description** - `d9fac6f` (feat)
3. **Task 3: Add schema-assertion test for the 14 edited + 4 excluded tools** - `972091b` (test)

**Plan metadata:** (this commit, following SUMMARY.md write)

## Files Created/Modified
- `internal/mcp/tools.go` - 13 `toolSchema(...)` required-fields lists edited; `workspace` property descriptions updated with the D-06 optional-note
- `internal/mcp/flowchart.go` - `memory_flowchart`'s required-fields list edited; `workspace` property description updated
- `internal/mcp/tools_schema_test.go` (new) - `TestToolSchema_WorkspaceNotRequired`, asserting the 14-tool/4-excluded-tool schema contract

## Decisions Made
- Used the exact D-06 optional-note wording specified by the plan for every edited `workspace` description, keeping all 14 sites textually consistent
- Where the plan called for `{"workspace"}` -> `{}` (memory_tags, memory_update, memory_wake_up, memory_symbols), used the empty-slice literal `[]string{}` directly rather than `nil`, since `toolSchema` treats both identically (`len(required) > 0` gate) and the plan's own `<action>` text specifies `{}`
- Decoded `Tool.InputSchema` in the new test by round-tripping through `json.Marshal`/`json.Unmarshal` into a small local `{Properties, Required}` struct, since the SDK exposes `InputSchema` as `any` and the vendored `go-sdk@v0.8.0` module does not expose the `jsonschema.Schema` struct fields for direct type-assertion in this test's import scope

## Deviations from Plan

None — plan executed exactly as written. All 13 tools.go required-fields lists and the flowchart.go one match the plan's `<action>` blocks line-for-line (verified against the RESEARCH.md enumeration table, accounting for a small line-number shift caused by 09-01's earlier edits to the same file).

## Issues Encountered

None. The negative-grep verification gate (`\}, \[\]string\{[^}]*"workspace"` matches nothing) passed on the first attempt for both `tools.go` and `flowchart.go`, and `TestToolSchema_WorkspaceNotRequired` passed on the first run with no debugging needed.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- All 14 tool schemas now advertise `workspace` as optional-but-present; combined with Plan 01's runtime context-fallback, an agent connecting via a `?workspace=`-bound `.mcp.json` URL can omit the argument end-to-end
- Plan 03 (broader security/integration tests) can now build on both the runtime fallback (Plan 01) and the schema visibility (this plan) without further groundwork here
- `go build ./...` and `go test -race -short ./internal/mcp/...` are green; `TestRegisterTools_CountAndNames` (18 tools) still passes unchanged

---
*Phase: 09-mcp-workspace-config-binding-bind-a-default-workspace-to-the*
*Completed: 2026-07-01*

## Self-Check: PASSED

All created/modified files exist on disk (tools.go, flowchart.go, tools_schema_test.go, SUMMARY.md) and all three task commits (0af00e7, d9fac6f, 972091b) are present in git history.
