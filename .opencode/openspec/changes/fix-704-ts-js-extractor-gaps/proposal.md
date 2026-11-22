# Close TypeScript, JavaScript, and Vue Symbol Extractor Gaps (#704)

## Why

After the `gotreesitter` `v0.57.0` upgrade (#703/#708) resolved TypeScript
typed-arrow parse errors and 5000-line JavaScript `memory_budget` stops, four
remaining symbol extraction gaps in `internal/symbol` still drop or truncate
symbols across `.ts`, `.tsx`, `.mts`, `.cts`, `.js`, `.mjs`, `.cjs`, `.jsx`,
and `.vue` files:

1. **First-match `kind:name` deduplication shadows top-level declarations:**
   `seen := map[string]bool{}` keeps whichever match appears first in AST order.
   An earlier nested `const foo = () => 1` inside another function (or inside an
   earlier `<script>` block of a multi-script `.vue` SFC) or an earlier
   `this.foo = () => 1` / `exports.foo = () => 1` assignment shadows a later
   top-level `export const foo = async (...) => { ... }`, leaving `foo` with a
   1-line span (`Line == EndLine`).
2. **`satisfies`, `as`, and parenthesized function initializers are missed:**
   `(async () => {}) satisfies Handler`, `((req: Req): Res => ...) as Handler`,
   and parenthesized arrows `((x: number) => x + 1)` wrap the function node in
   `satisfies_expression`, `as_expression`, `type_assertion`,
   `non_null_expression`, or `parenthesized_expression`, which the static query
   enumerations did not match.
3. **Nested HOF wrappers and `var` HOFs are missed while collection callbacks
   false-positive:** `memo(forwardRef(...))`, `React.memo(React.forwardRef(...))`,
   and `var legacy = debounce(function(x) { ... })` yielded zero symbols, whereas
   the unanchored `(call_expression arguments: (arguments [(arrow_function) ...]))`
   pattern misclassified eager collection calls like `const ids = items.map(...)`
   and `const total = nums.reduce(...)` as `KindFunction` symbols.
4. **Class-field arrows and function expressions are missed:** `public_field_definition`
   (TS/TSX) and `field_definition` (JS) members initialized to arrow or function
   expressions (`handle = async (req: Req): Promise<Res> => { ... }`) were not
   matched.

## What

- Rank symbol candidates per `kind:name` key so top-level module declarations
  and top-level class members replace nested declarations or member assignments
  in-place while keeping first-seen order among equal-rank candidates.
- Unwrap type assertions/satisfies/parentheses and nested HOF wrapper calls on
  function initializers across TS, TSX, JS, JSX, and Vue while excluding eager
  collection/promise/timer callbacks (`map`, `reduce`, `filter`, `then`, etc.).
- Extract class-field arrows and function expressions (`public_field_definition`
  in TS/TSX/Vue and `field_definition` in JS/JSX) as `KindMethod` symbols.
- Document `POST /api/v1/update` in `CHANGELOG.md` for backfilling symbol spans
  on already-indexed workspaces.
