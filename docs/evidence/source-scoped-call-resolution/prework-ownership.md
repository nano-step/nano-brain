# Prework Ownership and Non-Overlap (#609)

## Sources Reviewed

- GitHub issue #609: open high-risk graph bug fix for source-scoped JS/TS call
  targets; its scope names extractor output, graph identity, reindex behavior,
  and trace/flow/impact correctness.
- GitHub issue #501 and its local OpenSpec artifact
  `fix-import-target-resolution`: historical import-edge target normalization.
  The issue is closed, while the local artifact remains as stale historical
  planning material.
- Merged PR #555 (`d6eea9911a804b043676c786e32d15cd54a68d1a`, merged
  2026-07-06): extraction-time JS/TS/Vue import-specifier resolution to
  workspace paths. It is a prerequisite for project-local import binding, not
  a calls-edge resolver.
- Open PR #504: `state=OPEN`, `mergeable=CONFLICTING`, with 18 changed files.
  Its file list includes the stale #501 OpenSpec artifact and import resolver,
  graph-path, and watcher work.

Commands used for GitHub facts:

```text
gh issue view 609 --repo nano-step/nano-brain --json number,title,state,author,labels,body,url,comments
gh issue view 501 --repo nano-step/nano-brain --json number,title,state,author,labels,body,url,comments
gh pr view 555 --repo nano-step/nano-brain --json number,title,state,mergedAt,mergeCommit,headRefName,baseRefName,body,files,url
gh pr view 504 --repo nano-step/nano-brain --json state,mergeable,files
```

Private workspace examples present in the historical #501 issue text are
intentionally omitted from this durable record.

## Disposition

**Disposition: proceed as a new #609 change; do not reuse or mutate #501/#504.**

#555 owns the completed import-specifier/path normalization behavior. #609
depends on that current behavior and owns a different contract: resolving
`calls` edges from source scope, representing unprovable calls as
`<unresolved>`, and preventing canonical or unresolved call targets from
creating reader fan-out. The stale #501 artifact instead describes import-edge
normalization and fallback of unresolved module specifiers; reusing it would
conflate completed import work with the new calls-edge safety contract.

#504 is open and conflicting, but its exact changed paths do not intersect this
task's new OpenSpec directory or evidence files. It receives no comment,
state, branch, or artifact change from this task. If a future implementation
needs to modify a still-conflicting #504-owned path, the delivery coordinator
must obtain an explicit ownership resolution first.
