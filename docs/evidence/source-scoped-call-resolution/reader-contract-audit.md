# #609 Reader Contract Audit

## Scope and conclusion

This is the closed reader inventory and implementation audit for
`source-scoped-call-resolution` task 1.3. The four completion searches below
were run with
`--glob '!**/*_test.go'`; generated `internal/storage/sqlc/*.go` results are
listed as generated mirrors of the corresponding source query and are not
implementation edit targets.

Target classes are deliberately disjoint:

- Canonical JS/TS: an admitted `path.(js|jsx|mjs|cjs|ts|tsx|mts|cts)::symbol`
  identity. It is exact-only: no suffix, symbol-name, or cross-file fallback.
- Legacy: all other historical values, including bare names and Ruby namespace
  values such as `Api::V2::StoriesController#sync`; their existing tested
  fallback remains in force.
- `<unresolved>`: a persisted diagnostic target only. It is visible only in its
  originating raw one-hop calls edge. It is not a graph node, traversal
  frontier, topology/degree/PageRank member, materialized flow/document/hash,
  cache input, or derived REST/MCP count/response member.

The implementation closes the five initially identified seams: qualified flow
continuation, canonical impact suffix expansion, unresolved
flow/materialization, unresolved PageRank topology, and unresolved graph-context
cache input. The historical red controls named below now pass; legacy Ruby
controls remain green.

## Reader inventory

| Surface and source inventory | Canonical JS/TS policy | Legacy policy | `<unresolved>` policy | Topology/degree/cache policy | Focused proof / implementation risk |
| --- | --- | --- | --- | --- | --- |
| Carrier, de-dup, contextual registry: `internal/graph/{edge.go,registry.go}` | Preserve exact target value and source identity. | Preserve existing emitted target values. | Persist target-only value; no reason/raw-callee metadata. | No reader-side normalization. | `internal/graph/source_scoped_call_resolution_contract_test.go`; registry is a writer, not a derived reader. |
| Watcher writes/replaces and Ruby reconcile writes: `internal/watcher/watcher.go`, `internal/graph/ruby_resolver.go` | Persist exact source-scoped edge unchanged. | Keep Ruby namespace/reconcile behavior. | Store raw diagnostic edge only. | Do not create reader cache/index work from sentinel. | Existing watcher/Ruby tests; `TestLegacyRubyNamespacedTargetRetainsReconciliation` is green. |
| Raw graph SQL: `internal/storage/queries/graph.sql` (`GetOutgoingEdges`, `ListAllEdgesByWorkspace`) | Exact raw row allowed. | Existing exact raw row allowed. | The originating raw edge may be returned by a one-hop raw query only. | `ListTopGraphNodesByDegree`, `CountDistinctGraphNodes`, `ListEdgesBetweenNodes`, and `ListEdgesTouchingNodes` filter sentinel before derived result construction. | Flow/PageRank/storage regressions pass. |
| Incoming/impact SQL: `GetIncomingEdges`, `GetImpactors`, `GetImpactorsByTargets`, `GetOutgoingEdgesBySymbol` | Exact match only; never use `split_part` or bare expansion for canonical calls. | Retain existing suffix/bare compatibility. | Exclude before incoming/impactor query and reconciliation. | Never expand sentinel into a frontier. | Canonical exact and legacy suffix controls pass. |
| Impact frontier and REST/MCP mapping: `internal/symbol/impact_frontier.go`, `internal/server/handlers/impact.go`, `internal/mcp/tools.go` | Canonical target stays singular in frontier/response. | Bare suffix remains for legacy target only. | Exclude before queue/frontier/response counts. | No cache or traversal fan-out. | Symbol and MCP sentinel guards pass. |
| Trace and graph raw response: `internal/server/handlers/{graph.go,trace.go,graph_neighborhood.go,graph_overview.go}`, MCP graph/trace in `internal/mcp/tools.go` | Follow exact canonical node identity. | Existing bare resolution stays available. | Raw one-hop edge is the sole allowed diagnostic visibility; BFS/neighborhood/overview nodes, counts, and next queue exclude it. | Sentinel cannot become a shared node/hub. | Handler, MCP, and streamable HTTP controls pass. |
| Flow builder: `internal/flow/builder.go` | Traverse `idx.bySource[canonical]` only, retaining canonical node IDs. | Retain `symbolPart` reconciliation only for legacy. | Skip before node/edge/queue construction. | No canonical-to-bare fan-out; no sentinel traversal. | `TestQualifiedCallTraversesOnlyItsExactTarget` and `TestUnresolvedCallDoesNotEnterDerivedFlow` pass. |
| Flow materializer and REST/MCP flow: `internal/flow/materializer.go`, `internal/server/handlers/flow.go`, MCP flow in `internal/mcp/tools.go` | Materialize canonical exact chain/IDs. | Existing bare reconciliation continues. | Exclude before chain, content hash, document/chunk, and response. | Sentinel must not alter cache/hash/index. | `TestMaterializerDoesNotCacheUnresolvedFlowContent` passes. |
| CFG loader and REST/MCP flowchart: `internal/flow/cfg_loader.go`, `internal/server/handlers/flowchart.go`, `internal/mcp/flowchart.go` | Preserve exact call identity where a call target is used. | Existing handler-name logic unchanged. | Exclude before derived flowchart lookup/output if a calls target reaches it. | CFG parser itself is unrelated to calls-edge traversal. | Included as a boundary; `cfg_loader.go` normalization remains unchanged and parser-only `js_cflow.go`/`ruby_cflow.go` matches are irrelevant. |
| PageRank and overview API: `internal/graph/{pagerank.go,pagerank_service.go}`, `internal/storage/queries/pagerank.sql`, `internal/server/handlers/graph_pagerank.go` | Score exact canonical node. | Score existing legacy nodes. | Filter before `ListCallEdges`, graph construction, degree, score persistence, and response. | Sentinel cannot affect degree/PageRank. | `TestComputePageRankExcludesUnresolvedCallTarget` passes. |
| Code summary context/cache/cascade: `internal/codesummarize/{graph_context.go,cascade.go,service.go}`, `internal/storage/queries/code_summarization.sql` | Cache uses exact canonical caller/callee IDs. | Existing legacy IDs remain. | Filter before context, hash, cascade, queue, and chunk hash update. | Sentinel must not invalidate/cache a shared synthetic symbol. | `TestFetchGraphContextExcludesUnresolvedCalleeFromCacheInput` passes. |
| Migration and generated sqlc: `migrations/00008_knowledge_graph.sql`, `internal/storage/sqlc/{graph.sql.go,pagerank.sql.go,code_summarization.sql.go,models.go}` | Schema/model carrier only; source SQL owns semantics. | Unchanged. | No new column or metadata. | Source SQL and generated mirrors are synchronized; no schema change. | Targeted storage integration passes; `sqlc` is unavailable in the environment, so mirrors were synchronized manually. |

