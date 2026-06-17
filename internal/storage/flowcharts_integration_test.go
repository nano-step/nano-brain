//go:build integration

package storage

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
)

func TestUpsertFunctionFlowchart_RoundTrip(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	q := sqlc.New(db)
	ctx := context.Background()

	workspaceHash := uuid.New().String()[:8]
	_, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: workspaceHash,
		Name: "test-workspace",
		Path: "/tmp/test",
	})
	if err != nil {
		t.Fatalf("UpsertWorkspace: %v", err)
	}

	cfg := graph.CFG{
		Entry:      "src/handlers.ts::purchase",
		SourceFile: "src/handlers.ts",
		StartLine:  15,
		EndLine:    48,
		Nodes: []graph.CFGNode{
			{ID: "n0", Type: "start", Label: "src/handlers.ts::purchase", Line: 15},
			{ID: "n1", Type: "decision", Label: "!req.id", Line: 16},
			{ID: "n2", Type: "terminal", Kind: "return", Label: "return res.status(400)", Line: 17},
			{ID: "n3", Type: "terminal", Kind: "return", Label: "return res.status(200)", Line: 19},
		},
		Edges: []graph.CFGEdge{
			{From: "n0", To: "n1", Branch: "next"},
			{From: "n1", To: "n2", Branch: "yes"},
			{From: "n1", To: "n3", Branch: "no"},
		},
		Status: "complete",
	}

	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	err = q.UpsertFunctionFlowchart(ctx, sqlc.UpsertFunctionFlowchartParams{
		WorkspaceHash: workspaceHash,
		Entry:         "src/handlers.ts::purchase",
		SourceFile:    "src/handlers.ts",
		StartLine:     15,
		EndLine:       48,
		Status:        "complete",
		Cfg:           cfgJSON,
	})
	if err != nil {
		t.Fatalf("UpsertFunctionFlowchart: %v", err)
	}

	fc, err := q.GetFunctionFlowchart(ctx, sqlc.GetFunctionFlowchartParams{
		WorkspaceHash: workspaceHash,
		SourceFile:    "src/handlers.ts",
		StartLine:     15,
		EndLine:       48,
	})
	if err != nil {
		t.Fatalf("GetFunctionFlowchart: %v", err)
	}

	if fc.Entry != "src/handlers.ts::purchase" {
		t.Errorf("Entry = %q, want %q", fc.Entry, "src/handlers.ts::purchase")
	}
	if fc.Status != "complete" {
		t.Errorf("Status = %q, want %q", fc.Status, "complete")
	}
	if fc.StartLine != 15 {
		t.Errorf("StartLine = %d, want 15", fc.StartLine)
	}
	if fc.EndLine != 48 {
		t.Errorf("EndLine = %d, want 48", fc.EndLine)
	}

	var gotCfg graph.CFG
	if err := json.Unmarshal(fc.Cfg, &gotCfg); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if len(gotCfg.Nodes) != 4 {
		t.Errorf("len(Nodes) = %d, want 4", len(gotCfg.Nodes))
	}
	if len(gotCfg.Edges) != 3 {
		t.Errorf("len(Edges) = %d, want 3", len(gotCfg.Edges))
	}
}

func TestUpsertFunctionFlowchart_Refresh(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	q := sqlc.New(db)
	ctx := context.Background()

	workspaceHash := uuid.New().String()[:8]
	_, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: workspaceHash,
		Name: "test-workspace",
		Path: "/tmp/test",
	})
	if err != nil {
		t.Fatalf("UpsertWorkspace: %v", err)
	}

	cfg1 := graph.CFG{
		Entry:      "src/handler.ts::foo",
		SourceFile: "src/handler.ts",
		StartLine:  10,
		EndLine:    20,
		Nodes:      []graph.CFGNode{{ID: "n0", Type: "start", Label: "foo", Line: 10}},
		Status:     "complete",
	}
	cfg1JSON, _ := json.Marshal(cfg1)

	err = q.UpsertFunctionFlowchart(ctx, sqlc.UpsertFunctionFlowchartParams{
		WorkspaceHash: workspaceHash,
		Entry:         "src/handler.ts::foo",
		SourceFile:    "src/handler.ts",
		StartLine:     10,
		EndLine:       20,
		Status:        "complete",
		Cfg:           cfg1JSON,
	})
	if err != nil {
		t.Fatalf("UpsertFunctionFlowchart (first): %v", err)
	}

	cfg2 := graph.CFG{
		Entry:      "src/handler.ts::foo",
		SourceFile: "src/handler.ts",
		StartLine:  10,
		EndLine:    25,
		Nodes: []graph.CFGNode{
			{ID: "n0", Type: "start", Label: "foo", Line: 10},
			{ID: "n1", Type: "step", Label: "bar()", Line: 15},
		},
		Status: "complete",
	}
	cfg2JSON, _ := json.Marshal(cfg2)

	err = q.UpsertFunctionFlowchart(ctx, sqlc.UpsertFunctionFlowchartParams{
		WorkspaceHash: workspaceHash,
		Entry:         "src/handler.ts::foo",
		SourceFile:    "src/handler.ts",
		StartLine:     10,
		EndLine:       25,
		Status:        "complete",
		Cfg:           cfg2JSON,
	})
	if err != nil {
		t.Fatalf("UpsertFunctionFlowchart (second): %v", err)
	}

	fc, err := q.GetFunctionFlowchart(ctx, sqlc.GetFunctionFlowchartParams{
		WorkspaceHash: workspaceHash,
		SourceFile:    "src/handler.ts",
		StartLine:     10,
		EndLine:       25,
	})
	if err != nil {
		t.Fatalf("GetFunctionFlowchart: %v", err)
	}

	var gotCfg graph.CFG
	if err := json.Unmarshal(fc.Cfg, &gotCfg); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if len(gotCfg.Nodes) != 2 {
		t.Errorf("len(Nodes) = %d, want 2 (after refresh)", len(gotCfg.Nodes))
	}
}

