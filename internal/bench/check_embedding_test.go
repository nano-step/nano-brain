package bench_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestCheckClosePoolEmbedding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db, err := sql.Open("pgx", "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	
	// Check if ClosePool document has embedding
	var docID, title string
	var hasEmbedding bool
	
	err = db.QueryRowContext(ctx, `
		SELECT d.id, d.title, EXISTS(SELECT 1 FROM embeddings e WHERE e.document_id = d.id) as has_embedding
		FROM documents d
		WHERE d.id = 'c5678c8e-b09b-47b4-8170-878debe5ddd1'
		AND d.workspace_hash = '7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f'
	`).Scan(&docID, &title, &hasEmbedding)
	
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	
	fmt.Printf("Document ID: %s\n", docID)
	fmt.Printf("Title: %s\n", title)
	fmt.Printf("Has embedding: %v\n", hasEmbedding)
	
	// Check embedding dimensions
	if hasEmbedding {
		var dims int
		err = db.QueryRowContext(ctx, `
			SELECT array_length(e.embedding, 1)
			FROM embeddings e
			WHERE e.document_id = 'c5678c8e-b09b-47b4-8170-878debe5ddd1'
		`).Scan(&dims)
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		fmt.Printf("Embedding dimensions: %d\n", dims)
	}
}
