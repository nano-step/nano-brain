# Source-Scoped Call Resolution Contract Matrix

Status: Final contract and implementation evidence for #609. The historical
red-test notes below preserve the pre-implementation boundary; the focused
fixtures and reader controls now pass against the implementation.

## Persisted edge contract

| Item | Decision |
| --- | --- |
| Contextual call target | Emit exactly one canonical target only when the target declaration is provably source-reachable and has the identical emitted declaration identity. |
| Unknown result | Emit target-only `<unresolved>`. No `reason`, `raw_callee`, or equivalent metadata is persisted, queried, or mapped to REST/MCP. |
| Canonical predicate | `IsCanonicalJSCallTarget(string) bool` follows the test oracle below and does not classify a historical Ruby namespace as canonical. |
| Legacy control | `Api::V2::StoriesController#sync` remains a legacy target. Its `::` is Ruby namespace syntax, not a JS/TS path delimiter. |
| Legacy extractor | `ExtractEdges` remains non-contextual and emits bare call targets such as `run`; only the contextual route changes. |
| Resolver scope | Per contextual extraction invocation only. A direct-export catalog parses each target module at most once. No shared cross-run cache. |

The canonical predicate is equivalent to:

```text
relative/path.(js|jsx|mjs|cjs|ts|tsx|mts|cts)::
  symbol | Class.method | default | default.method
```

Paths are relative and normalized (no absolute or `..` segment); the symbol
part has one or two JavaScript identifier components. Thus bare names,
`<unresolved>`, Ruby namespaces, unsupported extensions, and three-part
symbols are not canonical. The executable test oracle is
`TestCanonicalJSCallTargetContract`; production implementation is deliberately
left to Task 2.1.

## Parser feasibility

`TestSourceScopedCallResolutionParserFeasibility` parses representative source
with the exact vendored `gotreesitter` grammars. It asserts the listed node
kinds and these available fields: `import_statement.source`,
`class_declaration.name`, `method_definition.name`, `method_definition.body`,
and `call_expression.function`.

| Fact | JavaScript grammar evidence | TypeScript grammar evidence | Task 2.1 decision |
| --- | --- | --- | --- |
| Direct import binding | `import_statement`, `import_clause`, `import_specifier`, `namespace_import`, `identifier` | Same node families | Support named aliases, default binding, and namespace binding only when `ImportContext` resolves one project file and the target catalog finds a direct export. |
| Function/class/method ownership | `function_declaration`, `class_declaration`, `class_body`, `method_definition`, `property_identifier` | Same node families | Support declarations whose emitted identity is `file::symbol` or `file::Class.method`; class methods require their own stable declaration identity. |
| Calls and members | `call_expression` with `function`; `member_expression` with `property_identifier` | Same node families | Support direct identifier calls, permitted import members, class methods, and the two exact typed `this.field.method()` forms below. Computed/optional/chained/dynamic calls are unresolved. |
| Parameters | `formal_parameters` with `identifier`; JavaScript does **not** expose `required_parameter` | `formal_parameters`, `required_parameter`, `type_annotation`, `type_identifier` | Model JS parameter identifiers and TS required parameters as bindings. Parameter shadows are unresolved unless a supported declaration is independently proven. |
| Blocks and catch | `statement_block`, `lexical_declaration`, `catch_clause` | Same lexical/catch structure where present | Use innermost lexical scope; block and catch bindings shadow imports. |
| JavaScript `var` | `variable_declaration`, `variable_declarator` | Not a TS receiver feature | Model function/module binding and hoisting. A bare or assignment-derived `var` callable is not a supported emitted declaration in this change, so ambiguous/unsupported calls are `<unresolved>`. |
| Function hoisting | `function_declaration` | `function_declaration` | A call to an unshadowed same-file function declaration may resolve even when the declaration appears later. |
| Nested functions | `function_declaration` nested in `statement_block` | Same | Nesting creates a new function boundary; do not leak parameter/local bindings across it. |
| Typed class property | Not applicable | `public_field_definition`, `type_annotation`, simple `type_identifier` | Support only `this.field.method()` with a simple named same-file or direct-import class type. |
| Constructor parameter property | Not applicable | `required_parameter` with `accessibility_modifier`, `type_annotation`, simple `type_identifier` | Support only `this.field.method()` where the constructor parameter has an access modifier and a simple named type. |