## Unresolved boundary map

The following distinction is mandatory and is intentionally narrower than
"hide it everywhere": the persisted source edge can be inspected as a raw
one-hop diagnostic edge, but every derived reader must drop the target before
it becomes a node, an expansion key, or cached/materialized state.

| Boundary | Raw one-hop sentinel allowed? | Derived exclusion point | Regression proof |
| --- | --- | --- | --- |
| `GetOutgoingEdges` and direct graph row mapping | Yes, only the source's own stored edge. | Before BFS/neighborhood/overview node maps and response totals. | `TestGraphTraceExcludesUnresolvedFromDerivedResponse` passes; direct raw graph output remains an allowed diagnostic control. |
| `GetIncomingEdges`, `GetImpactors*`, `GetOutgoingEdgesBySymbol`, impact frontier | No. | Before SQL target comparison, suffix expansion, and frontier enqueue. | Canonical exact and legacy suffix controls pass. |
| `BuildFlow`, REST/MCP flow and flowchart adapter paths | No. | Before flow node map, edge set, role classification, queue, and fan-out. | `TestUnresolvedCallDoesNotEnterDerivedFlow` passes. |
| `Materializer` flow text, hash, document/chunk and embedding queue | No. | Before `BuildFlow` result is rendered/hashed/upserted. | `TestMaterializerDoesNotCacheUnresolvedFlowContent` passes. |
| Degree, `CountDistinctGraphNodes`, `ListTopGraphNodesByDegree`, PageRank | No. | Before target projection and `ComputePageRank` graph construction. | `TestComputePageRankExcludesUnresolvedCallTarget` passes. |
| `FetchGraphContext`, `ComputeGraphContextHash`, cascade invalidation, summary queue | No. | Before caller/callee context map and cache/hash/cascade collection. | `TestFetchGraphContextExcludesUnresolvedCalleeFromCacheInput` passes. |
| REST/MCP trace, impact, graph overview/neighborhood counts | No, except the direct raw edge above. | Before response node map, traversal queue, paths, counts, and response mapping. | Handler/MCP unit controls and streamable HTTP integration pass; live `:3199` smoke is environment-blocked because no listener was available. |

