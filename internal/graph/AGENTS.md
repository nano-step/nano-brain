# internal/graph — AGENTS.md

Code-intelligence graph layer: framework route extraction, outbound integration
edges, and per-function control-flow graphs (CFGs). Built on the vendored
`gotreesitter` library (Go bindings over tree-sitter grammars).

## Two extraction families

There are two distinct extractor interfaces, both served by one `Registry`:

- **`Extractor`** (edge extraction) — `Supports(ext) bool` + `ExtractEdges(path, content) ([]Edge, error)`.
  Produces `contains`/`imports`/`calls`/`http`/`middleware`/`integration` edges
  stored in `graph_edges`. Implementations: Go/TS/JS/Python call-graph and
  symbol extractors, framework route extractors (Echo, Gin, net/http, Express,
  NestJS, Nuxt, Rails), and integration extractors.
- **`ControlFlowExtractor`** (CFG extraction) — `SupportsCFG(ext) bool` +
  `ExtractCFGs(path, content) ([]CFG, error)`. Produces one `CFG` per function
  (start/step/decision/terminal/merge nodes + branch-labeled edges) stored in
  `function_flowcharts`. Implementation: `JSControlFlowExtractor` (JS/TS only).

## CFG types (`cflow.go`)

- `CFGNode{ID, Type, Label, Line, Kind, Call}` — `Type` is one of
  `start|step|decision|terminal|merge`; `Kind` is `error|return` for terminals;
  `Call` is the target symbol for call steps.
- `CFGEdge{From, To, Branch}` — `Branch` is `yes|no|case:<v>|default|loop|next`.
- `CFG{Entry, SourceFile, StartLine, EndLine, Nodes, Edges, Status}` — `Status`
  is `complete|truncated|parse_error|unsupported`. `Entry` is `relfile::funcName`.

## JS/TS CFG extractor (`js_cflow.go`)

`ExtractCFGs` parses the whole file once, finds `function_declaration`,
`method_definition`, and variable-assigned `arrow_function` nodes, then walks
each body via `cfgbuilder.buildBlock` (recursive descent). Handles if/else,
switch, loops, return/throw/early-exit terminals, and try/catch. A 500-node cap
(`maxCFGNodes`) sets `Status = "truncated"`. Functions whose body yields only the
start node produce no CFG.

### Known limitations / TODO

- **Switch cases not expanded:** `buildSwitch` matches child node types `"case"`
  and `"default"`, but the tree-sitter-javascript grammar emits `switch_case` /
  `switch_default`. As a result only the `switch (...)` decision node is emitted
  today; individual case branches/terminals are not. Fix by matching the correct
  node type names before relying on `case:`-labeled edges.
- **Loops are summarized** as a single `step` node (no back-edge body yet).
- **try/catch** is a single `step` node, not separate try/catch/finally blocks.
- Line numbers come from `lineForByte(content, n.StartByte())` (defined in
  `go_extractor.go`). `gotreesitter.Node` has **no** `StartLine`/`EndLine` —
  always derive lines from byte offsets.

### Fixed (v2)

- **Junk nodes:** `buildBlock` now skips punctuation tokens (`{`, `}`, `;`) and
  comment nodes (`comment`, `line_comment`, `block_comment`) via
  `isIgnoredStatement()`. CFGs no longer contain step nodes for non-statements.
- **Absolute paths:** `ExtractCFGs` strips absolute paths to the basename, and
  the start node label is now the function name (not the full `file::func`
  entry). The `Entry` field still stores `relfile::funcName` for lookup.
- **Wrapped handlers:** `findAssignedName()` walks up the AST from an
  `arrow_function` to find the nearest `variable_declarator` ancestor, enabling
  extraction of wrapped idioms like `const fn = catchAsync(async () => {...})`.

## Registry (`registry.go`)

`NewRegistry(extractors...)` holds edge extractors; CFG extractors are added
separately via `RegisterControlFlowExtractor`. `ExtractCFGs` runs every
registered CFG extractor that supports the extension (skipping minified files via
`isMinified`). `HasControlFlowExtractors` lets callers (the watcher) skip CFG work
when none are wired. CFG extraction is gated on `cfg.Flow.Enabled` in `main.go`.

## Storage flow

The watcher's `extractAndUpsertCFGs` (in `internal/watcher`) calls
`Registry.ExtractCFGs`, deletes prior rows for the file
(`DeleteFunctionFlowchartsByFile`), and upserts each CFG
(`UpsertFunctionFlowchart`) with the full CFG marshaled into the `cfg` JSONB
column. Served by `POST /api/v1/graph/flowchart` and the `memory_flowchart` MCP
tool.
