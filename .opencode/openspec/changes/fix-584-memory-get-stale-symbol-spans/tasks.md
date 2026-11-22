# Tasks

- [x] Red integration tests: watcher backfill after restart; `memory_get`
      same-name ordering and stale-span end-to-end.
- [x] `ListFilesWithStaleSymbolSpans` sqlc query; deterministic
      `ResolveSymbolByName` ORDER BY (sqlc regenerate, no hand edits).
- [x] `Watcher.backfillSymbolSpans` once at startup after `processAll`, with
      the re-extract admission gates (filter, skipResolved, regular file,
      max size).
- [x] CHANGELOG entry; workaround note for older binaries.
- [x] Review rework: `parent_indexed` on the stale-span query so ignored or
      deleted files are never read; `DeleteStaleSymbolSpanDocs` after every
      pass so orphans converge; red tests for ignored/deleted/zero-symbol
      files and the third-start no-op.
- [x] Quick + full integration validation.
- [ ] Independent review and PR.
- [x] Review rework 2: delete stale docs only when provably orphaned —
      transient failures (unreadable, resolve/stat/read errors, failed
      upsert, missing extractor) keep them for the next start;
      `extractAndUpsertSymbols` reports `complete`. Red tests for
      unreadable file and missing extractor plus a mock-driven upsert
      failure unit test.
