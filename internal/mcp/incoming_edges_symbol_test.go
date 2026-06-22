//go:build integration

package mcp_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
)

func TestGetIncomingEdges_SymbolFallback(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })

	q := sqlc.New(db)
	wsHash := fmt.Sprintf("%x", sha256.Sum256([]byte("test_ws_"+uuid.New().String())))
	wsPath := "/tmp/test-ws-" + uuid.New().String()[:8]
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash, Name: "test-ws", Path: wsPath,
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	caller := wsPath + "/caller.go::handleRequest"
	targetFull := wsPath + "/target.go::ProcessData"
	targetSymbol := "ProcessData"

	if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
		WorkspaceHash: wsHash,
		SourceNode:    caller,
		TargetNode:    targetFull,
		EdgeType:      "calls",
		SourceFile:    "caller.go",
		Metadata:      []byte("{}"),
	}); err != nil {
		t.Fatalf("upsert edge: %v", err)
	}

	edges, err := q.GetIncomingEdges(ctx, sqlc.GetIncomingEdgesParams{
		WorkspaceHash: wsHash,
		TargetNode:    targetFull,
		Column3:       "",
	})
	if err != nil {
		t.Fatalf("GetIncomingEdges full path: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge for full path, got %d", len(edges))
	}

	edgesBySymbol, err := q.GetIncomingEdges(ctx, sqlc.GetIncomingEdgesParams{
		WorkspaceHash: wsHash,
		TargetNode:    targetSymbol,
		Column3:       "",
	})
	if err != nil {
		t.Fatalf("GetIncomingEdges symbol: %v", err)
	}
	if len(edgesBySymbol) != 1 {
		t.Fatalf("expected 1 edge for symbol-only lookup, got %d", len(edgesBySymbol))
	}
	if edgesBySymbol[0].SourceNode != caller {
		t.Errorf("expected source_node=%s, got %s", caller, edgesBySymbol[0].SourceNode)
	}
}
