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

- Backfill stale symbol spans at watcher start: a new query lists the
  parent files of symbol docs without `end_line` and whether each file is
  still indexed; the watcher re-runs symbol extraction for just the
  admitted files after the initial scan.
- Converge in one pass: stale docs whose symbols are no longer emitted —
  or whose files are no longer indexed (deleted, newly ignored) — are
  deleted instead of re-attempted on every start.
- Order `ResolveSymbolByName` deterministically — functions first, then
  types, consts/vars, and methods last — so same-name lookups are stable.
- Workaround for already-indexed workspaces on older binaries:
  `POST /api/v1/update`.
