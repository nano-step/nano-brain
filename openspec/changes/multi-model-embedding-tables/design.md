## Context

nano-brain stores all embeddings in a single PostgreSQL table `embeddings` with a `vector(768)` column, hardcoded in every sqlc query. The embedding provider and model are configurable (`config.embedding.provider`, `config.embedding.model`), but the storage table is not — switching from `nomic-embed-text` (768d) to `bge-m3` (1024d) requires dropping the existing table and re-embedding all chunks.

This design adds config-routable embedding table selection so the operator can:

- Keep existing `nomic-embed-text` embeddings untouched in the `embeddings` table.
- Switch to `bge-m3` embeddings stored in a new `embed_1024` table.
- Roll back by simply changing the config — no data loss.

### Affected modules

1. **`internal/config`** — add `table_name` field to `EmbeddingConfig`.
2. **`migrations/`** — add goose migration to create `embed_1024` table.
3. **`internal/storage/queries/embeddings.sql`** — add parallel queries targeting `embed_1024`.
4. **`internal/storage/sqlc/`** — regenerate (DO NOT EDIT).
5. **`internal/embed/queue.go`** — write to `config.Embedding.TableName`.
6. **`internal/search/service.go`** — query from `config.Embedding.TableName`.
7. **`internal/server/handlers/embed.go`** — pass table name to embed queue.
8. **`internal/mcp/tools.go`** — use configurable table for vsearch.
9. **Test files** — update mock `Querier` interfaces and table-driven tests.

## Goals / Non-Goals

**Goals:**

- Add `embedding.table_name` config field (default: `embeddings`) that routes both reads and writes to the correct embedding table.
- Create `embed_1024` table (1024d) via Goose migration, structurally identical to `embeddings`.
- Generate parallel sqlc queries for `embed_1024` table operations.
- Route embed queue writes to the configured table.
- Route vector search queries to the configured table.
- Keep existing `embeddings` table and data untouched.

**Non-Goals:**

- No API changes — REST, MCP, and CLI interfaces are unchanged.
- No changes to the embedding provider, model selection, or embedding concurrency.
- No automatic re-embedding — the operator must trigger `POST /api/v1/update` or `POST /api/v1/reindex` after switching config to populate the new table.
- No support for querying across multiple embedding tables simultaneously.
- No changes to BM25 search (it operates on `chunks.content`/`documents` tsvector, not embeddings).

## Decisions

### Decision 1: New sqlc queries per table (preferred) vs dynamic table names

The core challenge: sqlc generates static SQL with hardcoded table names. It does not support parameterized table names. The vector search queries (`VectorSearch`, `VectorSearchAll`, `VectorSearchWithTags`, `VectorSearchAllWithTags`) all reference `FROM embeddings` directly.

**Options considered:**

| Option | Approach | Pros | Cons |
|--------|----------|------|------|
| **A. Parallel queries** | Duplicate all 5 embedding-related queries in `embeddings_1024.sql` targeting `embed_1024` table. Regenerate sqlc. In Go layer, switch on `table_name`. | Type-safe. Follows existing patterns. Easy to code-review. | SQL duplication (~150 lines). |
| **B. Dynamic SQL helper** | Write a raw-SQL helper that uses `fmt.Sprintf` to interpolate table name, then calls `db.QueryContext` directly. | No duplication. Single code path. | Loses sqlc type safety. Bypasses codegen. Casts must be hand-written. |
| **C. PL/pgSQL wrapper** | Write a stored function `search_embeddings(table_name text, ...)` that uses `EXECUTE` to swap tables. | Single DB entry point. | Adds PL/pgSQL dependency. Harder to unit test. Not idiomatic Go. |

**Decision: Option A — Parallel sqlc queries.**

Rationale:
- nano-brain's architecture relies on sqlc for type safety. Introducing raw SQL for vector search would be inconsistent.
- The SQL duplication is mechanical and easy to generate/maintain. Both tables have identical columns — only the table name and embedding dimension change.
- The Go switch layer is thin: a single `selectTable(tableName string) querier` type that selects between `*Queries` for `embeddings` and `*Queries1024` for `embed_1024`.

### Decision 2: Interface-based table routing vs branching at call sites

The vector search methods are called from 4+ locations (search service, MCP tools, HTTP handlers, tests). Branching at every call site would be error-prone.

**Decision:** Define a `VectorQuerier` interface (already exists in `service.go` as `Querier`), then implement it with a struct that holds both `*sqlc.Queries` (768d) and `*sqlc.Queries1024` (1024d) and delegates to the right one based on config. Only the construction point needs to know about table name.

This keeps branching in one place and leaves all callers unchanged.

### Decision 3: Migration strategy for `embed_1024` table

The new table needs:
- `CREATE TABLE embed_1024 (LIKE embeddings INCLUDING ALL)` — but pgvector columns can't use `LIKE`. Must write explicit DDL.
- pgvector index: `CREATE INDEX ON embed_1024 USING hnsw (embedding vector_cosine_ops)`.
- No data migration — the table starts empty. Existing `embeddings` data stays.

