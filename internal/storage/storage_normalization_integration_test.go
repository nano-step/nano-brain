//go:build integration

package storage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/nano-brain/nano-brain/migrations"
	pgvector "github.com/pgvector/pgvector-go"
	"github.com/pressly/goose/v3"
)

func schemaTotalBytes(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) int64 {
	t.Helper()
	var total int64
	err := pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(pg_total_relation_size(c.oid)), 0)::bigint
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relkind = 'r'
	`, schema).Scan(&total)
	if err != nil {
		t.Fatalf("measure schema size: %v", err)
	}
	return total
}

func TestMigration00030_MigratesLegacyV29DataAndShrinksStorage(t *testing.T) {
	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(testutil.TestDSN())
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}

	schema := fmt.Sprintf("test_m30_%x", sha256.Sum256([]byte(t.Name())))[:18]
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s, public", schema))
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})

	_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	goose.SetTableName(schema + ".goose_db_version")
	t.Cleanup(func() { goose.SetTableName("goose_db_version") })

	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })

	if err := goose.UpToContext(ctx, db, ".", 29); err != nil {
		t.Fatalf("migrate up to v29: %v", err)
	}

	worktrees := []string{
		strings.Repeat("a", 64),
		strings.Repeat("b", 64),
		strings.Repeat("c", 64),
	}
	for i, ws := range worktrees {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO workspaces (hash, name, path) VALUES ($1, $2, $3)`,
			ws, fmt.Sprintf("wt-%d", i), fmt.Sprintf("/data/workspaces/app-wt-%d", i)); err != nil {
			t.Fatalf("insert workspace: %v", err)
		}
	}

	baseVec := make([]float32, 768)
	for i := range baseVec {
		baseVec[i] = float32(i%17+1) * 0.01
	}

	const filesPerWorktree = 120
	for _, ws := range worktrees {
		for f := 0; f < filesPerWorktree; f++ {
			docID := uuid.New()
			chunkID := uuid.New()
			relPath := fmt.Sprintf("packages/service-billing/internal/handlers/checkout_handler_%03d.go", f)
			body := fmt.Sprintf("package handlers\n// CheckoutHandler%03d processes payment settlement and ledger entries.\n%s\n",
				f, strings.Repeat("func step() { validateOrderSettlementBalance() }\n", 50))
			cHash := fmt.Sprintf("%x", sha256.Sum256([]byte(body)))

			if _, err := db.ExecContext(ctx,
				`INSERT INTO documents (id, workspace_hash, content_hash, title, content, source_path, collection)
				 VALUES ($1, $2, $3, $4, $5, $6, 'code')`,
				docID, ws, cHash, fmt.Sprintf("checkout_handler_%03d.go", f), body, relPath); err != nil {
				t.Fatalf("seed v29 doc: %v", err)
			}
			if _, err := db.ExecContext(ctx,
				`INSERT INTO chunks (id, document_id, workspace_hash, content_hash, content, chunk_index, embed_status)
				 VALUES ($1, $2, $3, $4, $5, 0, 'embedded')`,
				chunkID, docID, ws, cHash, body); err != nil {
				t.Fatalf("seed v29 chunk: %v", err)
			}
			if _, err := db.ExecContext(ctx,
				`INSERT INTO embeddings (chunk_id, workspace_hash, provider, model, embedding)
				 VALUES ($1, $2, 'ollama', 'nomic-embed-text', $3)`,
				chunkID, ws, pgvector.NewVector(baseVec)); err != nil {
				t.Fatalf("seed v29 embedding: %v", err)
			}
			for e := 0; e < 6; e++ {
				if _, err := db.ExecContext(ctx,
					`INSERT INTO chunk_entities (chunk_id, entity_name, entity_type, workspace_hash)
					 VALUES ($1, $2, 'function', $3) ON CONFLICT DO NOTHING`,
					chunkID, fmt.Sprintf("validateOrderSettlementBalance_%d", e), ws); err != nil {
					t.Fatalf("seed v29 entity: %v", err)
				}
				if _, err := db.ExecContext(ctx,
					`INSERT INTO graph_edges (workspace_hash, source_node, target_node, edge_type, source_file)
					 VALUES ($1, $2, $3, 'calls', $4) ON CONFLICT DO NOTHING`,
					ws,
					fmt.Sprintf("%s::CheckoutHandler%03d", relPath, f),
					fmt.Sprintf("packages/service-billing/internal/ledger/service.go::SettleOrder%d", e),
					relPath); err != nil {
					t.Fatalf("seed v29 edge: %v", err)
				}
			}
		}
	}

	v29Bytes := schemaTotalBytes(t, ctx, pool, schema)

	if err := goose.UpToContext(ctx, db, ".", 30); err != nil {
		t.Fatalf("migrate v29 -> v30: %v", err)
	}

	v30Bytes := schemaTotalBytes(t, ctx, pool, schema)
	reductionPct := 100.0 * (1.0 - float64(v30Bytes)/float64(v29Bytes))
	t.Logf("Storage footprint: v29=%d KB -> v30=%d KB (reduction=%.1f%%)", v29Bytes/1024, v30Bytes/1024, reductionPct)

	if reductionPct < 48.0 {
		t.Errorf("expected >= 48%% storage reduction on 360-file micro-corpus after v30 normalization, got %.1f%% (v29=%d, v30=%d)",
			reductionPct, v29Bytes, v30Bytes)
	}

	q := sqlc.New(db)
	ws0 := worktrees[0]

	doc, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    "packages/service-billing/internal/handlers/checkout_handler_000.go",
		WorkspaceHash: ws0,
	})
	if err != nil || !strings.Contains(doc.Content, "CheckoutHandler000") {
		t.Fatalf("GetDocumentBySourcePath after migration failed: err=%v len=%d", err, len(doc.Content))
	}

	bm25Hits, err := q.BM25Search(ctx, sqlc.BM25SearchParams{
		WorkspaceHash: ws0,
		Query:         "CheckoutHandler005 settlement",
		MaxResults:    5,
	})
	if err != nil || len(bm25Hits) == 0 {
		t.Fatalf("BM25Search after migration failed: err=%v hits=%d", err, len(bm25Hits))
	}

	vecHits, err := q.VectorSearch(ctx, sqlc.VectorSearchParams{
		WorkspaceHash:  ws0,
		QueryEmbedding: pgvector.NewVector(baseVec),
		MaxResults:     5,
	})
	if err != nil || len(vecHits) != 5 {
		t.Fatalf("VectorSearch on halfvec(768) after migration failed: err=%v hits=%d", err, len(vecHits))
	}

	impactors, err := q.GetImpactorsByTargets(ctx, sqlc.GetImpactorsByTargetsParams{
		WorkspaceHash: ws0,
		Column2:       []string{"SettleOrder0"},
		Column3:       "calls",
	})
	if err != nil || len(impactors) != filesPerWorktree {
		t.Fatalf("GetImpactorsByTargets after migration = %d (err=%v), want %d", len(impactors), err, filesPerWorktree)
	}

	entityChunks, err := q.GetChunkIDsByEntityNames(ctx, sqlc.GetChunkIDsByEntityNamesParams{
		WorkspaceHash: ws0,
		Column2:       []string{"validateOrderSettlementBalance_0"},
	})
	if err != nil || len(entityChunks) != filesPerWorktree {
		t.Fatalf("GetChunkIDsByEntityNames after migration = %d (err=%v), want %d", len(entityChunks), err, filesPerWorktree)
	}

	// Verify ReuseEmbeddingIfCached links an identical chunk in a 4th worktree with 0 embedder calls.
	ws4 := strings.Repeat("d", 64)
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{Hash: ws4, Name: "wt-4", Path: "/data/workspaces/app-wt-4"}); err != nil {
		t.Fatalf("upsert ws4: %v", err)
	}
	dupBody := doc.Content
	newDoc, err := q.UpsertDocument(ctx, sqlc.UpsertDocumentParams{
		WorkspaceHash: ws4,
		ContentHash:   doc.ContentHash,
		Title:         doc.Title,
		Content:       dupBody,
		SourcePath:    doc.SourcePath,
		Collection:    "code",
	})
	if err != nil {
		t.Fatalf("upsert dup doc: %v", err)
	}
	newChunkID, err := q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
		DocumentID:        newDoc.ID,
		WorkspaceHash:     ws4,
		ContentHash:       doc.ContentHash,
		Content:           dupBody,
		ChunkType:         "raw",
		EmbeddingStrategy: "raw_code",
	})
	if err != nil {
		t.Fatalf("upsert dup chunk: %v", err)
	}
	linkedID, err := q.ReuseEmbeddingIfCached(ctx, sqlc.ReuseEmbeddingIfCachedParams{
		ChunkID:       newChunkID,
		WorkspaceHash: ws4,
		Provider:      "ollama",
		Model:         "nomic-embed-text",
	})
	if err != nil || linkedID != newChunkID {
		t.Fatalf("ReuseEmbeddingIfCached = %v, %v; want %v, nil", linkedID, err, newChunkID)
	}
}