The parser can expose syntax facts; it cannot prove runtime aliasing, inferred
types, re-export resolution, CommonJS semantics, dynamic property names, or
framework DI. Those forms must produce target-only `<unresolved>` rather than
a guessed target.

## Direct-export catalog and imports

| Import/export case | Required outcome |
| --- | --- |
| Relative or configured alias specifier resolving through `ImportContext` to one admitted workspace file | Eligible for catalog lookup, including the existing extension/index resolution behavior. |
| `export function`, `export class`, `export const`, or direct default export in that exact file | Eligible only when the call shape selects that exact emitted declaration identity. |
| Named alias import | Resolve binding to the exported declaration, not the local alias spelling. |
| Default import | Resolve to `file::default` or `file::default.method`. |
| Namespace import | Resolve a direct member only; never search globally by member name. |
| `export {local}`, barrel, re-export chain, package, missing file, ambiguous file | `<unresolved>`. |
| Exporter rename/removal | Re-extraction replaces the importer call target with `<unresolved>`; no stale canonical edge remains. |

No CommonJS resolution, re-export traversal, global type checker, guessed DI,
or directory-proximity lookup is admitted.

## Focused fixture matrix

The “Current result before Task 2.1” column is retained as historical intake
evidence. The implementation now exercises the expected target column, while
the plain extractor controls continue to prove legacy bare-call compatibility.

| Test case | Expected target | Current result before Task 2.1 | Contract |
| --- | --- | --- | --- |
| `same-root-collision-named-alias` | `repo-a/lib/api.ts::run` | bare `run` | Same-named `repo-b/lib/api.ts::run` is never eligible. |
| `direct-default-method` | `repo-a/lib/default.ts::default.run` | bare `run` | Direct default class method only. |
| `direct-const-export` | `repo-a/lib/api.ts::callback` | bare `callback` | Direct `export const` identity only. |
| `namespace-direct-export` | `repo-a/lib/api.ts::run` | bare `run` | Namespace member must name a direct export. |
| `named-class-method` | `repo-a/lib/api.ts::Api.run` | bare `run` | Class method identity is source-scoped. |
| `external-package-is-unresolved` | `<unresolved>` | bare `run` | Packages cannot become project declarations. |
| `missing-project-import-is-unresolved` | `<unresolved>` | bare `run` | A missing target module is not guessed. |
| `same-file-typed-class-property` | `repo-a/consumer.ts::Api.run` | bare `run` | Exact supported TS receiver form. |
| `access-modified-constructor-parameter-property` | `repo-a/consumer.ts::Api.run` | bare `run` | Exact supported TS receiver form. |
| `function-declaration-hoisting` | `repo-a/consumer.ts::run` | bare `run` | Unshadowed same-file declaration is hoisted. |
| `parameter-shadow-is-unresolved` | `<unresolved>` | bare `run` | Parameter blocks import binding. |
| `block-let-shadow-is-unresolved` | `<unresolved>` | bare `run` | Inner lexical binding wins. |
| `catch-binding-shadow-is-unresolved` | `<unresolved>` | bare `run` | Catch binding wins. |
| `var-binding-is-unresolved` | `<unresolved>` | bare `run` | Hoisted binding without supported declaration identity is not guessed. |
| `nested-function-boundary-is-unresolved` | `<unresolved>` | bare `run` | Nested parameter scope does not leak the outer import. |
| `receiver-alias-is-unresolved` | `<unresolved>` | bare `run` | `const receiver = this.field` is outside admitted receiver forms. |
| `computed-member-is-unresolved` | `<unresolved>` | no call edge | Dynamic property syntax still records only the sentinel. |
| `TestTypeScriptContextualCallExporterChange` before removal | `repo-a/lib/api.ts::run` | bare `run` | Direct-export catalog creates the exact edge. |
| `TestTypeScriptContextualCallExporterChange` after removal | `<unresolved>` | bare `run` | Lifecycle invalidates stale importer edge. |
| `TestPlainTypeScriptExtractorKeepsBareCallTargets` | bare `run` | bare `run` | Green compatibility control. |
| `TestPlainJavaScriptExtractorKeepsBareCallTargets` | bare `run` | bare `run` | Green compatibility control. |

