//go:build integration

package mcp_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/sqlc-dev/pqtype"
)

func TestMemoryImpact_NodeFoundIndependentOfEdgeFilter(t *testing.T) {
	ctx, _, q, workspace, callTool := setupSearchMCP(t)
	if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
		WorkspaceHash: workspace,
		SourceNode:    "src/leaf.go::Leaf",
		TargetNode:    "outside",
		EdgeType:      "imports",
		SourceFile:    "src/leaf.go",
		Metadata:      []byte("{}"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
		WorkspaceHash: workspace,
		SourceNode:    "src/caller.go::Caller",
		TargetNode:    "BareOnly",
		EdgeType:      "calls",
		SourceFile:    "src/caller.go",
		Metadata:      []byte("{}"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: workspace,
		ContentHash:   "file-fixture",
		Title:         "leaf.go",
		Content:       "func NoEdges() {}",
		SourcePath:    "/tmp/" + workspace + "/src/leaf.go",
		Collection:    "code",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: workspace,
		ContentHash:   "symbol-fixture",
		Title:         "NoEdges",
		Content:       "func NoEdges() {}",
		SourcePath:    "/tmp/" + workspace + "/src/leaf.go?symbol=NoEdges&kind=function",
		Collection:    "code",
		Metadata:      pqtype.NullRawMessage{RawMessage: []byte(`{"source_type":"symbol"}`), Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: workspace,
		ContentHash:   "ghost-fixture",
		Title:         "Ghost",
		Content:       "func Ghost() {}",
		SourcePath:    "/tmp/" + workspace + "/src/gone.go?symbol=Ghost&kind=function",
		Collection:    "code",
		Metadata:      pqtype.NullRawMessage{RawMessage: []byte(`{"source_type":"symbol"}`), Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
	// The foreign-workspace edge needs its own registered workspace row now
	// that ensure_workspace_short_id no longer auto-creates one (issue #714).
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: "other-" + workspace, Name: "other-" + workspace, Path: "/tmp/other-" + workspace,
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
		WorkspaceHash: "other-" + workspace,
		SourceNode:    "foreign.go::OnlyHere",
		TargetNode:    "outside",
		EdgeType:      "calls",
		Metadata:      []byte("{}"),
	}); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		node, edgeType string
		found          bool
		candidates     int
	}{
		{"src/leaf.go::Leaf", "", true, 0},
		{"src/leaf.go::NoEdges", "", true, 0},
		{"src/gone.go::Ghost", "", false, 0},
		{"outside", "calls", true, 0},
		{"foreign.go::OnlyHere", "", false, 0},
		{"src/missing.go::Missing", "", false, 0},
		{"src/missing.go::BareOnly", "", false, 1},
	} {
		t.Run(test.node+"-"+test.edgeType, func(t *testing.T) {
			got := unmarshalGraphResp(t, callTool("memory_impact", map[string]any{
				"workspace": workspace, "node": test.node, "edge_type": test.edgeType,
			}))
			if got["node_found"] != test.found {
				t.Errorf("node_found = %v, want %v; response=%v", got["node_found"], test.found, got)
			}
			if impacted, ok := got["impacted"].([]any); !ok || len(impacted) != test.candidates {
				t.Errorf("impacted = %v, want %d bare-name candidates", got["impacted"], test.candidates)
			}
		})
	}
}
