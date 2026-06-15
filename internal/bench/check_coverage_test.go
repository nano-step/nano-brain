package bench_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestCheckEmbeddingCoverage(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	
	// Count documents with and without embeddings
	var totalDocs, docsWithEmbeddings int
	
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT d.id)
		FROM documents d
		WHERE d.workspace_hash = '7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f'
	`).Scan(&totalDocs)
	if err != nil {
		t.Fatal(err)
	}
	
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT d.id)
		FROM documents d
		INNER JOIN chunks c ON c.document_id = d.id
		INNER JOIN embeddings e ON e.chunk_id = c.id
		WHERE d.workspace_hash = '7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f'
	`).Scan(&docsWithEmbeddings)
	if err != nil {
		t.Fatal(err)
	}
	
	fmt.Printf("Total documents: %d\n", totalDocs)
	fmt.Printf("Documents with embeddings: %d\n", docsWithEmbeddings)
	fmt.Printf("Coverage: %.1f%%\n", float64(docsWithEmbeddings)/float64(totalDocs)*100)
	
	// Check benchmark dataset documents
	benchmarkDocIDs := []string{
		"c5678c8e-b09b-47b4-8170-878debe5ddd1",
		"b3434f87-1189-4027-9f04-a005f44d5e8e",
		"e519e4e6-8a12-4155-8b9e-9d7f448f9b36",
		"812f1ac3-0153-40e7-9c89-31193f54c752",
		"dd8665f1-8b03-4c1d-829e-58f677f44891",
	}
	
	fmt.Println("\nBenchmark dataset document embedding status:")
	for _, docID := range benchmarkDocIDs {
		var hasEmbedding bool
		err = db.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM chunks c
				INNER JOIN embeddings e ON e.chunk_id = c.id
				WHERE c.document_id = $1
			)
		`, docID).Scan(&hasEmbedding)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("  %s: %v\n", docID, hasEmbedding)
	}
}
