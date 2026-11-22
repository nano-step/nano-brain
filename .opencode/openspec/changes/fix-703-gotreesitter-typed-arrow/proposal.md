# Bump gotreesitter to v0.57.0 for TypeScript typed-arrow parsing (#703)

## Why

`github.com/odvcencio/gotreesitter` v0.19.1 misparses any non-async
TypeScript/TSX arrow function whose first parameter carries a type
annotation — `(p: P) => 1`, `(p: string): number =>`, `({ a }: P) =>`,
or an inline callback such as `items.map((x: Item) => x.id)` — emitting an
`ERROR` root node instead of a valid `program` tree. Because symbol
extraction, knowledge-graph extraction, control-flow extraction, and
symbol-aware chunking all query the parsed syntax tree, affected `.ts`,
`.tsx`, and `.vue` `<script setup lang="ts">` files lose both the typed
arrow declarations themselves and enclosing declarations corrupted by an
inner typed-arrow callback. In addition, v0.19.1 hits early
`memory_budget` stops on multi-thousand-line `.js` files, dropping tail
symbols.

## What

- Upgrade `github.com/odvcencio/gotreesitter` from `v0.19.1` to `v0.57.0`
  (>7 days old per supply-chain cooldown policy).
- Add regression tests across `internal/symbol`, `internal/graph`
  (`TypeScriptGraphExtractor`, `VueSFCExtractor`,
  `JSControlFlowExtractor`), and `internal/chunker` (`SymbolAwareChunker`)
  covering non-async typed-first-parameter arrows in `.ts`, `.tsx`, and
  `.vue`, outer async functions containing inner typed `.map(...)`
  callbacks, and 5000-line `.js` files.
- Document recovery for already-indexed workspaces (`POST /api/v1/update`
  for symbols, graph edges, and CFGs without re-embedding; reset + reindex
  if parent-file raw chunks need re-chunking).
