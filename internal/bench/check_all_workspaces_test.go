package bench_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestCheckAllWorkspaces(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	
	workspaces := []struct {
		name string
		hash string
	}{
		{"nano-brain", "7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f"},
		{"capyhome", "37b36e2888c7106246b7345b75cfae285f0255cd48ed8756783c324c7fd2f81f"},
		{"zengamingx", "d1915ee19311546a064576fc5df565da7ab20fe1c4a81c97e3ba6e9059d977b7"},
		{"oh-my-harness-loop-plugin", "e9420166be63af587afee963d95f67d5c0bca7421d21c4d06535319a36478475"},
	}
	
	for _, ws := range workspaces {
		fmt.Printf("\n=== %s ===\n", ws.name)
		fmt.Printf("Hash: %s\n", ws.hash[:16]+"...")
		
		// Total documents
		var totalDocs int
		err = db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM documents WHERE workspace_hash = $1
		`, ws.hash).Scan(&totalDocs)
		if err != nil {
			t.Fatal(err)
		}
		
		// Total chunks
		var totalChunks int
		err = db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM chunks WHERE workspace_hash = $1
		`, ws.hash).Scan(&totalChunks)
		if err != nil {
			t.Fatal(err)
		}
		
		// Documents with embeddings
		var docsWithEmbeddings int
		err = db.QueryRowContext(ctx, `
			SELECT COUNT(DISTINCT d.id)
			FROM documents d
			INNER JOIN chunks c ON c.document_id = d.id
			INNER JOIN embeddings e ON e.chunk_id = c.id
			WHERE d.workspace_hash = $1
		`, ws.hash).Scan(&docsWithEmbeddings)
		if err != nil {
			t.Fatal(err)
		}
		
		// Total embeddings
		var totalEmbeddings int
		err = db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM embeddings WHERE workspace_hash = $1
		`, ws.hash).Scan(&totalEmbeddings)
		if err != nil {
			t.Fatal(err)
		}
		
		// Collections breakdown
		rows, err := db.QueryContext(ctx, `
			SELECT collection, COUNT(*) as cnt
			FROM documents
			WHERE workspace_hash = $1
			GROUP BY collection
			ORDER BY cnt DESC
		`, ws.hash)
		if err != nil {
			t.Fatal(err)
		}
		
		fmt.Printf("Documents: %d\n", totalDocs)
		fmt.Printf("Chunks: %d\n", totalChunks)
		fmt.Printf("Embeddings: %d\n", totalEmbeddings)
		fmt.Printf("Docs with embeddings: %d (%.1f%%)\n", docsWithEmbeddings, float64(docsWithEmbeddings)/float64(totalDocs)*100)
		fmt.Printf("\nCollections:\n")
		for rows.Next() {
			var collection string
			var cnt int
			if err := rows.Scan(&collection, &cnt); err != nil {
				t.Fatal(err)
			}
			fmt.Printf("  %s: %d\n", collection, cnt)
		}
		rows.Close()
	}
}
