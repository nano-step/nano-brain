# Symbol Retrieval

## ADDED Requirements

### Requirement: Startup backfill of stale symbol spans
The watcher SHALL re-extract symbols from the parent files of symbol
documents without `end_line` metadata once during startup, after the
initial scan, so `memory_get` can slice the full symbol body.

#### Scenario: Stale symbol doc backfilled at watcher start
Given a workspace whose symbol doc for `foo` lacks `line`/`end_line`
metadata,
when the watcher starts on the unchanged tree,
then the symbol doc is re-extracted with its true line span, and
`memory_get` on `file::foo` returns the full body ending at the closing
line.

### Requirement: No blanket re-extract
The backfill MUST only touch files that parent a stale symbol document;
files whose symbol docs already carry complete spans MUST NOT be
re-extracted at startup.

#### Scenario: Unchanged file with complete spans untouched
Given a second file whose symbol docs already carry `line`/`end_line`,
when the startup backfill repairs a different file,
then the second file's symbol doc is not rewritten — its `updated_at`
is unchanged.

### Requirement: Deterministic same-name symbol resolution
`file::Symbol` lookups SHALL resolve name collisions in a fixed order:
functions first, then types (`struct`/`type`/`interface`), then
`const`/`var`, and methods last, with `source_path` as the final
tie-break.

#### Scenario: Same-file object property and const share a name
Given a file declaring both `handlers = { foo: async (a) => {…} }` and
`const foo = async (…) => {…}`,
when `memory_get` is called for `file::foo` repeatedly,
then every call returns the `const foo` function body, never the
object-property method's span.
