## Context

nano-brain currently stores code symbols and edges in PostgreSQL, exposing them via MCP tools (`memory_graph`, `memory_impact`, `memory_trace`, `memory_symbols`). The graph is flat — nodes and edges without structure, trust signals, or priority ranking.

**Current state:**
- `symbols` table: id, name, kind, file_path, line_start, line_end, content_hash
- `edges` table: id, from_symbol_id, to_symbol_id, edge_type, source_file
- No community detection
- No confidence classification on edges
- No god node ranking
- No surprise scoring

**Constraints:**
- Go 1.23, CGO_ENABLED=0 (static binary)
- PostgreSQL 17 + pgvector 0.8.2
- Must not break existing MCP tools
- Performance: queries should stay <100ms for common operations

## Goals / Non-Goals

**Goals:**
- Add confidence tags (EXTRACTED, INFERRED, AMBIGUOUS) to every edge
- Detect communities via Leiden algorithm
- Rank god nodes (most-connected real entities)
- Score surprising connections (cross-community, cross-file-type, peripheral→hub)
- Generate suggested questions from graph structure
- Expose all new capabilities via MCP tools

**Non-Goals:**
- HTML visualization (future phase)
- PR dashboard (future phase)
- Multi-modal extraction (future phase)
- Real-time streaming updates (batch processing is fine)

## Decisions

### 1. Leiden over Louvain

**Choice:** Use `gonum.org/v1/gonum/graph/community.Leiden`

**Rationale:**
- Leiden guarantees well-connected communities (Louvain can produce disconnected ones)
- gonum is the canonical Go scientific library, battle-tested
- Both require undirected graphs — our edges are directed but community detection works on undirected

**Alternative considered:** Louvain via networkx — rejected because Leiden is provably better

### 2. Confidence column on edges table

**Choice:** Add `confidence TEXT NOT NULL DEFAULT 'EXTRACTED'` to edges table

**Rationale:**
- TEXT enum is simple, queryable, and extensible
- Default to EXTRACTED (existing edges are from AST extraction, not inference)
- Smallint enum was considered but adds complexity for minimal gain

**Alternative considered:** Separate `edge_confidence` table — rejected because 1:1 relationship adds JOIN overhead

### 3. Community ID on symbols table

**Choice:** Add `community_id INTEGER` to symbols table

**Rationale:**
- Pre-computed community membership avoids runtime Leiden calls on every query
- Re-computed on graph rebuild (via `graphify update` or similar)
- NULL means not yet computed or isolated node

**Alternative considered:** Separate `symbol_communities` table — rejected because 1:1 relationship adds JOIN overhead

### 4. Batch analysis on graph rebuild

**Choice:** Run community detection + god node ranking + surprise scoring as batch job after graph changes

**Rationale:**
- Leiden is O(n log n) — fast enough for batch, too slow for real-time
- Results are stable across runs (deterministic with seed=42)
- Incremental updates: only re-compute when edges change

**Alternative considered:** Real-time Leiden on every query — rejected because latency impact

### 5. Composite surprise score

**Choice:** Weighted combination of 5 factors (confidence, cross-file-type, cross-repo, cross-community, peripheral→hub)

**Rationale:**
- Mirrors Graphify's proven approach
- Each factor is cheap to compute
- Weights can be tuned based on observed results

**Alternative considered:** Single-factor scoring — rejected because no single factor captures "surprising" well

## Risks / Trade-offs

| Risk | Impact | Mitigation |
|---|---|---|
| Leiden on large graphs (>100K nodes) may be slow | Query latency | Run batch job async, cache results |
| Community detection may produce unstable results across runs | Agent confusion | Use deterministic seed (42), stable ID remapping |
| Confidence tags may be inaccurate for existing edges | Agent trust | Default to EXTRACTED, add INFERRED/AMBIGUOUS only for new edge types |
| Surprise score weights may need tuning | Suboptimal ranking | Start with Graphify's weights, adjust based on feedback |

## Migration Plan

1. Add `confidence` column to `edges` table (nullable initially)
2. Backfill existing edges as `EXTRACTED` (they're all from AST extraction)
3. Make column NOT NULL with DEFAULT 'EXTRACTED'
4. Add `community_id` column to `symbols` table
5. Add `gonum.org/v1/gonum` dependency
6. Implement community detection function
7. Implement god node ranking function
8. Implement surprise scoring function
9. Implement suggested questions function
10. Add MCP tools: `memory_communities`, `memory_surprises`, `memory_god_nodes`
11. Run initial batch analysis on existing graph

**Rollback:** Drop new columns, remove gonum dependency, remove MCP tools

## Open Questions

- Should community detection run on every graph update, or on-demand?
- What's the right threshold for "surprising" connections?
- Should we expose raw community IDs or community labels (like Graphify's named communities)?
- How often should we re-run batch analysis (on every commit? nightly? on-demand)?
