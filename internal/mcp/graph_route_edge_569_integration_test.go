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
		{"POST /payment-intent", "createPaymentIntent", "http"},                        // bare target (as Express/Go extractors store it)
		{"GET /payment-intent", "payController.createPaymentIntent", "http"},           // controller-qualified target
		{"controllers/pay.js", "controllers/pay.js::createPaymentIntent", "contains"},  // qualified target
		{"other/unrelated.js::caller", "createPaymentIntent", "calls"},                 // bare cross-file JS calls must stay excluded
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

	var sawBareHTTP, sawCtrlHTTP, sawContains, sawBareJSCall bool
	edgesOut, _ := resp["edges"].([]any)
	for _, e := range edgesOut {
		em := e.(map[string]any)
		if em["edge_type"] == "http" && em["source"] == "POST /payment-intent" {
			sawBareHTTP = true
		}
		if em["edge_type"] == "http" && em["source"] == "GET /payment-intent" {
			sawCtrlHTTP = true
		}
		if em["edge_type"] == "contains" {
			sawContains = true
		}
		if em["edge_type"] == "calls" && em["source"] == "other/unrelated.js::caller" {
			sawBareJSCall = true
		}
	}
	if !sawBareHTTP {
		t.Errorf("incoming edges must include bare route->handler http edge (POST /payment-intent); got %+v", edgesOut)
	}
	if !sawCtrlHTTP {
		t.Errorf("incoming edges must include controller.method http edge (GET /payment-intent); got %+v", edgesOut)
	}
	if !sawContains {
		t.Errorf("incoming edges should still include the contains edge; got %+v", edgesOut)
	}
	if sawBareJSCall {
		t.Errorf("incoming edges must not leak bare cross-file JS calls edges; got %+v", edgesOut)
	}
}
