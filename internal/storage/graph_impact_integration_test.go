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

func TestSourceScopedReaderQueriesKeepCanonicalExactAndDropUnresolved(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	dbConn := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { dbConn.Close() })
	db := sqlc.New(dbConn)
	ctx := context.Background()
	workspace := "source-scoped-reader-query-test"
	if _, err := db.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: workspace,
		Name: workspace,
		Path: "/tmp/" + workspace,
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	for _, edge := range []sqlc.UpsertGraphEdgeParams{
		{
			WorkspaceHash: workspace,
			SourceNode:    "repo-a/consumer.ts::caller",
			TargetNode:    "repo-a/lib/api.ts::run",
			EdgeType:      "calls",
			SourceFile:    "repo-a/consumer.ts",
			Metadata:      []byte("{}"),
		},
		{
			WorkspaceHash: workspace,
			SourceNode:    "repo-a/consumer.ts::caller",
			TargetNode:    "run",
			EdgeType:      "calls",
			SourceFile:    "repo-a/consumer.ts",
			Metadata:      []byte("{}"),
		},
		{
			WorkspaceHash: workspace,
			SourceNode:    "repo-b/consumer.ts::caller",
			TargetNode:    "repo-b/lib/api.ts::run",
			EdgeType:      "calls",
			SourceFile:    "repo-b/consumer.ts",
			Metadata:      []byte("{}"),
		},
		{
			WorkspaceHash: workspace,
			SourceNode:    "repo-a/consumer.ts::caller",
			TargetNode:    "<unresolved>",
			EdgeType:      "calls",
			SourceFile:    "repo-a/consumer.ts",
			Metadata:      []byte("{}"),
		},
	} {
		if err := db.UpsertGraphEdge(ctx, edge); err != nil {
			t.Fatalf("upsert graph edge %q: %v", edge.TargetNode, err)
		}
	}

	impact, err := db.GetImpactorsByTargets(ctx, sqlc.GetImpactorsByTargetsParams{
		WorkspaceHash: workspace,
		Column2:       []string{"repo-a/lib/api.ts::run"},
		Column3:       "calls",
	})
	if err != nil {
		t.Fatalf("GetImpactorsByTargets: %v", err)
	}
	if len(impact) != 1 || impact[0].TargetNode != "repo-a/lib/api.ts::run" {
		t.Fatalf("canonical impact rows = %#v, want exact canonical row only", impact)
	}

	legacyImpact, err := db.GetImpactorsByTargets(ctx, sqlc.GetImpactorsByTargetsParams{
		WorkspaceHash: workspace,
		Column2:       []string{"run"},
		Column3:       "calls",
	})
	if err != nil {
		t.Fatalf("GetImpactorsByTargets for bare symbol: %v", err)
	}
	if len(legacyImpact) != 1 || legacyImpact[0].TargetNode != "run" {
		t.Fatalf("bare-symbol impact rows = %#v, want legacy target only", legacyImpact)
	}

	legacySingleImpact, err := db.GetImpactors(ctx, sqlc.GetImpactorsParams{
		WorkspaceHash: workspace,
		TargetNode:    "run",
		Column3:       "calls",
	})
	if err != nil {
		t.Fatalf("GetImpactors for bare symbol: %v", err)
	}
	if len(legacySingleImpact) != 1 {
		t.Fatalf("bare-symbol single impact rows = %d, want 1", len(legacySingleImpact))
	}

	callEdges, err := db.ListCallEdges(ctx, workspace)
	if err != nil {
		t.Fatalf("ListCallEdges: %v", err)
	}
	if len(callEdges) != 3 {
		t.Fatalf("ListCallEdges returned %d rows, want 3 non-sentinel calls", len(callEdges))
	}
	for _, edge := range callEdges {
		if edge.TargetNode == "<unresolved>" || edge.SourceNode == "<unresolved>" {
			t.Fatalf("unresolved edge reached PageRank input: %#v", edge)
		}
	}

	count, err := db.CountCallEdges(ctx, workspace)
	if err != nil {
		t.Fatalf("CountCallEdges: %v", err)
	}
	if count != 3 {
		t.Fatalf("CountCallEdges = %d, want 3", count)
	}

	outgoing, err := db.GetOutgoingEdgesBySymbol(ctx, sqlc.GetOutgoingEdgesBySymbolParams{
		WorkspaceHash: workspace,
		SourceNode:    "repo-a/consumer.ts::caller",
		Column3:       "calls",
	})
	if err != nil {
		t.Fatalf("GetOutgoingEdgesBySymbol: %v", err)
	}
	if len(outgoing) != 2 {
		t.Fatalf("canonical outgoing rows = %d, want 2 (exact source plus legacy row)", len(outgoing))
	}
	for _, row := range outgoing {
		if row.SourceNode != "repo-a/consumer.ts::caller" {
			t.Fatalf("canonical outgoing crossed source scope: %#v", row)
		}
	}

	stats, err := db.GraphStats(ctx, workspace)
	if err != nil {
		t.Fatalf("GraphStats: %v", err)
	}
	if stats.CallsCount != 3 {
		t.Fatalf("GraphStats.CallsCount = %d, want 3", stats.CallsCount)
	}
}
