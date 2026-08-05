# #609 Self-review

## Scope

Reviewed the complete `fix/609-source-scoped-call-resolution` diff against
`origin/master`. The change is limited to the #609 OpenSpec artifacts,
source-scoped JS/TS extraction, watcher re-extraction, graph-reader safety,
generated SQL mirrors, focused regressions, and durable evidence.

## Contract checks

- Canonical JS/TS targets are admitted only for exact relative source paths and
  emitted declaration identities; canonical impact lookup never falls back to
  a bare symbol.
- Direct named/default/namespace imports use the per-invocation direct-export
  catalog. The AST import walker handles aliases and exported names that are
  also keywords such as `from`.
- Same-file declarations, lexical shadowing, class methods, and the two
  parser-proven TypeScript receiver forms are covered. Dynamic, external,
  missing, re-exported, ambiguous, inferred, decorated, and shadowed forms
  produce target-only `<unresolved>`.
- Plain `ExtractEdges` remains the legacy bare-call route. The contextual route
  is used only when import context is available.
- Watcher write/create/rename/remove events mark admitted JS/TS workspaces for
  contextual re-extraction, and source replacement removes stale importer
  edges without touching the collision fixture's unrelated source edge.
- `<unresolved>` is excluded from flow, impact frontier, trace, neighborhood,
  PageRank, degree/count, graph context, code-summary cache inputs, and derived
  REST/MCP mappings while raw one-hop diagnostics remain available where the
  contract permits them.
- Source SQL and generated SQL mirrors were compared after each query edit;
  no schema migration or new persisted metadata was introduced.

## Verification

- Quick gate: `CGO_ENABLED=0 go build ./... && go test -race -short ./...` — PASS.
- Targeted storage regression after the final SQL reader edits — PASS against
  `nanobrain_test`; canonical exact and bare legacy target lookups remain
  isolated, and unresolved call rows stay out of derived counts.
- Relevant integration gate against `nanobrain_test` — PASS for graph,
  storage, watcher, flow, and codesummarize.
- Focused handler/MCP controls and streamable HTTP integration — PASS.
- Strict OpenSpec validation and in-progress harness gate — PASS.
- Independent read-only review — PASS after resolving the findings on
  unknown member targets, function/class `this` scope, variable initializers,
  multi-declarator exports, watcher lifecycle coverage, and anonymous default
  class parity.
- Privacy scan over changed files and `git diff --check` — PASS; no private
  workspace identifiers or paths are present in the #609 diff.

## Known environment limits

The full repository integration suite retains unrelated baseline failures in
workspace-resolution, harvest, summary compatibility, and the benchmark's
missing live `:3199` server. The bounded live REST/MCP probe and capability
benchmark were therefore recorded as environment-blocked/skip; no server was
started and port 3100 was not touched.

## Verdict

PASS. No critical or major issue remains in the local diff or the independent
review. The live `:3199` smoke limitation is environmental and documented.
