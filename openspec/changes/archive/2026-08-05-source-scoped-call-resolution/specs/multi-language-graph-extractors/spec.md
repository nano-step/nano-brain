## ADDED Requirements

### Requirement: Source-scoped JavaScript and TypeScript call targets

The system SHALL use contextual extraction to emit a canonical JavaScript or
TypeScript `calls` target only when the target is provable from the source file:
a same-file declaration, direct project-local named/default/namespace import,
local class method, or explicitly supported parser-proven typed receiver. The
canonical target SHALL exactly equal an emitted declaration identity. The system
SHALL emit target-only `<unresolved>` for ambiguous, dynamic, shadowed,
unsupported, external, or unavailable targets. Existing non-contextual
extraction SHALL continue to emit its legacy bare targets.

#### Scenario: Same-named declarations in separate roots

- **WHEN** a contextual source file calls a directly imported declaration and a
  separate project root contains a declaration with the same bare name
- **THEN** the persisted `calls` target SHALL be only the imported module's
  canonical declaration identity
- **AND** the separate-root declaration SHALL not be selected

#### Scenario: Contextual target cannot be proved

- **WHEN** a contextual call is dynamic, ambiguous, shadowed, externally
  imported, or uses an unsupported receiver form
- **THEN** its persisted target SHALL be `<unresolved>`
- **AND** no guessed canonical target SHALL be emitted

#### Scenario: Exporter lifecycle invalidates importers

- **WHEN** an admitted JS/TS exporter is written, created, renamed, or removed
- **THEN** contextual JS/TS files in that workspace SHALL be re-extracted
- **AND** stale importer call edges SHALL be replaced without deleting edges
  from unrelated source files

### Requirement: Canonical call target reader safety

The system SHALL treat a canonical JavaScript or TypeScript call target as an
exact identity in graph SQL, traversal, impact, flow, graph context, PageRank,
REST, and MCP readers. It SHALL not use bare-name or suffix fallback for that
target. The system MAY expose `<unresolved>` only as its originating raw
one-hop edge and SHALL exclude it from derived traversal, node, topology,
aggregation, cache, frontier, flow, and response-count work. Historical bare
targets, including Ruby namespaced targets, SHALL retain their tested legacy
compatibility behavior.

#### Scenario: Canonical target does not fan out

- **WHEN** a reader processes a canonical JS/TS call target whose declaration
  is absent or whose bare symbol exists elsewhere
- **THEN** the reader SHALL not substitute a same-named bare declaration
- **AND** the target SHALL not create a cross-root trace, impact, or flow edge

#### Scenario: Unresolved target remains non-traversable

- **WHEN** a reader processes a persisted `<unresolved>` call edge
- **THEN** it MAY show that originating raw edge where supported
- **BUT** it SHALL not create a derived node, traversal expansion, topology or
  degree contribution, PageRank entry, flow node, or derived REST/MCP count

#### Scenario: Legacy namespaced target remains compatible

- **WHEN** a reader processes a historical Ruby namespaced target
- **THEN** the canonical JS/TS predicate SHALL not classify it as canonical
- **AND** the reader SHALL retain its existing tested legacy behavior
