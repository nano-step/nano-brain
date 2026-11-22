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
land exactly as a fresh index would produce them. Two properties make the
pass convergent rather than a repair attempt repeated on every start:

- **The parent doc is the admission record.** The initial scan already
  deleted the file doc of every file a nested `.gitignore`/
  `.nano-brainignore` now excludes, so `parent_indexed = false` means the
  file must not be read at all — a plain `ShouldSkip` check cannot see
  nested ignore files and would silently re-index the ignored file and
  enqueue embeddings. Rows with no parent have their stale symbol docs
  deleted without a read; a deleted file's doc can linger, but the Stat
  check catches that case.
- **Upsert alone does not converge.** A stale doc for a symbol no longer
  emitted (renamed, removed, or a file that now parses to zero symbols)
  would keep its empty `end_line` forever and be re-extracted on every
  start, resetting `updated_at` and skewing recency ranking. So after every
  admission outcome — re-extracted or not — `DeleteStaleSymbolSpanDocs`
  removes the docs still lacking a span for that file. Re-emitted symbols
  carry `end_line` and survive; whatever is deleted was an orphan. Files
  are counted as repaired only when extraction re-emitted at least one
  symbol; deletions are counted separately.

Query errors are Warn-and-continue; the log line reports both `repaired`
and `removed` counts per collection, only when either is non-zero.

`ResolveSymbolByName` previously had no `ORDER BY`; a same-name collision
settled on heap order. A `CASE` on `metadata->>'kind'` ranks `function`
first, `struct`/`type`/`interface` next, `const`/`var`, `method` last,
everything else after, with `source_path` as a stable tie-break. Callers
iterate the list to prefer an exact file match, so the order only matters
within one file — where it now guarantees the function shadows the
object-property method rather than the reverse.

## Non-goals

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
