# Source-Scoped JavaScript and TypeScript Call Resolution

## Why

JavaScript and TypeScript extractors persist many `calls` edges with a bare
callee name. Readers can reconcile that name with unrelated symbols, producing
impossible cross-root links, self-loops, and false trace, flow, and impact
results. Query-time proximity mitigation does not prove which callee a source
file can invoke.

The system must resolve only calls that are locally provable from the source
file and its project-local direct imports. Every other contextual call must be
explicitly unresolved rather than represented as a guessed qualified edge.

## What Changes

- Add a contextual JavaScript/TypeScript extraction path that emits one
  source-reachable canonical target for supported local declarations, direct
  project-local named/default/namespace imports, local class methods, and the
  explicitly supported typed receivers.
- Emit the target-only `<unresolved>` sentinel when a target cannot be proved.
- Preserve canonical JS/TS identities exactly in graph readers, REST, MCP,
  trace, impact, flow, PageRank, and graph-context processing; unresolved
  edges remain diagnostic raw edges only and cannot become traversal hubs.
- Re-extract contextual files on admitted JS/TS changes so importer edges are
  replaced when an exporter changes, is renamed, or is removed.
- Add collision, unresolved, legacy compatibility, lifecycle, and isolated
  `nanobrain_test` / `:3199` validation coverage.

## Non-Goals

- No whole-project TypeScript compiler, CommonJS, re-export chain, dynamic-call,
  heuristic proximity, or global symbol-resolution support.
- No schema migration, no behavior change to legacy non-contextual extraction,
  and no change to the historical bare-edge compatibility policy outside the
  documented reader guards.
- No changes to the completed #501 import-specifier resolution work or its open
  historical pull request.

## Impact

This is a high-risk bug fix: it changes graph identity and affects public graph
surfaces. It builds on the merged JS/TS import-path canonicalization from #555,
but owns only call-edge resolution and reader safety. The change requires
unit, integration, isolated smoke, benchmark comparison, strict OpenSpec
validation, and independent review before delivery.
