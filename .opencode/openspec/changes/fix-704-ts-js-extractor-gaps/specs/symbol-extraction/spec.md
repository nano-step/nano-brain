# Symbol Extraction

## ADDED Requirements

### Requirement: Top-level declarations take precedence over nested and member-assigned duplicates
The TypeScript, JavaScript, and Vue symbol extractors SHALL rank symbol
candidates sharing the same `kind:name` key so exported module-scope
declarations and exported module-scope class members (including class
expressions assigned at module scope and TypeScript `namespace` declarations)
outrank unexported module-scope declarations, which in turn replace earlier
nested declarations, object-literal members, or member assignments
(`this.foo = ...`, `exports.foo = ...`), while keeping standalone
member-assigned exports when no top-level declaration exists.

#### Scenario: Top-level function replaces earlier nested or member-assigned function
- **WHEN** a `.ts`, `.tsx`, `.js`, or `.vue` file (including multi-script `.vue` SFCs and TypeScript `namespace` blocks) defines a nested `const foo = () => 1`, an unexported `class noopAdapter { handle() {} }`, or `{ handle: () => 0 }` / `this.foo = () => 1` / `exports.foo = () => 2` before an exported module-scope declaration, `export class OrderController`, or `export const RealController = class { ... }`
- **THEN** the extracted symbol carries the higher-ranked declaration's full `Line..EndLine` span and signature, and any standalone `module.exports.bar = ...` without a top-level counterpart is still extracted

### Requirement: Type-wrapped, parenthesized, and HOF-wrapped function initializers are extracted without collection-callback false positives
The TypeScript, JavaScript, and Vue symbol extractors SHALL extract `const`,
`let`, and `var` declarators initialized with functions wrapped in `satisfies`,
`as`, type assertions, parentheses (including inline comments), or higher-order
function wrappers (including nested, curried, and builder-chained wrappers such
as `memo(forwardRef(...))`, `React.memo(...)`, `connect(mapState)(...)`,
`create<T>()(...)`, `protectedProcedure.input(...).query(...)`, `defineStore`,
and `computed`), while scoping HOF call wrapping to variable/lexical
declarations and excluding eager collection, promise, timer, event-registration,
grouping, and test-runner callbacks (`items.map(...)`, `nums.reduce(...)`,
`window.setTimeout(...)`, `bus.on(...)`, `Object.groupBy(...)`, `describe(...)`).

#### Scenario: Wrapped function initializers emit symbols while collection and registration callbacks are ignored
- **WHEN** symbols are extracted from `.ts`, `.tsx`, `.js`, or `.vue` files containing `satisfies Handler` (with comments), `as Handler`, parenthesized arrows, `memo(forwardRef(...))`, curried/builder HOFs, `defineStore`/`computed`, multi-declarator `const TIMEOUT = 5000,\n  fetchWithRetry = ...`, and noise calls (`items.map(...)`, `window.setTimeout(...)`, `bus.on(...)`, `{ listener: bus.on(...) }`, `Object.groupBy(...)`, `describe(...)`)
- **THEN** each wrapped function declarator is emitted as `KindFunction` with its own `Signature` and full `Line..EndLine` span, and none of the collection, timer, event-listener, or test-suite callbacks are emitted

### Requirement: Class-field arrow and function expressions are extracted as methods
The TypeScript, JavaScript, and Vue symbol extractors SHALL extract class fields
(`public_field_definition` in TS/TSX/Vue and `field_definition` in JS/JSX)
whose initializers resolve directly (after type/parenthesis unwrapping) to
functions as `KindMethod` symbols with their full `Line..EndLine` spans and
field-line signatures (even when preceded by multiline `@decorator` lines),
while ignoring scalar or non-function class fields.

#### Scenario: Class-field arrows emit KindMethod symbols across TS, TSX, JS, JSX, and Vue
- **WHEN** symbols are extracted from a class containing arrow/function-expression fields (`handle = async (...) => { ... }`, `private onClick = (e: Event) => { ... }`, `@bound\n  onSubmit = async (...) => { ... }`) alongside scalar or timer fields (`scalar = 42`, `timer = setTimeout(...)`)
- **THEN** `handle`, `onClick`, and `onSubmit` are emitted with `KindMethod`, signatures containing the field name, and full `Line..EndLine` spans, and `scalar` and `timer` are not emitted
