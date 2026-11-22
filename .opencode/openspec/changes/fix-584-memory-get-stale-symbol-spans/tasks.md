# Tasks

- [x] Red integration tests: watcher backfill after restart; `memory_get`
      same-name ordering and stale-span end-to-end.
- [x] `ListFilesWithStaleSymbolSpans` sqlc query; deterministic
      `ResolveSymbolByName` ORDER BY (sqlc regenerate, no hand edits).
- [x] `Watcher.backfillSymbolSpans` once at startup after `processAll`, with
      the re-extract admission gates (filter, skipResolved, regular file,
      max size).
- [x] CHANGELOG entry; workaround note for older binaries.
- [x] Quick + full integration validation.
- [ ] Independent review and PR.
