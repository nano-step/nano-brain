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
	}{
		{"src/leaf.go::Leaf", "", true},
		{"src/leaf.go::NoEdges", "", true},
		{"outside", "calls", true},
		{"foreign.go::OnlyHere", "", false},
		{"src/missing.go::Missing", "", false},
	} {
		t.Run(test.node+"-"+test.edgeType, func(t *testing.T) {
			got := unmarshalGraphResp(t, callTool("memory_impact", map[string]any{
				"workspace": workspace, "node": test.node, "edge_type": test.edgeType,
			}))
			if got["node_found"] != test.found {
				t.Errorf("node_found = %v, want %v; response=%v", got["node_found"], test.found, got)
			}
			if impacted, ok := got["impacted"].([]any); !ok || len(impacted) != 0 {
				t.Errorf("impacted = %v, want empty array", got["impacted"])
			}
		})
	}
}
