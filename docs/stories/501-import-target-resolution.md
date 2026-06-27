---
github_issue: nano-step/nano-brain#501
openspec_change: openspec/changes/fix-import-target-resolution
lane: high-risk
change_type: bug-fix
risk_flags:
  - data-model
  - existing-behavior
  - search-quality
hard_gates:
  - data-model
branch: feat/501-import-target-resolution
worktree: .opencode/worktrees/501-import-target-resolution
---

# US-501 Import Target Resolution + Workspace-Relative Canonical Node Form

## Status

specs-locked — deep-design PASS (architecture + scope/risk + verification), design
revised, openspec validates. Next: implement (delegated) → validate → review gate.

## Lane

**high-risk** — escalated from `normal` after deep-design revealed the fix touches
the **data model** (graph node identity / canonical form) across every graph tool
and requires a reindex of all workspaces. Hard gate: data-model. The canonical-form
direction (workspace-relative end-to-end) was confirmed with the user.

## GitHub Issue

`nano-step/nano-brain#501`

## OpenSpec Change

`openspec/changes/fix-import-target-resolution/` — proposal, design, tasks,
`specs/multi-language-graph-extractors/spec.md`.

## Deep-Design Verdict

Both reviewers (architecture + scope/risk) + orchestrator verification converged:
the original design's canonical form was backwards (query→absolute via
`graph_paths.go:45`, storage→relative via #450). Decision: **workspace-relative,
end-to-end** — includes changing `resolveNodeAgainstWorkspace`.

## Acceptance Criteria

- **AC1** `resolveNodeAgainstWorkspace` returns the workspace-relative form (not absolute); `graph_paths_test.go` updated to match.
- **AC2** Import edge `target_node` is resolved to a workspace-relative path that byte-matches the stored `source_node` form of the same file.
- **AC3** After `POST /api/v1/reindex`, reverse lookup (`memory_graph direction=in` / `memory_impact`) on an aliased file's relative path returns its importers (0 → N).
- **AC4** Scoped npm (`@org/pkg`) and bare (`ramda`, `lodash/fp`) specifiers pass through unchanged (`@/` alias rule does not match `@org/`).
- **AC5** Unresolvable/ambiguous specifiers fall back to the raw spec (never an extensionless half-path); a warning is logged; no edge dropped.
- **AC6** Relative imports (`./`, `../`) resolve against the source file's directory.
- **AC7** Reindex is idempotent — no raw/resolved duplicate edges (delete-by-source_file tx).
- **AC8** `go build` + `go test -race -short` + `go test -race -tags=integration` green; smoke:e2e on a committed fixture proves 0 → N.

## Test Strategy

- Unit: `import_resolver_test.go` (decision-table cases), `graph_paths_test.go` (relative output), canonical-form byte-equality test.
- Integration: reindex a committed fixture `internal/graph/testdata/alias-import/` → assert `GetIncomingEdges` returns the importer.
- smoke:e2e: server on :3199 (nanobrain_test), index fixture, `memory_graph(node=<rel file>, direction=in)`.

## Harness Delta

None (no new harness rule introduced by this story).

## Out of Scope

`export … from` / dynamic `import()` extraction; Nuxt auto-imported
components/composables (Vue SFC work); monorepo workspace-package mapping.