**Constraints:**
- Migration 00024 (`00024_embed_1024_table.sql`).
- Must be reversible (`DROP TABLE IF EXISTS embed_1024 CASCADE`).
- Use `CONCURRENTLY` for index creation on production (but for an empty table, no lock contention concern).

### Decision 4: Config default and validation

- `embedding.table_name` defaults to `embeddings` (backward compatible).
- Validation: ensure `table_name` is non-empty and only contains allowed characters (`[a-z_]`).
- The config field is NOT validated at load time against actual DB tables — the table is created by migration, so if the migration ran, it exists.

### Decision 5: Embed queue writes

The embed queue's `InsertEmbedding` calls is already behind an interface (`EmbeddingQuerier` in `queue.go`). Add `InsertEmbedding1024` method to a new `EmbeddingQuerier1024` interface, or make the existing interface accept a table-routing parameter.

**Simplest approach:** `queue.Querier` interface already has `InsertEmbedding`. Add `InsertEmbedding1024` to a separate struct. Construct the embed queue with both query sets and delegate based on `cfg.Embedding.TableName`.

### Decision 6: What about test harnesses?

Tests use `mockQuerier` structs that implement the existing `Querier` interface. When we add the new 1024 queries, we need to either:

- Add `VectorSearchOn1024` etc. to the mock — breaks all existing tests.
- OR keep the `Querier` interface abstract and add a separate `Querier1024` interface that only the table-routing wrapper implements.

**Decision:** The table-routing wrapper implements the existing `Querier` interface. For the 1024 table, it internally calls `*Queries1024` methods. Existing mocks remain unchanged because the Querier interface doesn't change — only the concrete implementation behind it gains table-awareness.

## Risks / Trade-offs

| Risk | Impact | Mitigation |
|------|--------|------------|
| **sqlc regeneration breaks** if `sqlc.yaml` doesn't support two result types with the same column structure. | Build failure. | Verify: `sqlc generate` produces no errors. The Go types will be identical to the existing ones (same columns, different package receiver). Use a different Go type name suffix (e.g., `Embedding1024`). |
| **Forgetting to update all call sites** (search service, MCP, handlers) to use the routing wrapper. | Some code paths query the wrong table. | Single routing wrapper — all callers go through it. Only the construction point changes. |
| **embed `InsertEmbedding` hardcodes a single table** in its sqlc call. | Writes go to wrong table. | Separate the embed queue's querier into a table-aware wrapper, same pattern as search. |
| **Config change requires re-embed** before queries return results. | New table empty → vsearch returns 0 results. | Documented in migration plan. The operator runs `POST /api/v1/update` after switching config. |
| **pgvector dimension mismatch** — inserting 1024d vector into 768d column panics. | Hard crash at write time. | Config validation warn if `model` + `table_name` are mismatched (e.g., `bge-m3` + `embeddings`). Not enforced at startup but logged at WARN level. |

## Migration Plan

1. **Add `table_name` to `EmbeddingConfig`** with default `"embeddings"`.
2. **Create goose migration `00024_embed_1024_table.sql`**:
   - `CREATE TABLE embed_1024 (...)` — same columns as `embeddings` but `vector(1024)`.
   - `CREATE INDEX ON embed_1024 USING hnsw (embedding vector_cosine_ops)`.
   - Reversible.
3. **Add `embeddings_1024.sql` queries** to `internal/storage/queries/` — parallel to `embeddings.sql` targeting `embed_1024`.
4. **Run `sqlc generate`** to produce Go code.
5. **Create table-routing wrapper** in `internal/search/`:
   - `NewTableRouter(cfg *config.Config, queries768 *sqlc.Queries, queries1024 *sqlc.Queries1024) Querier`
   - Delegates to `queries768` if `cfg.Embedding.TableName == "embeddings"`, else `queries1024`.
6. **Create embed queue routing wrapper** — same pattern for `InsertEmbedding`.
7. **Wire routing wrappers** in `cmd/nano-brain/main.go` or wherever services are constructed.
8. **Update search service, MCP tools, HTTP handlers** — they already depend on the `Querier` interface. The routing wrapper is injected at construction; no code changes needed in callers.
9. **Update embed queue** to take the routing-enabled querier.
10. **Update default config** in `getDefaults()` and `.nano-brain/config.yml` to set `table_name: embed_1024` and `model: bge-m3`.
11. **Run tests** — `go build ./... && go test -race -short ./...`.
12. **Re-embed**: after deploy, hit `POST /api/v1/update` to re-embed all chunks into the new table.

**Rollback:** Change `table_name` back to `embeddings` and restart. Old embeddings are still there.

## Open Questions

1. Should the config validation WARN if `model` suggests 1024d but `table_name` points to a 768d table (or vice versa)? This is a best-effort check since the exact dimension is not always known at startup.
2. Should we name the new sqlc Go types `Embedding1024` (suffixed) or keep them `Embedding` (same as existing) in a separate package? Using a separate Go package for 1024-queries avoids type name collisions.