func TestDeleteFunctionFlowchartsByFile(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	q := sqlc.New(db)
	ctx := context.Background()

	workspaceHash := uuid.New().String()[:8]
	_, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: workspaceHash,
		Name: "test-workspace",
		Path: "/tmp/test",
	})
	if err != nil {
		t.Fatalf("UpsertWorkspace: %v", err)
	}

	cfg := graph.CFG{
		Entry:      "src/handler.ts::foo",
		SourceFile: "src/handler.ts",
		StartLine:  10,
		EndLine:    20,
		Nodes:      []graph.CFGNode{{ID: "n0", Type: "start", Label: "foo", Line: 10}},
		Status:     "complete",
	}
	cfgJSON, _ := json.Marshal(cfg)

	for _, startLine := range []int{10, 30, 50} {
		err = q.UpsertFunctionFlowchart(ctx, sqlc.UpsertFunctionFlowchartParams{
			WorkspaceHash: workspaceHash,
			Entry:         "src/handler.ts::foo",
			SourceFile:    "src/handler.ts",
			StartLine:     int32(startLine),
			EndLine:       int32(startLine + 10),
			Status:        "complete",
			Cfg:           cfgJSON,
		})
		if err != nil {
			t.Fatalf("UpsertFunctionFlowchart (line %d): %v", startLine, err)
		}
	}

	err = q.DeleteFunctionFlowchartsByFile(ctx, sqlc.DeleteFunctionFlowchartsByFileParams{
		WorkspaceHash: workspaceHash,
		SourceFile:    "src/handler.ts",
	})
	if err != nil {
		t.Fatalf("DeleteFunctionFlowchartsByFile: %v", err)
	}

	for _, startLine := range []int{10, 30, 50} {
		_, err = q.GetFunctionFlowchart(ctx, sqlc.GetFunctionFlowchartParams{
			WorkspaceHash: workspaceHash,
			SourceFile:    "src/handler.ts",
			StartLine:     int32(startLine),
			EndLine:       int32(startLine + 10),
		})
		if err == nil {
			t.Errorf("GetFunctionFlowchart (line %d) should have returned error after delete", startLine)
		}
	}
}

func TestGetFunctionFlowchartByHandler(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	q := sqlc.New(db)
	ctx := context.Background()

	workspaceHash := uuid.New().String()[:8]
	_, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: workspaceHash,
		Name: "test-workspace",
		Path: "/tmp/test",
	})
	if err != nil {
		t.Fatalf("UpsertWorkspace: %v", err)
	}

	cfg := graph.CFG{
		Entry:      "src/routes.ts::purchaseHandler",
		SourceFile: "src/routes.ts",
		StartLine:  15,
		EndLine:    48,
		Nodes:      []graph.CFGNode{{ID: "n0", Type: "start", Label: "purchaseHandler", Line: 15}},
		Status:     "complete",
	}
	cfgJSON, _ := json.Marshal(cfg)

	err = q.UpsertFunctionFlowchart(ctx, sqlc.UpsertFunctionFlowchartParams{
		WorkspaceHash: workspaceHash,
		Entry:         "src/routes.ts::purchaseHandler",
		SourceFile:    "src/routes.ts",
		StartLine:     15,
		EndLine:       48,
		Status:        "complete",
		Cfg:           cfgJSON,
	})
	if err != nil {
		t.Fatalf("UpsertFunctionFlowchart: %v", err)
	}

	fc, err := q.GetFunctionFlowchartByHandler(ctx, sqlc.GetFunctionFlowchartByHandlerParams{
		WorkspaceHash: workspaceHash,
		Entry:         "purchaseHandler",
	})
	if err != nil {
		t.Fatalf("GetFunctionFlowchartByHandler: %v", err)
	}

	if fc.Entry != "src/routes.ts::purchaseHandler" {
		t.Errorf("Entry = %q, want %q", fc.Entry, "src/routes.ts::purchaseHandler")
	}
}
