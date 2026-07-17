//go:build integration

package storage

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	pgvector "github.com/pgvector/pgvector-go"
)

func TestInsertEmbedding_returnsNoRows_when_sourceChunkWasDeleted(t *testing.T) {
	// Given
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	q := sqlc.New(db)
	workspaceHash := uuid.NewString()
	_, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: workspaceHash,
		Name: "embedding-race-test",
		Path: "/tmp/embedding-race-test",
	})
	if err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}
	document, err := q.UpsertDocument(ctx, sqlc.UpsertDocumentParams{
		WorkspaceHash: workspaceHash,
		ContentHash:   uuid.NewString(),
		Title:         "source.txt",
		Content:       "source content",
		SourcePath:    "source.txt",
		Collection:    "code",
	})
	if err != nil {
		t.Fatalf("upsert document: %v", err)
	}
	chunkID, err := q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
		DocumentID:        document.ID,
		WorkspaceHash:     workspaceHash,
		ContentHash:       uuid.NewString(),
		Content:           "chunk content",
		ChunkType:         "text",
		EmbeddingStrategy: "default",
	})
	if err != nil {
		t.Fatalf("upsert chunk: %v", err)
	}
	if err := q.DeleteChunksByIDs(ctx, []uuid.UUID{chunkID}); err != nil {
		t.Fatalf("delete source chunk: %v", err)
	}

	// When
	_, err = q.InsertEmbedding(ctx, sqlc.InsertEmbeddingParams{
		ChunkID:       chunkID,
		WorkspaceHash: workspaceHash,
		Provider:      "test",
		Model:         "test-model",
		Embedding:     pgvector.NewVector(make([]float32, 768)),
	})

	// Then
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("InsertEmbedding error = %v, want sql.ErrNoRows", err)
	}
	count, err := q.CountEmbeddingsByWorkspace(ctx, workspaceHash)
	if err != nil {
		t.Fatalf("count embeddings: %v", err)
	}
	if count != 0 {
		t.Fatalf("embedding count = %d, want 0", count)
	}
}
