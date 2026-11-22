# Design

`memory_get` slices the parent file's content by the symbol doc's
`line`/`end_line` metadata. Symbol docs written before #539 carry neither,
and their parents hit the content-hash early exit in `processFile` on every
scan — a restart alone can never repair them.

Rather than re-extract every file (`ReextractSymbolsForWorkspace` does that
but is far heavier than startup needs), a single query returns the parent
paths of just the stale docs: `ListFilesWithStaleSymbolSpans` selects
`split_part(source_path, '?symbol=', 1)` for symbol docs in a
(workspace, collection) whose `end_line` is absent or `'0'`, and reports
`parent_indexed` — whether the initial scan still indexes that file. The
watcher runs this once per collection in `Run`, after `processAll`.

Per returned file the loop applies the same per-file admission as the
re-extract walks — collection filter (`FileFilter.ShouldSkip`, the
single-path gate the hot-rescan handler uses), `skipResolved` containment
for any symlink component, regular-file and `max_file_size` checks — then
re-extracts through `extractAndUpsertSymbols`, so spans, content and chunks
land exactly as a fresh index would produce them. Stale docs are deleted
only when they are provably orphaned; anything ambiguous is kept for the
next start, because a deleted stale doc is never rebuilt (the content hash
keeps the file off every later scan).

Delete without reading when the row is provably orphaned: `parent_indexed`
is false (the initial scan deleted the file doc of every file a nested
`.gitignore`/`.nano-brainignore` now excludes — a plain `ShouldSkip` cannot
see nested ignore files and would silently re-index and embed the ignored
content), the filter skips the path, `skipResolved` says the docs should be
purged (outside root, denied target, unresolvable path), `os.Stat` reports
`fs.ErrNotExist` or a non-regular file, or the file exceeds
`max_file_size`.

Keep the docs — Debug log, no delete — on transient failure: `skipResolved`
skips without purge (unresolvable root, EIO, …), `os.Stat` fails otherwise,
`os.ReadFile` fails (EACCES, …), or no registered extractor supports the
extension (checked before reading, so a binary without the TS extractor
does not wipe `.ts` symbols).

For admitted files `extractAndUpsertSymbols` returns `(n, complete)`:
`complete` is false when `Extract` errors or any symbol's upsert or chunk
write fails. Docs are deleted only when `complete` — a partial pass is not
proof that the still-stale docs are orphans. Upsert alone does not converge
either: a symbol no longer emitted (renamed, removed, or a file that now
parses to zero symbols) would keep its empty `end_line` forever and be
re-extracted on every start, resetting `updated_at`. So a complete pass is
followed by `DeleteStaleSymbolSpanDocs`, which only matches docs still
lacking `end_line`: re-emitted symbols survive, orphans die. Files count
as repaired only when `n > 0`; deletions are counted separately.

Query errors are Warn-and-continue; a Warn is logged once per file on an
incomplete pass; the per-collection Info line reports both `repaired` and
`removed` counts, only when either is non-zero.

`ResolveSymbolByName` previously had no `ORDER BY`; a same-name collision
settled on heap order. A `CASE` on `metadata->>'kind'` ranks `function`
first, `struct`/`type`/`interface` next, `const`/`var`, `method` last,
everything else after, with `source_path` as a stable tie-break. Callers
iterate the list to prefer an exact file match, so the order only matters
within one file — where it now guarantees the function shadows the
object-property method rather than the reverse.

## Non-goals

- Transient failures (unreadable file, resolve error, failed upsert,
  missing extractor) are not retried within the pass; the kept docs are
  handled at the next start.
- Collections registered at runtime through hot paths are not backfilled;
  their stale spans are repaired at the next restart.
- Deleting fresh symbol docs is out of scope — the delete only matches
  rows still lacking `end_line`, so anything emitted by current extractors
  is never removed.
- Hot-event behaviour is unchanged: the backfill is synchronous, once in
  `Run` after `processAll`, and emits no notifications.

Regression coverage is red-green: a watcher integration test strips
`line`/`end_line` from a real indexed symbol doc, plants a stale orphan,
restarts the watcher on the unchanged tree, and asserts the span returns,
the orphan is deleted, an untouched neighbour keeps its `updated_at`, and a
third start does no further work; sibling tests cover the ignored-file,
deleted-file and zero-symbol deletions; an MCP integration test calls
`memory_get` on `src/handlers.ts::foo` where the file holds both a
`foo:` object property and `const foo`, and asserts the function body.
