# Design

`memory_get` slices the parent file's content by the symbol doc's
`line`/`end_line` metadata. Symbol docs written before #539 carry neither,
and their parents hit the content-hash early exit in `processFile` on every
scan — a restart alone can never repair them.

Rather than re-extract every file (`ReextractSymbolsForWorkspace` does that
but is far heavier than startup needs), a single query returns the parent
paths of just the stale docs: `ListFilesWithStaleSymbolSpans` selects
`split_part(source_path, '?symbol=', 1)` for symbol docs in a
(workspace, collection) whose `end_line` is absent or `'0'`. The watcher
runs this once per collection in `Run`, after `processAll`, and applies the
same per-file admission as the re-extract walks — collection filter
(`FileFilter.ShouldSkip`, the single-path gate the hot-rescan handler uses),
`skipResolved` containment for any symlink component, regular-file and
`max_file_size` checks — before re-extracting. Query errors are Warn-and-
continue; missing/unreadable files are skipped silently. Each backfill
writes through the normal `extractAndUpsertSymbols` path, so spans, content
and chunks land exactly as a fresh index would produce them.

`ResolveSymbolByName` previously had no `ORDER BY`; a same-name collision
settled on heap order. A `CASE` on `metadata->>'kind'` ranks `function`
first, `struct`/`type`/`interface` next, `const`/`var`, `method` last,
everything else after, with `source_path` as a stable tie-break. Callers
iterate the list to prefer an exact file match, so the order only matters
within one file — where it now guarantees the function shadows the
object-property method rather than the reverse.

Regression coverage is red-green: a watcher integration test strips
`line`/`end_line` from a real indexed symbol doc, restarts the watcher on
the unchanged tree, and asserts the span returns while an untouched
neighbour file keeps its `updated_at`; an MCP integration test calls
`memory_get` on `src/handlers.ts::foo` where the file holds both a
`foo:` object property and `const foo`, and asserts the function body.
