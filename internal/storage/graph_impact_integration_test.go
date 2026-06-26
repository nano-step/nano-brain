//go:build integration

package storage_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
)

func TestGraphImpactQueriesMatchSymbolPart(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	dbConn := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { dbConn.Close() })
	db := sqlc.New(dbConn)
	ctx := context.Background()
	workspace := "graph-impact-symbol-part-test"
	if _, err := db.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: workspace,
		Name: "graph-impact-symbol-part-test",
		Path: "/tmp/graph-impact-symbol-part-test",
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	if err := db.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
		WorkspaceHash: workspace,
		SourceNode:    "app/workers/billing_worker.rb::BillingWorker#perform",
		TargetNode:    "app/models/story.rb::Story#create_print_orders",
		EdgeType:      "calls",
		SourceFile:    "app/workers/billing_worker.rb",
		Metadata:      []byte("{}"),
	}); err != nil {
		t.Fatalf("upsert edge: %v", err)
	}

	rows, err := db.GetImpactorsByTargets(ctx, sqlc.GetImpactorsByTargetsParams{
		WorkspaceHash: workspace,
		Column2:       []string{"Story#create_print_orders"},
		Column3:       "calls",
	})
	if err != nil {
		t.Fatalf("get impactors by targets: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].SourceNode != "app/workers/billing_worker.rb::BillingWorker#perform" {
		t.Fatalf("source = %q", rows[0].SourceNode)
	}

	one, err := db.GetImpactors(ctx, sqlc.GetImpactorsParams{
		WorkspaceHash: workspace,
		TargetNode:    "Story#create_print_orders",
		Column3:       "calls",
	})
	if err != nil {
		t.Fatalf("get impactors: %v", err)
	}
	if len(one) != 1 {
		t.Fatalf("single rows = %d, want 1", len(one))
	}
}
