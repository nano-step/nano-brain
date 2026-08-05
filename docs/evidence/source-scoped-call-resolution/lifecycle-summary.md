# Task 3.2 lifecycle summary

`TestReextractEdgesForWorkspaceReplacesSourceEdges` proves, against an isolated
Postgres schema, that contextual edges are source-scoped and replacement-safe:

- initial indexing resolves the consumer to `repo-a/lib/api.ts::run`, despite
  the same export name in repo-b;
- two consecutive workspace re-extractions do not increase the consumer edge
  count;
- editing the consumer transitions its call target to `<unresolved>`;
- the unrelated repo-b exporter edge remains present;
- renaming the exporter and driving the watcher dirty/contextual event path
  leaves the importer unresolved rather than retaining a stale canonical edge.

The production watcher and contextual extractor paths are covered by this
change; no schema migration is required.
