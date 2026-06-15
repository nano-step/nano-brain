package bench_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestCheckSchema(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	
	// Check embeddings table schema
	rows, err := db.QueryContext(ctx, "SELECT column_name, data_type FROM information_schema.columns WHERE table_name = 'embeddings' ORDER BY ordinal_position")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	
	fmt.Println("Embeddings table columns:")
	for rows.Next() {
		var colName, dataType string
		if err := rows.Scan(&colName, &dataType); err != nil {
			t.Fatal(err)
		}
		fmt.Printf("  %s: %s\n", colName, dataType)
	}
	
	// Check if ClosePool has embedding
	var count int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM embeddings 
		WHERE chunk_id IN (
			SELECT id FROM chunks WHERE document_id = 'c5678c8e-b09b-47b4-8170-878debe5ddd1'
		)
	`).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("\nClosePool embeddings count: %d\n", count)
}
