# Restore full symbol bodies in memory_get (#584)

## Why

`memory_get` on `file::Symbol` returns only the declaration line when the
symbol document was indexed before #539 added `line`/`end_line` metadata.
Unchanged files exit at the content-hash check before symbol extraction, so
the missing span is never repaired on restart or incremental reindex. A
second defect makes same-name resolution nondeterministic: an object-property
`foo:` (kind=method) can shadow a top-level `const foo` (kind=function) in
the same file, so `file::foo` returns the wrong body.

## What

- Backfill stale symbol spans once at watcher start: a new query lists the
  parent files of symbol docs without `end_line`; the watcher re-runs symbol
  extraction for just those files after the initial scan.
- Order `ResolveSymbolByName` deterministically — functions first, then
  types, consts/vars, and methods last — so same-name lookups are stable.
- Workaround for already-indexed workspaces on older binaries:
  `POST /api/v1/update`.
