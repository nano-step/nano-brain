//go:build integration

package flow_test

import (
	"context"
	"database/sql"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/flow"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/rs/zerolog"
)

func setupDB(t *testing.T) (*sql.DB, *sqlc.Queries, string) {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })

	queries := sqlc.New(db)
	ctx := context.Background()

	wsHash := "test_flow_" + uuid.New().String()[:8]
	if _, err := queries.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash,
		Name: "test-flow-ws",
		Path: "/tmp/flow-test-" + wsHash,
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}
	return db, queries, wsHash
}

// insertEdge inserts a graph_edge row directly using raw SQL (avoids needing
// the full watcher stack).
func insertEdge(t *testing.T, db *sql.DB, wsHash, sourceNode, targetNode, edgeType, sourceFile string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO graph_edges (workspace_hash, source_node, target_node, edge_type, source_file)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT DO NOTHING`,
		wsHash, sourceNode, targetNode, edgeType, sourceFile)
	if err != nil {
		t.Fatalf("insertEdge %s->%s: %v", sourceNode, targetNode, err)
	}
}

func deleteEdge(t *testing.T, db *sql.DB, wsHash, sourceNode, targetNode, edgeType string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`DELETE FROM graph_edges WHERE workspace_hash=$1 AND source_node=$2 AND target_node=$3 AND edge_type=$4`,
		wsHash, sourceNode, targetNode, edgeType)
	if err != nil {
		t.Fatalf("deleteEdge %s->%s: %v", sourceNode, targetNode, err)
	}
}

func TestMaterializer_Integration_Upsert(t *testing.T) {
	db, queries, wsHash := setupDB(t)
	ctx := context.Background()

	// Seed: POST /api/topup --http--> TopupHandler --calls--> PaymentService
	insertEdge(t, db, wsHash, "POST /api/topup", "TopupHandler", "http", "routes.go")
	insertEdge(t, db, wsHash, "TopupHandler", "PaymentService", "calls", "handler.go")

	mat := flow.NewMaterializer(queries, nil, 5, 5, nil, zerolog.Nop())
	if err := mat.Materialize(ctx, wsHash); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	// Assert a flow document exists in collection "flows".
	doc, err := queries.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    "flow://POST /api/topup",
		WorkspaceHash: wsHash,
	})
	if err != nil {
		t.Fatalf("GetDocumentBySourcePath: %v", err)
	}
	if doc.Collection != "flows" {
		t.Errorf("collection = %q, want %q", doc.Collection, "flows")
	}
	if doc.Title != "POST /api/topup flow" {
		t.Errorf("title = %q, want %q", doc.Title, "POST /api/topup flow")
	}
	if doc.SourcePath != "flow://POST /api/topup" {
		t.Errorf("source_path = %q, want %q", doc.SourcePath, "flow://POST /api/topup")
	}
}

func TestMaterializer_Integration_DeleteStale(t *testing.T) {
	db, queries, wsHash := setupDB(t)
	ctx := context.Background()

	// Seed an http entry.
	insertEdge(t, db, wsHash, "POST /api/topup", "TopupHandler", "http", "routes.go")

	mat := flow.NewMaterializer(queries, nil, 5, 5, nil, zerolog.Nop())
	if err := mat.Materialize(ctx, wsHash); err != nil {
		t.Fatalf("Materialize (first): %v", err)
	}

	// Verify the doc exists.
	_, err := queries.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    "flow://POST /api/topup",
		WorkspaceHash: wsHash,
	})
	if err != nil {
		t.Fatalf("flow doc should exist after first Materialize: %v", err)
	}

	// Remove the http edge (route removed).
	deleteEdge(t, db, wsHash, "POST /api/topup", "TopupHandler", "http")

	// Re-materialize — stale doc should be deleted.
	if err := mat.Materialize(ctx, wsHash); err != nil {
		t.Fatalf("Materialize (second): %v", err)
	}

	_, err = queries.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    "flow://POST /api/topup",
		WorkspaceHash: wsHash,
	})
	if err == nil {
		t.Fatal("flow doc should have been deleted after route removal, but still exists")
	}
}

func TestMaterializer_Integration_EmbedEnqueue(t *testing.T) {
	db, queries, wsHash := setupDB(t)
	ctx := context.Background()

	insertEdge(t, db, wsHash, "GET /api/status", "StatusHandler", "http", "routes.go")

	var enqueuedCount atomic.Int32
	enqueueFn := func(_ uuid.UUID) { enqueuedCount.Add(1) }

	mat := flow.NewMaterializer(queries, enqueueFn, 5, 5, nil, zerolog.Nop())
	if err := mat.Materialize(ctx, wsHash); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	if enqueuedCount.Load() == 0 {
		t.Error("expected at least one chunk enqueued for embedding")
	}
}

func TestMaterializer_SingleFlight_Coalescing(t *testing.T) {
	_, queries, wsHash := setupDB(t)

	// No edges → Materialize is a lightweight no-op, good for concurrency test.
	mat := flow.NewMaterializer(queries, nil, 5, 5, nil, zerolog.Nop())

	ctx := context.Background()
	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Fire many Trigger calls concurrently. None should race or deadlock.
	for range goroutines {
		go func() {
			defer wg.Done()
			mat.Trigger(ctx, wsHash)
		}()
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
		// pass
	case <-time.After(10 * time.Second):
		t.Fatal("Trigger calls deadlocked or timed out")
	}
}

func TestMaterializer_Integration_ConsumerFlow(t *testing.T) {
	db, queries, wsHash := setupDB(t)
	ctx := context.Background()

	insertEdge(t, db, wsHash, "CONSUME trade.created", "TradeCreatedHandler", "integration", "handlers.go")
	insertEdge(t, db, wsHash, "TradeCreatedHandler", "TradeService", "calls", "handlers.go")

	mat := flow.NewMaterializer(queries, nil, 5, 5, nil, zerolog.Nop())
	if err := mat.Materialize(ctx, wsHash); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	doc, err := queries.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    "flow://CONSUME trade.created",
		WorkspaceHash: wsHash,
	})
	if err != nil {
		t.Fatalf("GetDocumentBySourcePath: %v", err)
	}
	if doc.Collection != "flows" {
		t.Errorf("collection = %q, want %q", doc.Collection, "flows")
	}
	if len(doc.Tags) != 2 || doc.Tags[0] != "flow" || doc.Tags[1] != "consumer" {
		t.Errorf("tags = %v, want [flow consumer]", doc.Tags)
	}
	if doc.Title != "CONSUME trade.created flow" {
		t.Errorf("title = %q, want %q", doc.Title, "CONSUME trade.created flow")
	}
}

// GetDocumentBySourcePath is called via the sqlc.Queries interface. We need
// the params struct — define a helper to avoid importing unexported types.
func init() {
	// Verify the interface is satisfied at compile time.
	var _ flow.MaterializerQuerier = (*sqlc.Queries)(nil)
}
