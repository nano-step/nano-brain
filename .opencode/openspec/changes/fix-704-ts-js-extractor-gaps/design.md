# Design

## Shared JS/TS/Vue Initializer & Ranking Helpers (`internal/symbol`)

`TypeScriptExtractor`, `JavaScriptExtractor`, and `VueExtractor` share the same
ECMAScript/TypeScript AST semantics in `gotreesitter`:

1. **Candidate Ranking on `kind:name` (`symbolDeclRank`):**
   Instead of `seen := map[string]bool{}`, each extractor tracks the slice index
   and rank of the best symbol emitted so far for `key = string(kind) + ":" + name`:
   - **Rank 4 (top-level declaration / top-level class member):**
     - Module-scope `function_declaration`, `generator_function_declaration`,
       `lexical_declaration`, `variable_declaration`, `class_declaration`,
       `interface_declaration`, `type_alias_declaration`, `enum_declaration`
       whose ancestor chain to `program` contains only optional `export_statement`.
     - `method_definition`, `public_field_definition`, or `field_definition`
       directly inside the `class_body` of a module-scope `class_declaration`.
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
     `instantiation_expression` (expression at `NamedChild(0)`), and
     `type_assertion` (expression at `NamedChild(NamedChildCount()-1)`).
   - Direct `arrow_function`, `function_expression`, and `generator_function`
     initializers return `true`.
   - `call_expression` initializers recurse up to depth 4 (`memo(forwardRef(...))`)
     when the callee is a HOF (`identifier` excluding timer/scheduler APIs like
     `setTimeout`/`setInterval`, or `identifier.property` `member_expression`
     excluding eager collection/promise/string/JSON receiver methods such as
     `map`, `flatMap`, `filter`, `reduce`, `find`, `some`, `every`, `forEach`,
     `sort`, `then`, `catch`, `finally`, `from`, `replace`, `parse`, `stringify`)
     and an unwrapped argument is a function or nested HOF call.

3. **Class-Field Arrows (`KindMethod`):**
   - TS/TSX query matches `(public_field_definition name: [(property_identifier) (private_property_identifier)] @name value: (_) @val) @decl`.
   - JS query matches `(field_definition property: [(property_identifier) (private_property_identifier)] @name value: (_) @val) @decl`.
   - Only fields passing `isFunctionInitializer` are emitted, mapped to `KindMethod`
     alongside `method_definition` and object-literal `pair`.
