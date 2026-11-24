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
	// Graph upserts require a registered workspace since migration 00033
	// (issue #714) — ensure_workspace_short_id raises instead of auto-creating.
	if _, err := q.UpsertWorkspace(context.Background(), sqlc.UpsertWorkspaceParams{
		Hash: workspace, Name: workspace, Path: "/tmp/" + workspace,
	}); err != nil {
		t.Fatal(err)
	}
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
	if err := q.UpsertGraphEdge(context.Background(), sqlc.UpsertGraphEdgeParams{
		WorkspaceHash: workspace,
		SourceNode:    "src/caller.go::Caller",
		TargetNode:    "BareOnly",
		EdgeType:      "calls",
		SourceFile:    "src/caller.go",
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
		{"outside", "calls", true, 0},
		{"src/missing.go::Missing", "", false, 0},
		{"src/missing.go::BareOnly", "", false, 1},
	} {
		t.Run(test.node+"-"+test.edgeType, func(t *testing.T) {
			got := callGraphImpact(t, q, workspace, test.node, test.edgeType, 1)
			if got["node_found"] != test.found {
				t.Errorf("node_found = %v, want %v; response=%v", got["node_found"], test.found, got)
			}
			if impacted, ok := got["impacted"].([]any); !ok || len(impacted) != test.candidates {
				t.Errorf("impacted = %v, want %d bare-name candidates", got["impacted"], test.candidates)
			}
		})
	}
}
