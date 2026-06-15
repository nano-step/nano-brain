package bench_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/embed"
	"github.com/nano-brain/nano-brain/internal/search"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
	"os"
)

func TestDebugSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg, err := config.Load("/tmp/config-bench.yml")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	ctx := context.Background()
	db, err := sql.Open("pgx", cfg.Database.URL)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	embedder, err := embed.NewFromConfig(cfg.Embedding)
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}

	queries := sqlc.New(db)
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	svc := search.NewSearchService(queries, embedder, cfg.Search, logger)

	// Test with the first query from benchmark
	results, err := svc.HybridSearch(ctx, "ClosePool", "7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f", 10, nil, nil, "")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	fmt.Printf("Query: ClosePool\n")
	fmt.Printf("Expected doc ID: c5678c8e-b09b-47b4-8170-878debe5ddd1\n")
	fmt.Printf("Results count: %d\n", len(results))
	for i, r := range results {
		fmt.Printf("  %d. ID: %s, Score: %.4f, Title: %s\n", i+1, r.DocumentID, r.Score, r.Title)
	}
}