## Completion-rule manifests

All commands were executed from the change worktree. Each manifest is a
closed source-level ledger: every individual match is classified by the file
class listed below. `included` means the match belongs to one of the reader
inventory rows above; `irrelevant` means it is an extractor/writer, generated
mirror, framework parser, or non-graph string utility; `new row` would have
required adding an inventory row. No `new row` was found.

### Manifest 1 — production `TargetNode` (200 matches)

```sh
rg -n --glob '*.go' --glob '*.sql' --glob '!**/*_test.go' 'TargetNode' internal migrations
```

| Every matching file class | Disposition and rationale |
| --- | --- |
| `internal/flow/{builder.go,materializer.go,cfg_loader.go}` | Included: flow traversal, materialization, and CFG-adjacent target normalization. |
| `internal/codesummarize/graph_context.go`, `internal/graph/{pagerank.go,pagerank_service.go}` | Included: cache and topology/PageRank consumers. |
| `internal/mcp/tools.go`, `internal/server/handlers/{graph.go,graph_neighborhood.go,graph_overview.go,graph_pagerank.go,trace.go,flow.go,flowchart.go}` | Included: raw/derived REST and MCP mapping/traversal surfaces. |
| `internal/storage/sqlc/{graph.sql.go,pagerank.sql.go,code_summarization.sql.go,models.go,stats.sql.go}` | Included as generated mirrors; source SQL is the implementation target. |
| `internal/watcher/watcher.go`, `internal/graph/{edge.go,registry.go,ruby_resolver.go,ruby_class_index.go}` | Included carrier/writer and Ruby compatibility boundary. |
| `internal/graph/{javascript,typescript,vue_sfc,go,python,ruby,echo,express,gin,nestjs,nethttp,nuxtjs,rails,rails_dsl,integration,js_integration,python_integration}_extractor.go`, `internal/graph/import_resolver.go` | Irrelevant to reader semantics: these are target producers. Their values are covered by the carrier row, not a consumer change. |
| `internal/links/extract.go`, `internal/server/{linksadapter.go,handlers/links.go}` | Irrelevant: document reference links, not calls-edge readers. |

### Manifest 2 — target fields in reader packages (280 matches)

```sh
rg -n --glob '*.go' --glob '*.sql' --glob '!**/*_test.go' \
  'TargetNode|target_node' internal/storage internal/flow internal/symbol \
  internal/mcp internal/server internal/codesummarize internal/graph internal/watcher
```

| Every matching file class | Disposition and rationale |
| --- | --- |
| `internal/storage/queries/{graph.sql,pagerank.sql,code_summarization.sql}` | Included source of exact/fallback SQL, degree, PageRank, and summary context behavior. |
| `internal/storage/queries/stats.sql` | Irrelevant: it counts document `references` backlinks only, never reads a `calls` target for traversal, reconciliation, topology, or cache work. |
| `internal/storage/sqlc/{graph.sql.go,pagerank.sql.go,code_summarization.sql.go,models.go,stats.sql.go}` | Included generated mirrors; do not edit directly. |
| `internal/symbol/impact_frontier.go` | Included: canonical targets bypass `::` suffix expansion while legacy targets retain it. |
| `internal/flow/{builder.go,materializer.go,cfg_loader.go}` | Included: canonical traversal/derived document and flowchart-adjacent normalization. |
| `internal/codesummarize/graph_context.go`, `internal/graph/{pagerank.go,pagerank_service.go}` | Included: derived cache and topology construction. |
| `internal/mcp/tools.go`, `internal/server/handlers/{graph.go,graph_neighborhood.go,graph_overview.go,graph_pagerank.go,trace.go,flow.go,flowchart.go}` | Included: REST/MCP reader mappings. |
| `internal/server/handlers/links.go`, `internal/server/linksadapter.go` | Irrelevant: these map document reference-link targets, not graph `calls` targets; they cannot construct calls traversal, topology, or cache state. |
| `internal/graph/{edge.go,registry.go,ruby_resolver.go,ruby_class_index.go}`, `internal/watcher/watcher.go` | Included carrier/source-replacement/Ruby boundary. |
| All extractor files and `internal/graph/import_resolver.go` also matched by manifest 1 | Irrelevant producers; no target is read for graph traversal/cache/materialization there. |

