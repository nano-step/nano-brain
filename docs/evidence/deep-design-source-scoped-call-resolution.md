# Deep-Design Analysis: Source-Scoped Call Resolution (#609)

## Scope and Authorization

This analysis reviews the #609 planning boundary after the user explicitly
started work with `$omo:start-work issue 609`. That command authorizes planning
and execution; it is not recorded as a separate product decision or human
sign-off. Product and design choices remain those captured in the approved
work plan and the `source-scoped-call-resolution` OpenSpec artifacts.

The scope is limited to JavaScript/TypeScript `calls` edges: source-scoped
resolution, safe unresolved representation, reader behavior, and lifecycle
re-extraction. Merged #555 import-specifier canonicalization is a prerequisite.
The stale #501 import-resolution artifact and open #504 remain out of scope and
unchanged.

## Decision Synthesis

| Area | Decision | Reasoning and boundary |
| --- | --- | --- |
| Canonical target | Use a recognized JS/TS canonical identity of `relative/path.ext::symbol`, class method, default export, or default method. | It is exact, source-reachable, and distinguishable from legacy targets such as Ruby namespaces. |
| Unprovable calls | Persist target-only `<unresolved>`. | Unknown, dynamic, ambiguous, external, shadowed, or unsupported calls must not turn into a guessed cross-root link. No reason/raw-callee metadata is added. |
| Resolver | Use an unexported contextual resolver with lexical declarations, direct project-local ES imports, class context, and only parser-proven typed receivers. | It provides local proof without adding a TypeScript compiler, global lookup, or proximity heuristic. |
| Import/export catalog | Resolve only a uniquely resolved relative/configured-alias module with a direct export; parse each target module at most once per extraction invocation and keep no cross-run cache. | Direct exports make declaration identity verifiable. Re-exports, barrels, packages, and missing/ambiguous modules are unresolved. |
| Consumers | Treat canonical JS/TS targets as exact and `<unresolved>` as raw-edge-only. | Graph SQL, impact, trace, flow, PageRank, graph context, code summary, REST, and MCP must not reintroduce fan-out through fallback or derived nodes. |
| Lifecycle | On admitted JS/TS write/create/rename/remove, re-extract contextual files for the affected workspace. | Exporter removal or rename must replace stale importer edges even when the event path no longer parses or exists. |
| Legacy behavior | Preserve legacy bare-target fallback, including Ruby namespaced targets. | The new canonical predicate is language/shape-specific; it must not change historical behavior outside contextual JS/TS calls. |

## Risk Review

| Risk | Disposition |
| --- | --- |
| False cross-root graph links | Positive collision fixtures and exact-reader tests are required before implementation is accepted. |
| A missing canonical declaration falls back to a bare symbol | Reader tests must prove no canonical-to-bare fallback in trace, impact, flow, REST, or MCP. |
| `<unresolved>` becomes a traversal hub | Exclude it before derived node, topology, degree, PageRank, cache, frontier, flow, and response-count work. |
| Stale edges survive exporter changes | Watcher lifecycle coverage must exercise write, create, rename, removal, repeated update, and preservation of other sources. |
| Scope creep or parser guessing | Reject CommonJS, re-export chains, dynamic calls, broad DI inference, and a whole-project type checker. |
| Privacy leakage | Use synthetic fixtures and placeholder identifiers only; run the repository privacy scan before every commit and do not record private workspace data in evidence. |
| Public graph quality regression | Require isolated `nanobrain_test`/`:3199` REST/MCP smoke, benchmark comparison, and independent review before archive. |

Alternatives rejected: query-time proximity remains insufficient because it
cannot prove the callee; global same-name lookup creates false edges; a
whole-project type checker and framework-wide dependency injection analysis
expand scope beyond the evidence-backed contract.

## Verdict

**PASS for planning.** The canonical-target, resolver/catalog, reader,
lifecycle, privacy, scope, and risk decisions are sufficiently bounded for the
next planned tasks. This is a durable planning analysis, not an implementation
review, independent-review verdict, archive approval, or fabricated human
product decision. The remaining high-risk validation and review gates stay
pending until implementation evidence exists.
