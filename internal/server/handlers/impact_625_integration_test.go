//go:build integration

package handlers_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

func TestGraphImpact_NodeFoundIndependentOfEdgeFilter(t *testing.T) {
	q := newQueries(t)
	workspace := "ws_" + uuid.NewString()
	if err := q.UpsertGraphEdge(context.Background(), sqlc.UpsertGraphEdgeParams{
		WorkspaceHash: workspace,
		SourceNode:    "src/leaf.go::Leaf",
		TargetNode:    "outside",
		EdgeType:      "imports",
		SourceFile:    "src/leaf.go",
		Metadata:      []byte("{}"),
	}); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		node, edgeType string
		found          bool
	}{
		{"src/leaf.go::Leaf", "", true},
		{"outside", "calls", true},
		{"src/missing.go::Missing", "", false},
	} {
		t.Run(test.node+"-"+test.edgeType, func(t *testing.T) {
			got := callGraphImpact(t, q, workspace, test.node, test.edgeType, 1)
			if got["node_found"] != test.found {
				t.Errorf("node_found = %v, want %v; response=%v", got["node_found"], test.found, got)
			}
			if impacted, ok := got["impacted"].([]any); !ok || len(impacted) != 0 {
				t.Errorf("impacted = %v, want empty array", got["impacted"])
			}
		})
	}
}
