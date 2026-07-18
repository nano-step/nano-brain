# Tasks: Source-Scoped JavaScript and TypeScript Call Resolution

## 1. Contract and ownership

- [ ] Record the #609 high-risk intake, ownership disposition, strict OpenSpec
  status/validation, and #504 non-overlap evidence without changing #501 or
  #504.
- [ ] Create a parser-feasibility and fixture matrix for supported declarations,
  imports, lexical scopes, JS hoisting, class ownership, and explicitly
  supported TS receivers. Define `IsCanonicalJSCallTarget`; prove Ruby
  namespaces remain legacy controls.
- [ ] Add initially failing focused fixtures for every admitted and excluded
  form. Each positive fixture must pair its canonical target with the same
  emitted declaration identity; legacy bare extraction remains covered.
- [ ] Audit every production graph target consumer. Record canonical,
  unresolved, legacy-bare, topology/degree/cache policy, and focused test for
  each SQL, graph, flow, symbol, REST, MCP, PageRank, and code-summary path.
  Account for the documented target-consumer searches before reader work starts.

## 2. Contextual extraction

- [ ] Implement the smallest unexported shared resolver. It models per-file
  declarations, direct-export project-local ES import bindings, lexical scope,
  class context, and only the parser-proven receiver forms from the contract.
  Use a per-invocation direct-export catalog with no shared cache; parse each
  target module no more than once.
- [ ] Resolve local/direct-import calls to canonical targets and all unsupported,
  duplicate, ambiguous, external, unavailable, dynamic, or shadowed calls to
  target-only `<unresolved>`. Do not change the legacy extractor interface.
- [ ] Wire the contextual route into JavaScript extraction while preserving
  existing bare-call behavior. Emit stable identities for class methods,
  default exports, and namespace members.
- [ ] Wire the contextual route into TypeScript extraction. Support only
  `this.field.method()` for a simple named same-file/direct-import class type
  and access-modified simple named constructor parameter properties; reject all
  other dependency-injection and inferred/complex type patterns.
- [ ] On admitted JS/TS watcher write, create, rename, and remove events,
  invoke workspace-local contextual re-extraction even when the event file is
  absent or unparsable. Prove exporter rename/removal clears stale importer
  edges.

## 3. Reader safety and lifecycle proof

- [ ] Apply the reader-audit contract: canonical JS/TS targets are exact and
  never suffix-expanded; `<unresolved>` is filtered before all derived work;
  legacy targets keep their established behavior. Cover graph SQL, traversal,
  impact, flow, PageRank, graph context, code-summary, REST, and MCP paths.
- [ ] Add unit and integration regressions for canonical collision isolation,
  absent canonical targets, unresolved traversal exclusion, Ruby compatibility,
  source-edge replacement, repeated update idempotence, and preservation of
  other source files.
- [ ] Run a bounded isolated `nanobrain_test`/`:3199` smoke with an exact PID
  cleanup path. Prove REST graph output and streamable-HTTP MCP trace/impact
  see only the source-reachable canonical target, then prove exporter removal
  yields `<unresolved>`.

## 4. Validation and delivery

- [ ] Run `go build ./... && go test -race -short ./...`, integration tests
  against `nanobrain_test`, focused fixture commands, the isolated smoke, and
  the applicable graph capability benchmark. Save timestamped raw output and
  a durable summary; do not use the development database or broad process
  termination.
- [ ] Run strict OpenSpec validation, privacy scan, diff self-review, and an
  independent review. Resolve review findings before archive or delivery.
- [ ] Confirm only #609 behavior is staged, archive only after all high-risk
  gates pass, and open the #609 pull request from the feature branch.
