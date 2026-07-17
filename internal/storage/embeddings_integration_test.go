//go:build integration

package storage

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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

func TestInsertEmbedding_preventsForeignKeyRace_when_deleteStartsDuringPersistence(t *testing.T) {
	// Given
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	q := sqlc.New(db)
	workspaceHash := uuid.NewString()
	_, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: workspaceHash,
		Name: "embedding-lock-test",
		Path: "/tmp/embedding-lock-test",
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
	if _, err := pool.Exec(ctx, `
		CREATE FUNCTION block_embedding_insert() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			PERFORM pg_advisory_xact_lock(600);
			RETURN NEW;
		END;
		$$;
		CREATE TRIGGER block_embedding_insert
		BEFORE INSERT ON embeddings
		FOR EACH ROW EXECUTE FUNCTION block_embedding_insert();
	`); err != nil {
		t.Fatalf("create insert blocker: %v", err)
	}
	holder, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire advisory-lock holder: %v", err)
	}
	t.Cleanup(func() { holder.Release() })
	if _, err := holder.Exec(ctx, "SELECT pg_advisory_lock(600)"); err != nil {
		t.Fatalf("acquire advisory lock: %v", err)
	}
	t.Cleanup(func() {
		_, _ = holder.Exec(context.Background(), "SELECT pg_advisory_unlock(600)")
	})
	deleteConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire delete connection: %v", err)
	}
	t.Cleanup(func() { deleteConn.Release() })

	// When
	deadline, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	insertDone := make(chan error, 1)
	go func() {
		_, insertErr := q.InsertEmbedding(deadline, sqlc.InsertEmbeddingParams{
			ChunkID:       chunkID,
			WorkspaceHash: workspaceHash,
			Provider:      "test",
			Model:         "test-model",
			Embedding:     pgvector.NewVector(make([]float32, 768)),
		})
		insertDone <- insertErr
	}()

	for {
		var waiting bool
		if err := pool.QueryRow(deadline, `
			SELECT EXISTS (
				SELECT 1 FROM pg_locks
				WHERE locktype = 'advisory' AND objid = 600 AND NOT granted
			)`).Scan(&waiting); err != nil {
			t.Fatalf("check insert blocker: %v", err)
		}
		if waiting {
			break
		}
		select {
		case <-deadline.Done():
			t.Fatalf("InsertEmbedding did not reach its post-lock insert trigger: %v", deadline.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}

	deleteDone := make(chan error, 1)
	deletePID := deleteConn.Conn().PgConn().PID()
	go func() {
		_, deleteErr := deleteConn.Exec(deadline, "DELETE FROM chunks WHERE id = $1", chunkID)
		deleteDone <- deleteErr
	}()
	for {
		var waiting bool
		if err := pool.QueryRow(deadline, `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE pid = $1 AND wait_event_type = 'Lock'
			)`, deletePID).Scan(&waiting); err != nil {
			t.Fatalf("check delete lock: %v", err)
		}
		if waiting {
			break
		}
		select {
		case <-deadline.Done():
			t.Fatalf("DELETE did not block on InsertEmbedding's key-share lock: %v", deadline.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	if _, err := holder.Exec(ctx, "SELECT pg_advisory_unlock(600)"); err != nil {
		t.Fatalf("release insert blocker: %v", err)
	}

	// Then
	if err := <-insertDone; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			t.Fatalf("InsertEmbedding returned forbidden foreign-key violation: %v", err)
		}
		t.Fatalf("InsertEmbedding: %v", err)
	}
	if err := <-deleteDone; err != nil {
		t.Fatalf("delete chunk: %v", err)
	}
	count, err := q.CountEmbeddingsByWorkspace(ctx, workspaceHash)
	if err != nil {
		t.Fatalf("count embeddings: %v", err)
	}
	if count != 0 {
		t.Fatalf("embedding count after cascading delete = %d, want 0", count)
	}
}