### Manifest 3 — normalization and reconciliation patterns (227 matches)

```sh
rg -n --glob '*.go' --glob '*.sql' --glob '!**/*_test.go' \
  'split_part|strings\.(Split|LastIndex|Contains|HasSuffix)|symbolPart|flowSymbol|unresolved|ListCallEdges|GetIncomingEdges|GetImpactors|GetOutgoingEdgesBySymbol' \
  internal migrations
```

| Every matching file class | Disposition and rationale |
| --- | --- |
| `internal/storage/queries/graph.sql`, `internal/storage/sqlc/graph.sql.go`, `internal/symbol/impact_frontier.go` | Included: the `split_part`/suffix paths are the exact canonical-to-bare risk; generated Go mirrors SQL only. |
| `internal/flow/{builder.go,cfg_loader.go,sequence.go}`, `internal/server/handlers/flow.go`, `internal/mcp/tools.go` | Included: `symbolPart`, flow symbol matching, queue/derived flow output, and flowchart mapping require target-class gates. |
| `internal/server/handlers/{graph.go,graph_neighborhood.go,graph_pagerank.go,impact.go,trace.go,flowchart.go}`, `internal/mcp/{tools.go,flowchart.go,graph_paths.go}` | Included: traversal/reconciliation and REST/MCP response boundaries. |
| `internal/codesummarize/{graph_context.go,service.go}`, `internal/graph/pagerank_service.go` | Included: cache and PageRank readers. |
| `internal/codesummarize/retry.go` | Irrelevant: retry/backoff configuration uses `strings.Contains` for provider errors; it does not inspect graph nodes, targets, or graph context. |
| `internal/graph/{registry.go,ruby_resolver.go,ruby_class_index.go}` | Included carrier/Ruby compatibility; namespace splitting must remain legacy-only. |
| `internal/graph/{detector.go,import_resolver.go,js_cflow.go,js_integration_extractor.go,nestjs_extractor.go,nuxtjs_extractor.go,rails_dsl_extractor.go,rails_extractor.go,ruby_cflow.go,ruby_extractor.go,ts_router_helpers.go,vue_sfc_extractor.go}` | Irrelevant parser/extractor/framework matching: no calls-edge reader traversal, cache, or materialization is constructed. |
| `internal/{bench,chunk,config,embed,harvest,links,migrate,search,summarize}/**`, `internal/server/middleware/**`, `migrations/00018_code_summarization_failures.sql` | Irrelevant non-reader string/config/search/migration matches. They never consume a graph calls target. |
| `internal/server/handlers/{code_summarize_failures.go,code_summarize_retry.go,collection.go,documents.go}` | Irrelevant: these handlers use `strings.Contains`/splits for summary-failure, collection, or document request processing; none reads a graph calls target or emits graph-derived traversal/topology/cache state. |
| `internal/storage/{queries/documents.sql,queries/flowcharts.sql,sqlc/documents.sql.go,sqlc/flowcharts.sql.go,sqlc/pagerank.sql.go}` | Irrelevant document/CFG storage or generated SQL mirror; the only reader-relevant PageRank source is already included above. |
| `internal/watcher/{filter.go,watcher.go}` | Included only for source edge extraction/replacement boundary; extension/path string operations are otherwise irrelevant to reader target semantics. |

### Manifest 4 — graph/cache/materialization entry points (159 matches)

```sh
rg -n --glob '*.go' --glob '*.sql' --glob '!**/*_test.go' \
  'GetCalleeNodes|FetchGraphContext|GraphContext|graph_context_hash|ComputePageRank|IncrementEdgeCount|extractAndUpsertEdges|ListAllEdgesByWorkspace|BuildFlow|LoadFlowCFGs|GetIncomingEdges|GetImpactors|GetOutgoingEdgesBySymbol' \
  internal migrations
```

