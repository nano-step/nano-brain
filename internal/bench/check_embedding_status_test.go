package bench_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestCheckEmbeddingStatus(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	
	// Check embedding status by collection for nano-brain workspace
	rows, err := db.QueryContext(ctx, `
		SELECT 
			d.collection,
			COUNT(DISTINCT d.id) as total_docs,
			COUNT(DISTINCT CASE WHEN e.id IS NOT NULL THEN d.id END) as docs_with_embeddings
		FROM documents d
		LEFT JOIN chunks c ON c.document_id = d.id
		LEFT JOIN embeddings e ON e.chunk_id = c.id
		WHERE d.workspace_hash = '7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f'
		GROUP BY d.collection
		ORDER BY total_docs DESC
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	
	fmt.Println("=== nano-brain workspace: Embedding status by collection ===")
	fmt.Printf("%-20s %10s %15s %10s\n", "Collection", "Docs", "With Embedding", "Coverage")
	fmt.Println("------------------------------------------------------------")
	
	for rows.Next() {
		var collection string
		var totalDocs, docsWithEmbeddings int
		if err := rows.Scan(&collection, &totalDocs, &docsWithEmbeddings); err != nil {
			t.Fatal(err)
		}
		coverage := float64(docsWithEmbeddings) / float64(totalDocs) * 100
		fmt.Printf("%-20s %10d %15d %9.1f%%\n", collection, totalDocs, docsWithEmbeddings, coverage)
	}
	
	// Check if there are any chunks without embeddings
	var chunksWithoutEmbeddings int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM chunks c
		WHERE c.workspace_hash = '7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f'
		AND NOT EXISTS (
			SELECT 1 FROM embeddings e WHERE e.chunk_id = c.id
		)
	`).Scan(&chunksWithoutEmbeddings)
	if err != nil {
		t.Fatal(err)
	}
	
	fmt.Printf("\nChunks without embeddings: %d\n", chunksWithoutEmbeddings)
}
