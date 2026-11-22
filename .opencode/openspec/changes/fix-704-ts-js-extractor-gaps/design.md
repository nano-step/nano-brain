# Design

## Shared JS/TS/Vue Initializer & Ranking Helpers (`internal/symbol`)

`TypeScriptExtractor`, `JavaScriptExtractor`, and `VueExtractor` share the same
ECMAScript/TypeScript AST semantics in `gotreesitter`:

1. **Candidate Ranking on `kind:name` (`symbolDeclRank`):**
   Instead of `seen := map[string]bool{}`, each extractor tracks the slice index
   and rank of the best symbol emitted so far for `key = string(kind) + ":" + name`:
   - **Rank 5 (exported module-scope declaration / member of exported module-scope class):**
     - Exported module-scope (including exported TypeScript `namespace`/`internal_module`)
       `function_declaration`, `generator_function_declaration`,
       `lexical_declaration`, `variable_declaration`, `class_declaration`,
       `interface_declaration`, `type_alias_declaration`, `enum_declaration`.
     - `method_definition`, `public_field_definition`, or `field_definition`
       directly inside the `class_body` of an exported module-scope class
       (`export class ...`, `export default class ...`, or
       `export const X = class { ... }`).
   - **Rank 4 (unexported module-scope declaration / member of unexported module-scope class):**
     - Unexported module-scope or TypeScript `namespace`-scoped declarations and
       members of unexported module-scope class declarations or class expressions
       (`const X = class { ... }`, `module.exports = class { ... }`).
   - **Rank 3 (top-level member assignment or object-literal member):**
     - Unnested `assignment_expression` (`exports.foo = ...`, `module.exports.foo = ...`)
       and unnested object-literal `pair` / `method_definition`.
   - **Rank 2 (nested declaration / nested class member):**
     - Declarations or class members enclosed inside a function, generator,
       arrow function, or method body.
   - **Rank 1 (nested assignment / nested object-literal member):**
     - `this.foo = () => 1` inside a constructor/method or `pair` inside a
       function-local object literal.
   When a candidate for an existing `key` arrives with `rank > prevRank`, it
   replaces `symbols[idx]` in-place; equal or lower ranks keep the earlier
   entry so output order and count remain deterministic, including across
   multiple `<script>` + `<script setup>` injections in `VueExtractor`.

2. **Function Initializer Unwrapping & HOF Detection (`isFunctionInitializer`):**
   - `unwrapJSExpr` iteratively peels `parenthesized_expression`,
     `satisfies_expression`, `as_expression`, `non_null_expression`,
     `instantiation_expression` (first non-comment named child), and
     `type_assertion` (last non-comment named child), skipping `comment` and
     `html_comment` extras.
   - Direct `arrow_function`, `function_expression`, `function`, and
     `generator_function` initializers return `true` for all value-bearing
     captures (`lexical_declaration`, `variable_declaration`, `pair`,
     `assignment_expression`, `public_field_definition`, `field_definition`).
   - `call_expression` HOF wrapping applies only to `lexical_declaration` and
     `variable_declaration` (up to depth 4, e.g. `memo(forwardRef(...))`) so
     `pair`, `assignment_expression`, and class fields never treat registration
     or timer calls as methods/functions.
   - `isHOFCallee` walks up to 8 steps across `identifier`, `member_expression`,
     and `call_expression` nodes (supporting curried HOFs like
     `connect(mapState)(fn)`, `create<T>()(fn)` and builder chains like
     `protectedProcedure.input(s).query(fn)`) rooted at an `identifier`, while
     rejecting a unified blocklist on both `identifier` and `property_identifier`
     names (timers/schedulers `setTimeout`, `setInterval`, etc., eager
     collection/promise/string/JSON/grouping helpers, event/server registration
     methods `on`, `once`, `off`, `listen`, `subscribe`, and test DSL
     `describe`, `it`, `test`, while preserving `computed` and `defineStore`).

3. **Class-Field Arrows & Multi-Line / Multi-Declarator Signatures:**
   - TS/TSX query matches `(public_field_definition name: [(property_identifier) (private_property_identifier)] @name value: (_) @val) @decl`.
   - JS query matches `(field_definition property: [(property_identifier) (private_property_identifier)] @name value: (_) @val) @decl`.
   - Only fields passing `isFunctionInitializer` (`allowHOF = false`) are
     emitted, mapped to `KindMethod` alongside `method_definition` and
     object-literal `pair`.
   - Multi-declarator `lexical_declaration` / `variable_declaration` statements
     compute `EndLine` and `Signature` from the individual `variable_declarator`,
     and declarations whose `declNode` starts on an earlier line than `nameNode`
     (such as `@bound\nonSubmit = ...`) extract the trimmed line containing
     `nameNode` rather than emitting `"@bound"` or an earlier declarator.