| Every matching file class | Disposition and rationale |
| --- | --- |
| `internal/codesummarize/{cascade.go,graph_context.go,prompt.go,provider.go,retry.go,service.go}` | Included: graph context creation, prompt/cache hashing, cascade/queue behavior. Provider/retry matches are call sites only but are part of the same cache pipeline. |
| `internal/flow/{builder.go,cfg_loader.go,materializer.go}`, `internal/server/handlers/flow.go`, `internal/mcp/tools.go` | Included: flow traversal, flow materialization/hash, REST/MCP mapping and CFG loading. |
| `internal/symbol/impact_frontier.go` | Included: it expands impact frontier targets. Canonical JS/TS targets stay exact; legacy qualified Ruby/bare targets retain suffix expansion; `<unresolved>` never enters a frontier. Both focused controls pass. |
| `internal/storage/queries/{graph.sql,code_summarization.sql}`, `internal/storage/sqlc/{graph.sql.go,code_summarization.sql.go,models.go}` | Included source queries and generated mirrors for traversal/cache data. |
| `internal/graph/{pagerank.go,pagerank_service.go}`, `internal/storage/queries/pagerank.sql`, `internal/storage/sqlc/pagerank.sql.go`, `internal/server/handlers/graph_pagerank.go` | Included: PageRank/degree pipeline. |
| `internal/server/handlers/{graph.go,graph_neighborhood.go,impact.go,trace.go,flowchart.go}` | Included: graph/impact/trace/flowchart derived response mapping. |
| `internal/watcher/{watcher.go,filter.go}` | Included: edge upsert/replacement lifecycle boundary; `filter.go` is irrelevant to reader decisions after admission. |
| `internal/storage/sqlc/embeddings.sql.go`, `migrations/00022_graph_context_hash.sql` | Irrelevant generated embedding storage/schema carrier: no target class decision. |

The search counts are a review guard. If any command later reports a new
match, the implementation owner must classify it as an existing row, add a new
row, or document why it is irrelevant before changing a reader.

## Focused test evidence

Raw output is recorded in
`.omo/evidence/609-source-scoped-call-resolution/task-3-609-source-scoped-call-resolution.md`.

| Control | Command | Outcome | Reader seam proved |
| --- | --- | --- | --- |
| Qualified exact flow | `go test ./internal/flow -run 'TestQualifiedCallTraversesOnlyItsExactTarget$' -count=1` | Pass. | `BuildFlow` indexes/follows canonical target exactly and does not fall back to repo-b's bare `run`. |
| Unresolved derived flow | `go test ./internal/flow -run 'TestUnresolvedCallDoesNotEnterDerivedFlow$' -count=1` | Pass. | Raw one-hop diagnostic visibility is distinct from derived flow output. |
| Unresolved materializer/cache | `go test ./internal/flow -run 'TestMaterializerDoesNotCacheUnresolvedFlowContent$' -count=1` | Pass. | `Materializer` filters before document/chunk/cache work. |
| Canonical impact no suffix fallback | `go test ./internal/symbol -run 'TestExpandImpactFrontierDoesNotBareExpandCanonicalJSTarget$' -count=1` | Pass. | `repo-a/lib/api.ts::run` remains exact and does not expand to bare `run`. |
| Legacy impact suffix control | `go test ./internal/symbol -run 'TestExpandImpactFrontierKeepsLegacyBareSuffixExpansion$' -count=1` | Green. | Historical Ruby/bare compatibility remains. |
| Unresolved PageRank topology | `go test ./internal/graph -run 'TestComputePageRankExcludesUnresolvedCallTarget$' -count=1` | Pass. | Degree/PageRank filters before graph construction. |
| Unresolved graph-context cache | `go test ./internal/codesummarize -run 'TestFetchGraphContextExcludesUnresolvedCalleeFromCacheInput$' -count=1` | Pass. | Context hash/cascade cache filters before map construction. |
| Unresolved REST trace response | `go test ./internal/server/handlers -run 'TestGraphTraceExcludesUnresolvedFromDerivedResponse$' -count=1` | Pass. | Raw edge visibility does not authorize a derived REST traversal/response node. |
| Ruby namespace compatibility | `go test ./internal/flow ./internal/symbol -run 'Test(LegacyRubyNamespacedTargetRetainsReconciliation|NamespacedControllerReconciliation|ExpandImpactFrontierKeepsLegacyBareSuffixExpansion)$' -count=1` | Green. | Ruby `::` remains legacy; it is never treated as canonical JS/TS merely because it contains `::`. |

Independent verifier commands (no database or server required):

```sh
go test ./internal/flow ./internal/symbol ./internal/graph ./internal/codesummarize ./internal/server/handlers \
  -run 'Test(Qualified|Unresolved|LegacyRuby|ExpandImpactFrontier|ComputePageRank|FetchGraphContext|Materializer)' -count=1

go test ./internal/flow ./internal/symbol \
  -run 'Test(LegacyRubyNamespacedTargetRetainsReconciliation|NamespacedControllerReconciliation|ExpandImpactFrontierKeepsLegacyBareSuffixExpansion)$' -count=1
```
