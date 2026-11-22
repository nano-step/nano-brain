# Symbol Extraction

## ADDED Requirements

### Requirement: Top-level declarations take precedence over nested and member-assigned duplicates
The TypeScript, JavaScript, and Vue symbol extractors SHALL rank symbol
candidates sharing the same `kind:name` key so module-scope declarations and
top-level class members replace earlier nested declarations or member
assignments (`this.foo = ...`, `exports.foo = ...`) while keeping standalone
member-assigned exports when no top-level declaration exists.

#### Scenario: Top-level function replaces earlier nested or member-assigned function
- **WHEN** a `.ts`, `.tsx`, `.js`, or `.vue` file (including multi-script `.vue` SFCs) defines a nested `const foo = () => 1` or `this.foo = () => 1` / `exports.foo = () => 2` before a multi-line top-level `export const foo = ...` or `export function foo(...)`
- **THEN** the extracted `KindFunction` symbol `foo` carries the top-level declaration's full `Line..EndLine` span, and any standalone `module.exports.bar = ...` without a top-level counterpart is still extracted

### Requirement: Type-wrapped, parenthesized, and HOF-wrapped function initializers are extracted without collection-callback false positives
The TypeScript, JavaScript, and Vue symbol extractors SHALL extract `const`,
`let`, and `var` declarators initialized with functions wrapped in `satisfies`,
`as`, type assertions, parentheses, or higher-order function wrappers (including
nested wrappers such as `memo(forwardRef(...))` and `React.memo(...)`), and
SHALL NOT classify eager collection or promise callbacks (`items.map(...)`,
`nums.reduce(...)`) as function symbols.

#### Scenario: Wrapped function initializers emit symbols while collection callbacks are ignored
- **WHEN** symbols are extracted from `.ts`, `.tsx`, `.js`, or `.vue` files containing `satisfies Handler`, `as Handler`, parenthesized arrows, `memo(forwardRef(...))`, `var legacy = debounce(function(...) {})`, and `const ids = items.map(...)` / `const total = nums.reduce(...)`
- **THEN** each wrapped function declarator is emitted as `KindFunction` with its full `Line..EndLine` span and `ids` / `total` are not emitted

### Requirement: Class-field arrow and function expressions are extracted as methods
The TypeScript, JavaScript, and Vue symbol extractors SHALL extract class fields
(`public_field_definition` in TS/TSX/Vue and `field_definition` in JS/JSX)
whose initializers resolve to functions as `KindMethod` symbols with their full
`Line..EndLine` spans, while ignoring scalar or non-function class fields.

#### Scenario: Class-field arrows emit KindMethod symbols across TS, TSX, JS, JSX, and Vue
- **WHEN** symbols are extracted from a class containing arrow/function-expression fields (`handle = async (...) => { ... }`, `private onClick = (e: Event) => { ... }`) alongside scalar fields (`scalar = 42`)
- **THEN** `handle` and `onClick` are emitted with `KindMethod` and full `Line..EndLine` spans, and `scalar` is not emitted
