//go:build integration

package storage

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
)

func TestUpsertChunkPreservesEmbedStatus(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	q := sqlc.New(db)
	ctx := context.Background()

	workspaceHash := uuid.New().String()[:8]
	_, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: workspaceHash,
		Name: "test-workspace",
		Path: "/tmp/test",
	})
	if err != nil {
		t.Fatalf("UpsertWorkspace: %v", err)
	}

	contentHash := uuid.New().String()
	docRow, err := q.UpsertDocument(ctx, sqlc.UpsertDocumentParams{
		WorkspaceHash: workspaceHash,
		ContentHash:   contentHash,
		Title:         "test-file.txt",
		Content:       "Test content",
		SourcePath:    "test-file.txt",
		Collection:    "code",
	})
	if err != nil {
		t.Fatalf("UpsertDocument: %v", err)
	}
	docID := docRow.ID

	t.Run("new chunk gets pending status", func(t *testing.T) {
		chunkID, err := q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
			DocumentID:        docID,
			WorkspaceHash:     workspaceHash,
			ContentHash:       uuid.New().String(),
			Content:           "Test chunk content",
			ChunkIndex:        0,
			ChunkType:         "text",
			EmbeddingStrategy: "default",
		})
		if err != nil {
			t.Fatalf("UpsertChunk: %v", err)
		}

		chunk, err := q.GetChunkByID(ctx, chunkID)
		if err != nil {
			t.Fatalf("GetChunkByID: %v", err)
		}
		if chunk.EmbedStatus != "pending" {
			t.Errorf("Expected embed_status 'pending', got '%s'", chunk.EmbedStatus)
		}
	})

	t.Run("unchanged chunk preserves embed_status", func(t *testing.T) {
		content := "Test chunk content for preservation"
		contentHash := uuid.New().String()

		chunkID, err := q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
			DocumentID:        docID,
			WorkspaceHash:     workspaceHash,
			ContentHash:       contentHash,
			Content:           content,
			ChunkIndex:        1,
			ChunkType:         "text",
			EmbeddingStrategy: "default",
		})
		if err != nil {
			t.Fatalf("UpsertChunk first insert: %v", err)
		}

		err = q.MarkChunkEmbedded(ctx, sqlc.MarkChunkEmbeddedParams{
			ID:            chunkID,
			WorkspaceHash: workspaceHash,
		})
		if err != nil {
			t.Fatalf("MarkChunkEmbedded: %v", err)
		}

		chunk, err := q.GetChunkByID(ctx, chunkID)
		if err != nil {
			t.Fatalf("GetChunkByID: %v", err)
		}
		if chunk.EmbedStatus != "embedded" {
			t.Fatalf("Expected embed_status 'embedded', got '%s'", chunk.EmbedStatus)
		}

		_, err = q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
			DocumentID:        docID,
			WorkspaceHash:     workspaceHash,
			ContentHash:       contentHash,
			Content:           content,
			ChunkIndex:        1,
			ChunkType:         "text",
			EmbeddingStrategy: "default",
		})
		if err != nil {
			t.Fatalf("UpsertChunk second insert: %v", err)
		}

		chunk, err = q.GetChunkByID(ctx, chunkID)
		if err != nil {
			t.Fatalf("GetChunkByID: %v", err)
		}
		if chunk.EmbedStatus != "embedded" {
			t.Errorf("Expected embed_status 'embedded' after upsert with same content, got '%s'", chunk.EmbedStatus)
		}
	})

	t.Run("changed chunk resets embed_status", func(t *testing.T) {
		originalContent := "Original content"
		contentHash := uuid.New().String()

		chunkID, err := q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
			DocumentID:        docID,
			WorkspaceHash:     workspaceHash,
			ContentHash:       contentHash,
			Content:           originalContent,
			ChunkIndex:        2,
			ChunkType:         "text",
			EmbeddingStrategy: "default",
		})
		if err != nil {
			t.Fatalf("UpsertChunk first insert: %v", err)
		}

		err = q.MarkChunkEmbedded(ctx, sqlc.MarkChunkEmbeddedParams{
			ID:            chunkID,
			WorkspaceHash: workspaceHash,
		})
		if err != nil {
			t.Fatalf("MarkChunkEmbedded: %v", err)
		}

		chunk, err := q.GetChunkByID(ctx, chunkID)
		if err != nil {
			t.Fatalf("GetChunkByID: %v", err)
		}
		if chunk.EmbedStatus != "embedded" {
			t.Fatalf("Expected embed_status 'embedded', got '%s'", chunk.EmbedStatus)
		}

		_, err = q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
			DocumentID:        docID,
			WorkspaceHash:     workspaceHash,
			ContentHash:       contentHash,
			Content:           "Different content",
			ChunkIndex:        2,
			ChunkType:         "text",
			EmbeddingStrategy: "default",
		})
		if err != nil {
			t.Fatalf("UpsertChunk second insert: %v", err)
		}

		chunk, err = q.GetChunkByID(ctx, chunkID)
		if err != nil {
			t.Fatalf("GetChunkByID: %v", err)
		}
		if chunk.EmbedStatus != "pending" {
			t.Errorf("Expected embed_status 'pending' after upsert with different content, got '%s'", chunk.EmbedStatus)
		}
	})

	t.Run("failed chunk with unchanged content preserves status", func(t *testing.T) {
		content := "Failed chunk content"
		contentHash := uuid.New().String()

		chunkID, err := q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
			DocumentID:        docID,
			WorkspaceHash:     workspaceHash,
			ContentHash:       contentHash,
			Content:           content,
			ChunkIndex:        3,
			ChunkType:         "text",
			EmbeddingStrategy: "default",
		})
		if err != nil {
			t.Fatalf("UpsertChunk first insert: %v", err)
		}

		err = q.MarkChunkEmbedFailed(ctx, sqlc.MarkChunkEmbedFailedParams{
			ID:            chunkID,
			WorkspaceHash: workspaceHash,
		})
		if err != nil {
			t.Fatalf("MarkChunkEmbedFailed: %v", err)
		}

		chunk, err := q.GetChunkByID(ctx, chunkID)
		if err != nil {
			t.Fatalf("GetChunkByID: %v", err)
		}
		if chunk.EmbedStatus != "embed_failed" {
			t.Fatalf("Expected embed_status 'embed_failed', got '%s'", chunk.EmbedStatus)
		}

		_, err = q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
			DocumentID:        docID,
			WorkspaceHash:     workspaceHash,
			ContentHash:       contentHash,
			Content:           content,
			ChunkIndex:        3,
			ChunkType:         "text",
			EmbeddingStrategy: "default",
		})
		if err != nil {
			t.Fatalf("UpsertChunk second insert: %v", err)
		}

		chunk, err = q.GetChunkByID(ctx, chunkID)
		if err != nil {
			t.Fatalf("GetChunkByID: %v", err)
		}
		if chunk.EmbedStatus != "embed_failed" {
			t.Errorf("Expected embed_status 'embed_failed' after upsert with same content, got '%s'", chunk.EmbedStatus)
		}
	})
}
