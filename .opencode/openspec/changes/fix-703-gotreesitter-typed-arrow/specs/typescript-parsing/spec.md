# TypeScript Parsing

## ADDED Requirements

### Requirement: Non-async typed-parameter arrow function extraction
The symbol, graph, CFG, and chunking pipelines SHALL parse non-async
TypeScript and TSX arrow functions whose first parameter carries a type
annotation — in `.ts`, `.tsx`, and `.vue` `<script setup lang="ts">` files
— without collapsing the syntax tree into an `ERROR` root.

#### Scenario: Direct typed-first-parameter arrows emit symbols and chunks
Given `.ts` and `.tsx` files declaring non-async arrow functions with a
typed first parameter (identifier, multiple parameters, destructured
object parameter, JSX body, or multi-line block body),
when symbols and chunks are extracted,
then `symbol.Registry` emits each function symbol with its full
`Line..EndLine` span and `SymbolAwareChunker` emits a `symbol` chunk
covering the full declaration span rather than falling back to `raw`.

#### Scenario: Inner typed arrow callback preserves enclosing declarations
Given a `.tsx` or `.vue` `<script setup lang="ts">` file where an outer
`async` arrow function contains an inner `.map((x: Item) => x.id)`
callback,
when symbols are extracted,
then the outer function symbol is emitted with its complete multi-line
`Line..EndLine` span.

#### Scenario: Graph edges and CFGs extracted for typed-parameter arrows
Given `.ts`, `.tsx`, and `.vue` `<script setup lang="ts">` files declaring
`export const handler = (req: Request): Response => { return helper(req); }`
and a multi-branch non-async typed arrow in `.ts`/`.tsx`,
when `TypeScriptGraphExtractor`, `VueSFCExtractor`, and
`JSControlFlowExtractor` run,
then `contains` and `calls` edges (`file -> file::handler` and
`file::handler -> helper`) and a complete CFG with decision and terminal
nodes are emitted.

### Requirement: Large JavaScript files complete without early budget truncation
JavaScript symbol extraction SHALL parse multi-thousand-line `.js` files
without prematurely dropping declarations near the end of the file due to
parser memory-budget truncation.

#### Scenario: Symbols past line 4900 in a 5000-line JavaScript file
Given a 5000-line `.js` module declaring exported arrow functions through
the end of the file,
when `symbol.Registry` extracts symbols,
then functions declared past line 4900 are emitted.
