//go:build integration

package mcp_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

// Issue #569 (#542 F6): memory_graph(direction="in") on a route handler must
// surface the route->handler http edge, whose target is stored BARE, even when
// queried with the qualified "file::handler" node.
func TestMemoryGraph_IncomingSurfacesRouteHandlerHTTPEdge(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)

	edges := []struct{ src, tgt, kind string }{
		{"POST /payment-intent", "createPaymentIntent", "http"},                     // bare target (as extractors store it)
		{"controllers/pay.js", "controllers/pay.js::createPaymentIntent", "contains"}, // qualified target
	}
	for _, e := range edges {
		if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
			WorkspaceHash: wsHash, SourceNode: e.src, TargetNode: e.tgt,
			EdgeType: e.kind, SourceFile: "controllers/pay.js", Metadata: []byte("{}"),
		}); err != nil {
			t.Fatalf("upsert edge %+v: %v", e, err)
		}
	}

	resp := unmarshalGraphResp(t, callTool("memory_graph", map[string]any{
		"workspace": wsHash,
		"node":      "controllers/pay.js::createPaymentIntent", // qualified handler node
		"direction": "in",
	}))

	var sawHTTP, sawContains bool
	edgesOut, _ := resp["edges"].([]any)
	for _, e := range edgesOut {
		em := e.(map[string]any)
		if em["edge_type"] == "http" && em["source"] == "POST /payment-intent" {
			sawHTTP = true
		}
		if em["edge_type"] == "contains" {
			sawContains = true
		}
	}
	if !sawHTTP {
		t.Errorf("incoming edges must include the route->handler http edge (POST /payment-intent); got %+v", edgesOut)
	}
	if !sawContains {
		t.Errorf("incoming edges should still include the contains edge; got %+v", edgesOut)
	}
}
