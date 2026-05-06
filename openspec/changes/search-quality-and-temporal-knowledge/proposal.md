## Why

Search results are contaminated by documents from unrelated workspaces, dominated by large single files, and unable to distinguish current knowledge from superseded knowledge — causing agents to receive outdated or irrelevant context on every query.

## What Changes

- **Strict workspace isolation**: FTS and vector search filter to current workspace only; remove implicit `'global'` leakage
- **Qdrant payload filter**: Store `project_hash` as Qdrant point payload and push workspace filter server-side instead of post-filtering
- **Document length normalization**: Apply log-based length penalty after RRF fusion to prevent large files from dominating
- **Recency scoring**: Add time-decay boost for `sessions` and `memory` collections so recent knowledge ranks above stale
- **Temporal metadata in results**: Expose `createdAt`/`modifiedAt` on `SearchResult` so consuming agents can reason about recency
- **Supersede demotion fix**: Reduce superseded doc multiplier from `0.3` → `0.05` and fix bug where `supersedeDocument` passes `0` as new doc ID
- **Bayesian confidence decay**: Add `domain_type` and `last_reinforced_at` columns to documents; compute confidence at query time based on domain half-life

## Capabilities

### New Capabilities

- `workspace-isolated-search`: Search results are filtered to the active workspace at both FTS and vector layers; no cross-workspace contamination
- `temporal-scoring`: Results in `sessions` and `memory` collections receive a recency boost; older documents decay in ranking; `SearchResult` exposes date metadata
- `confidence-decay`: Documents carry a `domain_type` (tech-stack, process, preference, external, general) with configurable half-life; confidence computed at query time surfaces stale facts clearly

### Modified Capabilities

- `search`: RRF fusion pipeline gains length normalization penalty and recency boost post-processing step

## Impact

- `src/search.ts` — RRF fusion, recency boost, length penalty, supersede demotion factor
- `src/store/index.ts` — schema migration (add `domain_type`, `last_reinforced_at`); fix `supersedeDocument` bug; workspace filter changes
- `src/store/documents.ts` — add `createdAt`/`modifiedAt` to `SearchResult`; Qdrant payload upsert
- `src/store/vectors.ts` — pass `project_hash` payload on upsert; add payload filter to search
- `src/store/fts-worker.ts` — expose date columns in search output
- `src/types.ts` — extend `SearchResult` interface
- One-time Qdrant migration job to backfill `project_hash` payload on existing points
- `docker-compose.yml` — no changes needed
