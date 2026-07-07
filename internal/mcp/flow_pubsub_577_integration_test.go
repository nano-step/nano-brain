//go:build integration

package mcp_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

// Issue #577 (#546): memory_flow auto-stitches an in-workspace publisher to its
// consumer by topic, so a flow that publishes to a channel surfaces the handler
// that subscribes to it — without the caller passing stitch_workspaces.
func TestMemoryFlow_AutoStitchesInWorkspacePubSub(t *testing.T) {
	ctx, q, wsHash, callTool := setupFlowMCP(t)

	edges := []struct {
		src, tgt, kind, meta string
	}{
		// POST /trade -> createTrade (route handler)
		{"POST /trade", "createTrade", "http", `{"method":"POST","path":"/trade"}`},
		// createTrade publishes to "trade.created"
		{"svc.js::createTrade", "publish:trade.created", "integration", `{"kind":"cache_pubsub","topic":"trade.created"}`},
		// a handler consumes "trade.created" (CONSUME entry, as redis.subscribe now emits)
		{"CONSUME trade.created", "TradeCreatedHandler", "integration", `{"kind":"queue_consumer","topic":"trade.created"}`},
	}
	for _, e := range edges {
		if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
			WorkspaceHash: wsHash, SourceNode: e.src, TargetNode: e.tgt,
			EdgeType: e.kind, SourceFile: "svc.js", Metadata: []byte(e.meta),
		}); err != nil {
			t.Fatalf("upsert edge %+v: %v", e, err)
		}
	}

	resp := unmarshalGraphResp(t, callTool("memory_flow", map[string]any{
		"workspace": wsHash, "entry": "POST /trade", "format": "json",
	}))
	if found, _ := resp["found"].(bool); !found {
		t.Fatalf("flow not found: %+v", resp)
	}

	// The consumer side must appear in the flow, linked across the bus by topic —
	// without any stitch_workspaces argument. Stitch links the publisher to the
	// "CONSUME <topic>" consumer entry (the subscriber is one further hop).
	var sawConsumer bool
	if nodes, ok := resp["nodes"].([]any); ok {
		for _, n := range nodes {
			if n.(map[string]any)["id"] == "CONSUME trade.created" {
				sawConsumer = true
			}
		}
	}
	if !sawConsumer {
		t.Fatalf("consumer entry not linked into the flow (auto-stitch failed): nodes=%+v", resp["nodes"])
	}
	// ...and via a cross_service edge (not a normal call/http edge). The edge must
	// originate from the in-flow publisher node ("createTrade") — NOT the qualified
	// "svc.js::createTrade", which would be a duplicate, disconnected node splitting
	// the graph (the publisher endpoint is often qualified while the flow carries
	// only the bare symbol).
	var sawCrossService bool
	if es, ok := resp["edges"].([]any); ok {
		for _, e := range es {
			em := e.(map[string]any)
			if em["kind"] == "cross_service" {
				sawCrossService = true
				if from := em["from"].(string); from != "createTrade" {
					t.Errorf("cross_service edge must originate from the in-flow node %q, got %q (graph split)", "createTrade", from)
				}
			}
		}
	}
	if !sawCrossService {
		t.Errorf("expected a cross_service edge linking publisher to consumer: edges=%+v", resp["edges"])
	}

	// Well-formedness (guards the stitch-scoping fixes): every edge endpoint must
	// be a declared node (no dangling), and no self-edge (a consumer entry must
	// not be treated as a publisher and stitch to itself).
	nodeSet := map[string]bool{}
	if nodes, ok := resp["nodes"].([]any); ok {
		for _, n := range nodes {
			nodeSet[n.(map[string]any)["id"].(string)] = true
		}
	}
	if es, ok := resp["edges"].([]any); ok {
		for _, e := range es {
			em := e.(map[string]any)
			from, to := em["from"].(string), em["to"].(string)
			if from == to {
				t.Errorf("self-edge in flow: %q -> %q", from, to)
			}
			if !nodeSet[from] || !nodeSet[to] {
				t.Errorf("dangling edge endpoint not in nodes: %q -> %q", from, to)
			}
		}
	}
}
