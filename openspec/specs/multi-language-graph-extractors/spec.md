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

