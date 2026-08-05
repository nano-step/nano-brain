# multi-language-graph-extractors Specification

## Purpose
TBD - created by archiving change extend-code-intelligence. Update Purpose after archive.
## Requirements
### Requirement: TypeScript graph extraction
The system SHALL extract knowledge graph edges (contains, imports, calls) from TypeScript files (.ts, .tsx) using gotreesitter, following the same pattern as the existing Go graph extractor. The extractor SHALL use `grammars.TypescriptLanguage()` for `.ts` files and `grammars.TsxLanguage()` for `.tsx` files (dual grammar pattern). Call detection SHALL use the two-phase approach: function-body-range query (using `statement_block` node type) then call-expression query within byte ranges.

#### Scenario: TypeScript ES import extraction
- **WHEN** the watcher processes a TypeScript file containing `import { foo } from "./bar"`
- **THEN** a graph edge with edge_type="imports", source_node=file path, target_node="./bar" (quotes stripped) SHALL be persisted in graph_edges

#### Scenario: TypeScript contains extraction
- **WHEN** the watcher processes a TypeScript file containing `function handleRequest() {}`
- **THEN** a graph edge with edge_type="contains", source_node=file path, target_node=file+"::handleRequest" SHALL be persisted

#### Scenario: TypeScript call extraction with enclosing function
- **WHEN** the watcher processes a TypeScript file where function `handleRequest` calls `validateInput`
- **THEN** a graph edge with edge_type="calls", source_node=file+"::handleRequest", target_node="validateInput" SHALL be persisted (two-phase: enclosing function determined via byte range matching)

#### Scenario: TypeScript require() import
- **WHEN** the watcher processes a TypeScript file containing `const x = require("./module")`
- **THEN** a graph edge with edge_type="imports", target_node="./module" SHALL be persisted
- **AND** other function calls with string args (e.g., `console.log("hello")`) SHALL NOT produce import edges

#### Scenario: TSX file uses separate grammar
- **WHEN** the watcher processes a `.tsx` file containing JSX syntax
- **THEN** the extractor SHALL parse it using `TsxLanguage()` grammar, not `TypescriptLanguage()`

### Requirement: JavaScript graph extraction
The system SHALL extract knowledge graph edges from JavaScript files (.js, .jsx) using gotreesitter.

#### Scenario: JavaScript ES import extraction
- **WHEN** the watcher processes a JavaScript file containing `import defaultExport from "./lib"`
- **THEN** a graph edge with edge_type="imports", target_node="./lib" SHALL be persisted

#### Scenario: JavaScript CommonJS require extraction
- **WHEN** the watcher processes a JavaScript file containing `const lib = require("lodash")`
- **THEN** a graph edge with edge_type="imports", target_node="lodash" SHALL be persisted

#### Scenario: JavaScript function contains and calls
- **WHEN** the watcher processes a JavaScript file with function `main` calling `processData`
- **THEN** both a "contains" edge for `main` and a "calls" edge from `main` to `processData` SHALL be persisted

### Requirement: Python graph extraction
The system SHALL extract knowledge graph edges from Python files (.py) using gotreesitter. Call detection SHALL use the two-phase approach with `block` node type for function bodies. Python uses `call` node type (not `call_expression`).

#### Scenario: Python import extraction
- **WHEN** the watcher processes a Python file containing `import os`
- **THEN** a graph edge with edge_type="imports", target_node="os" SHALL be persisted

#### Scenario: Python from-import extraction
- **WHEN** the watcher processes a Python file containing `from pathlib import Path`
- **THEN** a graph edge with edge_type="imports", target_node="pathlib" SHALL be persisted

#### Scenario: Python function and class contains
- **WHEN** the watcher processes a Python file containing `def process():` and `class Handler:`
- **THEN** graph edges with edge_type="contains" SHALL be persisted for both symbols

#### Scenario: Python call extraction
- **WHEN** the watcher processes a Python file where function `main` calls `process()`
- **THEN** a graph edge with edge_type="calls", source_node=file+"::main", target_node="process" SHALL be persisted

#### Scenario: Python module-level assignment vs nested
- **WHEN** the watcher processes a Python file containing `MY_CONST = 42` at module level and `x = 5` inside a function
- **THEN** a "contains" edge SHALL be persisted for `MY_CONST` only — `x` SHALL NOT produce a contains edge

### Requirement: Extractor registration
All new graph extractors SHALL be registered in the graph.Registry at server startup alongside the existing Go extractor.

#### Scenario: All extractors active
- **WHEN** the server starts
- **THEN** the graph registry SHALL contain extractors for Go, TypeScript, JavaScript, and Python

#### Scenario: Extractor failure does not crash server
- **WHEN** a graph extractor constructor returns an error
- **THEN** the server SHALL log a warning and continue startup without that extractor

### Requirement: No schema changes
All new extractors SHALL write to the existing `graph_edges` table using the existing `UpsertGraphEdge` query. No database migration SHALL be required.

#### Scenario: Existing table reuse
- **WHEN** a TypeScript file is processed
- **THEN** edges SHALL be stored in the same `graph_edges` table as Go edges, with `language` in metadata

### Requirement: Build constraint maintained
The binary SHALL continue to build with `CGO_ENABLED=0`.

#### Scenario: Static build passes
- **WHEN** running `CGO_ENABLED=0 go build ./...`
- **THEN** the build SHALL succeed with exit code 0

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

