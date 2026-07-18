# Design: Source-Scoped JavaScript and TypeScript Call Resolution

## Context

The existing import resolver establishes a workspace-relative identity for
project-local JS/TS modules. That path identity is a prerequisite, not a call
resolver: a bare `run()` edge still does not prove which same-named declaration
the source can invoke. #609 therefore resolves call identity during contextual
extraction and makes downstream readers distinguish canonical JS/TS targets
from historical bare targets.

The design deliberately separates three target classes:

1. Canonical JS/TS targets have the form
   `relative/path.ext::symbol`, `relative/path.ext::Class.method`,
   `relative/path.ext::default`, or `relative/path.ext::default.method`.
2. `<unresolved>` records that a contextual call was observed but cannot be
   proven safely.
3. Legacy targets retain their current compatibility behavior, including Ruby
   namespaced symbols that contain `::` but are not canonical JS/TS targets.

## Goals and Non-Goals

The resolver may use parser-proven lexical bindings, declarations, direct
project-local ES imports, class context, and the limited typed receiver forms
defined by the implementation contract. Resolution must use the innermost
scope and reject dynamic, ambiguous, shadowed, unsupported, external, missing,
or re-exported targets.

The resolver must not introduce a TypeScript compiler, cross-run resolver
cache, global lookup, directory-proximity heuristic, CommonJS support, or
guessed dependency-injection analysis. It parses a target module at most once
per contextual extraction invocation and accepts only direct exports.

## Contract

Contextual extraction emits a canonical target only when its exact declaration
identity is emitted in the same project graph. Otherwise it emits
`<unresolved>` as the target only; no reason or raw-callee metadata is added to
the persisted graph contract.

Readers treat a canonical JS/TS target as exact identity and never fall back to
bare-name or suffix matching. `<unresolved>` may be retained as the originating
raw one-hop edge, but is excluded before derived node construction, traversal,
topology and degree aggregation, PageRank, cache/index work, frontier growth,
flow materialization, response counts, and REST/MCP derived mappings. Legacy
bare targets retain their tested fallback behavior.

## Implementation Boundaries

The shared resolver is unexported and is used only by a new contextual route;
the existing extractor interface and legacy bare-call output remain compatible.
Watcher write, create, rename, and remove events for admitted JS/TS paths must
trigger workspace-local contextual re-extraction even if the event file is no
longer present or cannot parse. This replaces stale importer edges after an
exporter lifecycle change without touching other source files.

The implementation inventory covers the graph registry and watcher; graph SQL;
impact, trace, neighborhood, overview, flow, flowchart, PageRank, graph-context
and code-summary consumers; and their REST/MCP mappings. The reader audit is a
closed manifest: any newly found production target consumer is added before
reader implementation begins.

## Validation and Rollout

Focused fixtures first prove positive source-reachable calls and negative
same-name, dynamic, ambiguous, shadowed, and unsupported calls. Integration
coverage exercises source replacement, repeated update, importer invalidation
after exporter rename/removal, and preservation of unrelated source edges.

All runtime checks use the isolated `nanobrain_test` database and port `3199`.
The final high-risk ladder includes build/unit, integration, bounded REST/MCP
smoke, capability benchmark comparison, strict OpenSpec validation, privacy
scan, diff review, and independent review. Existing workspaces receive the new
identity through the documented update/re-extraction lifecycle; no migration is
introduced.
