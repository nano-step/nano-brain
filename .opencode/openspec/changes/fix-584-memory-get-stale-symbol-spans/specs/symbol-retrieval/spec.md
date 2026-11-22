# Symbol Retrieval

## ADDED Requirements

### Requirement: Startup backfill of stale symbol spans
The watcher SHALL re-extract symbols from the parent files of symbol
documents without `end_line` metadata during startup, after the initial
scan, so `memory_get` can slice the full symbol body.

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

### Requirement: Convergent stale-doc cleanup
After the admission and re-extraction pass for a file, the watcher MUST
delete its remaining spanless symbol docs — symbols no longer emitted, or
files no longer indexed — so a later restart finds no stale rows and does
no further work.

#### Scenario: Orphan stale doc for a removed symbol is deleted
Given a stale symbol doc for a name the file no longer declares,
when the watcher starts and backfills that file,
then the re-emitted symbols keep their spans and the orphan doc is
deleted, and a subsequent start leaves the file's `updated_at`
untouched.

#### Scenario: Stale docs of a deleted file are deleted
Given a stale symbol doc whose parent file was removed from disk,
when the watcher starts on the tree without it,
then the stale doc is deleted.

#### Scenario: Stale doc survives no extraction
Given a file that parses to zero symbols but still has a stale symbol
doc,
when the watcher starts,
then the stale doc is deleted and the file is not counted as repaired.

### Requirement: Ignored files are never re-indexed by the backfill
The backfill MUST NOT read or re-extract a file the initial scan treated
as ignored — including files excluded only by a nested `.gitignore` or
`.nano-brainignore` — and MUST delete its leftover stale symbol docs.

#### Scenario: Newly ignored file's stale symbols are dropped
Given a file that was indexed, then excluded by a new nested
`.gitignore` entry and edited,
when the watcher restarts,
then the file's stale symbol docs are deleted and no new symbol docs are
created from its ignored content.

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
