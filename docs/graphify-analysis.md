# Graphify Analysis: Architecture & Integration Plan

**Date:** 2026-06-11
**Repo:** https://github.com/safishamsi/graphify (65k+ stars, YC S26)
**Status:** Analysis complete, implementation deferred (lower priority than current issues)

---

## Pipeline Architecture

```
detect() → extract() → build_graph() → cluster() → analyze() → report() → export()
```

Each stage is a single function in its own module. Communication via plain Python dicts and NetworkX graphs.

---

## Key Files Analyzed

| File | Lines | What It Does |
|---|---|---|
| `cluster.py` | 272 | Leiden community detection via graspologic, fallback to Louvain via networkx |
| `analyze.py` | 732 | God nodes, surprising connections, suggest_questions, import cycles, graph_diff |
| `extract.py` | 11,748 | Tree-sitter AST extraction for 28+ languages |
| `ARCHITECTURE.md` | 85 | Module responsibilities, extraction schema, confidence labels |

---

## Extraction Schema

```json
{
  "nodes": [
    {"id": "unique_string", "label": "human name", "source_file": "path", "source_location": "L42"}
  ],
  "edges": [
    {"source": "id_a", "target": "id_b", "relation": "calls|imports|uses|...", "confidence": "EXTRACTED|INFERRED|AMBIGUOUS"}
  ]
}
```

---

## Confidence Labels

| Label | Meaning |
|---|---|
| `EXTRACTED` | Explicitly stated in source (import statement, direct call) |
| `INFERRED` | Reasonable deduction (call-graph second pass, co-occurrence) |
| `AMBIGUOUS` | Uncertain, flagged for human review |

---

## Community Detection (cluster.py)

### Algorithm

- Primary: Leiden via `graspologic.partition.leiden`
- Fallback: Louvain via `networkx.community.louvain_communities`
- Both require undirected graphs

### Key Functions

| Function | Signature | Returns |
|---|---|---|
| `cluster` | `cluster(G, resolution, exclude_hubs_percentile)` | `{community_id: [node_ids]}` |
| `_partition` | `_partition(G, resolution)` | `{node_id: community_id}` |
| `_split_community` | `_split_community(G, nodes)` | `list[list[str]]` (second Leiden pass) |
| `cohesion_score` | `cohesion_score(G, community_nodes)` | `float` (actual/max possible edges) |
| `remap_communities_to_previous` | `remap_communities_to_previous(communities, prev)` | `dict` (stable IDs across runs) |

### Parameters

- `resolution > 1.0` → more, smaller communities
- `resolution < 1.0` → fewer, larger communities
- `exclude_hubs_percentile` → nodes above this degree excluded from partitioning

### Splitting Rules

- Communities > 25% of graph (min 10 nodes) → split via second Leiden pass
- Communities with cohesion < 0.05 (min 50 nodes) → re-split

### Hub Exclusion

- High-degree nodes excluded from partitioning
- Reattached by majority-vote of neighbors' communities

---

## God Nodes (analyze.py)

### `god_nodes(G, top_n=10)`

Returns top N most-connected real entities. Excludes:

- File-level hubs (label matches source filename, or starts with `.` and ends with `()`)
- Concept nodes (empty source_file, no file extension)
- JSON noise labels (start, end, name, id, type, etc.)
- Built-in noise (str, int, Path, Any, Optional, List, Dict, etc.)

---

## Surprising Connections (analyze.py)

### `surprising_connections(G, communities, top_n=5)`

Multi-file corpora: cross-file edges between real entities
Single-file corpora: cross-community edges that bridge distant graph parts

### `surprise_score()` Factors

| Factor | Score | Notes |
|---|---|---|
| Confidence weight | AMBIGUOUS=3, INFERRED=2, EXTRACTED=1 | Uncertain connections are more noteworthy |
| Cross file-type | +2 | code↔paper more surprising than code↔code |
| Cross-repo | +2 | Different top-level directory |
| Cross-community | +1 | Leiden says structurally distant |
| Peripheral→hub | +1 | Low-degree node reaching god node |

### Suppression Rules

- INFERRED calls/uses that cross language boundaries → suppressed
- INFERRED code→doc "calls" edges → suppressed (extraction artefacts)

---

## Suggested Questions (analyze.py)

### `suggest_questions(G, communities, community_labels, top_n=7)`

Generates questions from:

1. **AMBIGUOUS edges** → unresolved relationship questions
2. **Bridge nodes** (high betweenness) → cross-cutting concern questions
3. **God nodes with many INFERRED edges** → verification questions
4. **Isolated/weakly-connected nodes** → exploration questions
5. **Low-cohesion communities** → structural questions

---

## Graph Diff (analyze.py)

### `graph_diff(G_old, G_new)`

Compares two graph snapshots:

```json
{
  "new_nodes": [{"id": ..., "label": ...}],
  "removed_nodes": [{"id": ..., "label": ...}],
  "new_edges": [{"source": ..., "target": ..., "relation": ..., "confidence": ...}],
  "removed_edges": [...],
  "summary": "3 new nodes, 5 new edges, 1 node removed"
}
```

---

## Import Cycle Detection (analyze.py)

### `find_import_cycles(G, max_cycle_length=5, top_n=20)`

- Collapses symbol-level nodes to parent file
- Builds directed file-level graph from `imports_from` edges
- Finds simple cycles (shortest first)
- Deduplicates rotations

---

## Integration Plan for nano-brain

| Phase | What | Effort | Impact |
|---|---|---|---|
| **1** | Add `confidence` column to `edges` table | 1 day | Know what's extracted vs inferred |
| **2** | Add `gonum` Leiden community detection | 1 week | God nodes, cross-module surprises |
| **3** | Add `surprise_score()` logic | 2-3 days | Surface non-obvious connections |
| **4** | Add `suggest_questions()` logic | 2-3 days | Auto-generate codebase questions |
| **5** | Expose via MCP tools | 2-3 days | `memory_communities`, `memory_surprises` |

### Dependencies

- Phase 1: None
- Phase 2: `gonum.org/v1/gonum` (already in Go ecosystem)
- Phase 3: Depends on Phase 2 (needs community IDs)
- Phase 4: Depends on Phase 2 + 3
- Phase 5: Depends on Phase 2 + 3 + 4

### Prerequisites

- Resolve current higher-priority issues first
- Review existing `edges` table schema
- Decide on confidence column type (TEXT enum vs smallint)

---

## Reference: gonum Leiden Usage

```go
import "gonum.org/v1/gonum/graph/community"

reduced := community.Leiden(graph, 1.0, nil)
// reduced.Communities() → map of node IDs to community IDs
```

---

## Key Learnings

1. **Community detection is well-understood** — Leiden is the standard, gonum has it in Go
2. **Confidence tags are lightweight** — just add a column, minimal code change
3. **Surprise scoring is composite** — multiple factors, not just one metric
4. **Hub exclusion matters** — without it, utility nodes dominate god-node rankings
5. **Graph diff is useful for incremental updates** — compare old vs new snapshots
