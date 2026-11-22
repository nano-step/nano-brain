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
land exactly as a fresh index would produce them. Symbol docs follow the
file doc: while the parent document exists, its stale symbol docs are
deleted only after a complete re-extraction proves they are orphans; every
other outcome keeps them, because deleting is irreversible — the file's
unchanged content hash keeps it off every subsequent scan, so a deleted
stale doc is never rebuilt.

Delete without reading only when `parent_indexed` is false: the initial
scan deleted the file doc of every file a nested `.gitignore`/
`.nano-brainignore` now excludes, so the parent row is the admission
record — a plain `ShouldSkip` cannot see nested ignore files and would
silently re-index and embed the ignored content. Once the parent doc is
gone, the row is cleaned up on a later start.

With the parent present, the file must also have been admitted by the
same run's initial scan — `startupAdmitted` records every path the walk
reaches past `adm.ignore`, because a surviving file doc does not prove
admission (the walk may never have reached the file, and a transient
`cleanupIgnoredDocument` failure leaves the doc behind). Rows outside
that set are kept unread. Beyond that, keep the docs — Debug log, no
delete — on every other outcome, in order: the filter skips the path,
`skipResolved` skips
(purge or not — a file the scan no longer serves may still be indexed
again later), `os.Stat` fails for any reason including `fs.ErrNotExist`
(the file may return unchanged; the hash path never re-extracts it), the
file is not regular or exceeds `max_file_size` (a config value that can
be raised later), no registered extractor supports the extension (checked
before reading, so a binary without the TS extractor does not wipe `.ts`
symbols), the extension is a known binary type, `os.ReadFile` fails
(EACCES, …), or the content is binary — the same `isBinaryExtension` /
`isBinaryContent` gates `processFile` applies, so the backfill never reads
symbols out of content normal indexing rejects.

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

- Non-admitted or partially processed files (ignored, unresolvable,
  missing, non-regular, oversized, binary, unreadable, unsupported
  extension, incomplete upsert) are not retried within the pass; the kept
  docs are handled at the next start.
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
