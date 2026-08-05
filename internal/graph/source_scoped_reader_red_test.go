package graph_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func TestComputePageRankExcludesUnresolvedCallTarget(t *testing.T) {
	// Given: an unresolved raw calls edge remains stored for diagnostics.
	edges := []graph.Edge{{
		SourceNode: "repo-a/consumer.ts::caller",
		TargetNode: "<unresolved>",
		Kind:       graph.EdgeCalls,
	}}

	// When
	scores := graph.ComputePageRank(edges, 0.85, 100, 1e-6)

	// Then: topology and degree/PageRank state must not create a shared hub.
	if _, ok := scores["<unresolved>"]; ok {
		t.Fatal("unresolved sentinel entered PageRank topology")
	}
}
