//go:build integration

package embed

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/rs/zerolog"
)

func TestQueue_ScanByStatus_SkipsInflightChunks(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })

	_, err := db.ExecContext(ctx, "DELETE FROM chunks")
	if err != nil {
		t.Fatalf("clean chunks: %v", err)
	}

	queries := sqlc.New(db)

	wsHash := "test_ws_" + uuid.New().String()[:8]
	if _, err := queries.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash,
		Name: "test-ws",
		Path: "/tmp/test-" + wsHash,
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	docID := uuid.New()
	_, err = db.ExecContext(ctx, `INSERT INTO documents (id, workspace_hash, source_path, content_hash, collection)
		VALUES ($1, $2, $3, $4, $5)`,
		docID, wsHash, "test/file.go", "abc123", "code")
	if err != nil {
		t.Fatalf("insert document: %v", err)
	}

	chunkIDs := make([]uuid.UUID, 5)
	for i := range chunkIDs {
		chunkIDs[i] = uuid.New()
		_, err := db.ExecContext(ctx, `INSERT INTO chunks (id, document_id, workspace_hash, chunk_index, content, content_hash, embed_status)
			VALUES ($1, $2, $3, $4, $5, $6, 'pending')`,
			chunkIDs[i], docID, wsHash, i, "test content", uuid.New().String()[:8])
		if err != nil {
			t.Fatalf("insert chunk %d: %v", i, err)
		}
	}

	me := &mockEmbedder{}
	eq := NewQueue(me, queries, zerolog.Nop(), "test", "test", 1)

	eq.inflight.Store(chunkIDs[0], struct{}{})
	eq.inflight.Store(chunkIDs[1], struct{}{})

	total := eq.scanByStatus(ctx, false)

	if total != 3 {
		t.Errorf("scanByStatus enqueued %d, want 3 (should skip 2 inflight)", total)
	}
	if len(eq.ch) != 3 {
		t.Errorf("channel len = %d, want 3", len(eq.ch))
	}
}

func TestMigration_EmbedStatusIndex_Exists(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()

	var exists bool
	err := pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE indexname = 'idx_chunks_embed_status')",
	).Scan(&exists)
	if err != nil {
		t.Fatalf("query pg_indexes: %v", err)
	}
	if !exists {
		t.Error("idx_chunks_embed_status index does not exist after migration")
	}
}
