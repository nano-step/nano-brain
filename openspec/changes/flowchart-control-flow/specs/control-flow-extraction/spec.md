## ADDED Requirements

### Requirement: Conditional label enrichment (Phase 1a)
The system SHALL enrich existing `conditional` boolean flags on graph edges with predicate text. When a call edge is inside a conditional block (if/switch), the edge metadata SHALL include a `condition_label` field containing the normalized predicate source text.

#### Scenario: Conditional edge carries predicate label
- **WHEN** a JS/TS call edge is inside an `if` block with condition `err !== null`
- **THEN** the edge metadata includes `"condition_label": "err !== null"`
- **AND** the `conditional` boolean remains `true`

#### Scenario: Long predicates are truncated
- **WHEN** a condition predicate exceeds 80 characters
- **THEN** the `condition_label` is truncated to 80 characters with `…` suffix
- **AND** the full predicate remains in `metadata.condition_raw`

### Requirement: Intra-procedural CFG extraction for JS/TS (Phase 1b)
The system SHALL provide a `ControlFlowExtractor` that, given a JS/TS file, produces control-flow graphs (CFGs) for all functions in the file: `start`/`step`/`decision`/`terminal`/`merge` nodes and `branch`-labeled edges (`yes`/`no` | `case:<value>` | `default` | `loop` | `next`).

#### Scenario: Guard-clause handler produces decisions and terminals
- **WHEN** extracting a handler that does `if (err) { res.status(400).json({error}); return; } ... res.status(200).json(data)`
- **THEN** the CFG contains a `decision` node for the error check
- **AND** a `terminal` node `400` reached by the `yes` branch
- **AND** a `terminal` node for the success response reached by the `no` path

#### Scenario: Function with no branches yields empty CFG
- **WHEN** extracting a function with only sequential statements and one return
- **THEN** the extractor returns a CFG with no `decision` nodes
- **AND** callers MAY treat this as "no flowchart available"

#### Scenario: Switch produces case branches
- **WHEN** the function contains a `switch` with three `case`s and a `default`
- **THEN** the CFG has one `decision` node with four outgoing edges labeled `case:<value>` and `default`

### Requirement: Early-exit and terminal recognition
The extractor SHALL treat `return`, `throw`, and the Express `res.status(N).json(...)` / `res.status(N).send(...)` followed by `return` idiom as `terminal` nodes labeled with the outcome (e.g. status code), ending that path.

#### Scenario: res.json() + return collapses to one terminal
- **WHEN** a branch body is `res.status(402).json({error: "insufficient"}); return;`
- **THEN** the CFG contains a single `terminal` node labeled with `402`
- **AND** no `step` node is emitted for the `return`

### Requirement: Batch extraction per file
The extractor SHALL process ALL functions in a file in a single parse pass, returning `[]CFG`. This avoids re-parsing the file per function.

#### Scenario: File with 5 functions returns 5 CFGs
- **WHEN** a JS/TS file contains 5 function declarations
- **THEN** `ExtractCFGs` returns a slice of 5 `CFG` values
- **AND** the file is parsed exactly once

### Requirement: Location-based keying
CFGs SHALL be keyed by `(workspace_hash, source_file, start_line, end_line)` — NOT by symbol name. This supports anonymous functions which have no symbol.

#### Scenario: Anonymous function is keyed by location
- **WHEN** a handler is an anonymous arrow function `(req, res) => { ... }`
- **THEN** the CFG is stored with `source_file`, `start_line`, `end_line` from the function's source position
- **AND** no `symbol` field is required

### Requirement: Minified-file and language guard
The extractor SHALL be skipped for files identified as minified, and SHALL only run for supported languages (JS/TS in Phase 1b).

#### Scenario: Minified file is skipped
- **WHEN** the watcher processes a file for which `isMinified` returns true
- **THEN** no CFG extraction is attempted for that file

### Requirement: CFG size limit
The extractor SHALL cap CFGs at 500 nodes. If exceeded, truncate and set `status: "truncated"`.

#### Scenario: Large switch statement is truncated
- **WHEN** a function contains a switch with 100+ cases producing >500 nodes
- **THEN** the CFG is truncated at 500 nodes
- **AND** `status` is set to `"truncated"`

### Requirement: Watcher-driven storage lifecycle
CFGs SHALL be persisted per `(workspace_hash, source_file, start_line, end_line)` and refreshed when the source file changes: existing CFGs for a file are deleted before the file's functions are re-extracted.

#### Scenario: Re-indexing a changed file refreshes its CFGs
- **WHEN** a JS/TS file is re-processed by the watcher
- **THEN** prior `function_flowcharts` rows for that file are deleted
- **AND** CFGs for the file's current functions are upserted