### JavaScript contextual additions

Each positive call fixture invokes `requireJavaScriptDeclarationIdentity` after
its target assertion, so a green resolver test also proves that the exact
canonical target is emitted as a declaration identity by the target module.

| Test case | Expected target | Current result before Task 2.1 | Contract |
| --- | --- | --- | --- |
| `named-direct-export` | `repo-a/lib/api.js::run` | bare `execute` | Named import resolves to its direct exported declaration, not its local alias. |
| `default-class-method` | `repo-a/lib/default.js::default.run` | bare `run` | Default import can select only the direct default class method. |
| `namespace-direct-export` | `repo-a/lib/api.js::run` | bare `run` | Namespace member selects only a direct export. |
| `named-class-method` | `repo-a/lib/api.js::Api.run` | bare `run` | Imported class method has a source-scoped declaration identity. |
| `parameter-shadow-is-unresolved` | `<unresolved>` | bare `run` | A JavaScript parameter shadows the direct import. |
| `computed-member-is-unresolved` | `<unresolved>` | no call edge | Dynamic property access is never guessed. |
| `TestJavaScriptContextualCallDeclarationIdentities` | matching `run`, `Api.run`, `default`, and `default.run` containment identities | class/default identities missing | Every canonical JavaScript call target has the identical emitted declaration identity. |

### Excluded TypeScript receivers

| Test case | Expected target | Current result before Task 2.1 | Contract |
| --- | --- | --- | --- |
| `plain-constructor-parameter-is-unresolved` | `<unresolved>` | bare `run` | A constructor parameter without an access modifier is not a property. |
| `interface-typed-property-is-unresolved` | `<unresolved>` | bare `run` | Interface types do not prove a concrete class target. |
| `union-typed-property-is-unresolved` | `<unresolved>` | bare `run` | Union types are ambiguous. |
| `generic-typed-property-is-unresolved` | `<unresolved>` | bare `run` | Generic instantiation is outside the simple named-type contract. |
| `any-typed-property-is-unresolved` | `<unresolved>` | bare `run` | `any` does not provide a target identity. |
| `inferred-property-is-unresolved` | `<unresolved>` | bare `run` | Inferred property type is outside the parser-only contract. |

Every non-sentinel target above is also the declaration identity that the
contextual extractor must emit for the corresponding direct declaration. Task
2.1 owns the shared catalog/identity proof; Task 2.2/2.3 owns the JS/TS
emission wiring.

## Implementation handoff

The implementation adds the production `IsCanonicalJSCallTarget` predicate,
builds a one-invocation direct-export catalog, and resolves only the matrix's
admitted bindings and scope facts. The JS/TS contextual paths preserve the
plain extractors, while watcher invalidation and reader safety enforce the
required target transition.

## Historical Task 1.2 verification

| Command | Result | Interpretation |
| --- | --- | --- |
| `go test ./internal/graph -run 'Test.*(Contextual|Call|Import|Scope|Receiver|Unresolved)' -count=1` | Intentionally red | All 34 contract cases fail on current bare/omitted call targets or missing canonical declaration identities, proving the source-scoped resolver is absent without a compile or fixture failure. |
| `go test ./internal/graph -run 'Test(SourceScopedCallResolutionParserFeasibility|CanonicalJSCallTargetContract|ContextualCallTargetsRemainMetadataFree|Plain.*ExtractorKeepsBareCallTargets)$' -count=1 -v` | Green | The parser node/field feasibility assertions, canonical predicate oracle (including Ruby legacy control), target-only metadata control, and legacy plain JS/TS bare-call controls pass. |

The full raw command evidence is in
`.omo/evidence/609-source-scoped-call-resolution/task-2-609-source-scoped-call-resolution.md`.
