//go:build integration

package mcp_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

// Issue #546 E2: memory_flow auto-stitches a Bull queue producer
// (queue.add("jobName", data)) to its consumer (queue.process("jobName",
// handler)) by job-name, the same way #577/#578 linked Redis pub/sub — since
// both extractors emit the same "CONSUME <topic>" / Metadata["topic"] shape,
// flow.Stitch requires no changes for this producer/consumer pair.
func TestMemoryFlow_AutoStitchesBullQueueProducerConsumer(t *testing.T) {
	ctx, q, wsHash, callTool := setupFlowMCP(t)

	edges := []struct {
		src, tgt, kind, meta string
	}{
		// POST /schedule -> scheduleEmail (route handler)
		{"POST /schedule", "scheduleEmail", "http", `{"method":"POST","path":"/schedule"}`},
		// scheduleEmail enqueues a Bull job "emailJob" (queue.add)
		{"svc.js::scheduleEmail", "produce:emailJob", "integration", `{"kind":"queue_publish","method":"add","receiver":"mainQueue","topic":"emailJob"}`},
		// a worker consumes "emailJob" (queue.process, as the extractor now emits)
		{"CONSUME emailJob", "sendEmailWorker", "integration", `{"kind":"queue_consumer","method":"process","receiver":"mainQueue","topic":"emailJob"}`},
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
		"workspace": wsHash, "entry": "POST /schedule", "format": "json",
	}))
	if found, _ := resp["found"].(bool); !found {
		t.Fatalf("flow not found: %+v", resp)
	}

	var sawConsumer bool
	if nodes, ok := resp["nodes"].([]any); ok {
		for _, n := range nodes {
			if n.(map[string]any)["id"] == "CONSUME emailJob" {
				sawConsumer = true
			}
		}
	}
	if !sawConsumer {
		t.Fatalf("consumer entry not linked into the flow (auto-stitch failed): nodes=%+v", resp["nodes"])
	}

	var sawCrossService bool
	if es, ok := resp["edges"].([]any); ok {
		for _, e := range es {
			em := e.(map[string]any)
			if em["kind"] == "cross_service" {
				sawCrossService = true
				if from := em["from"].(string); from != "scheduleEmail" {
					t.Errorf("cross_service edge must originate from the in-flow node %q, got %q", "scheduleEmail", from)
				}
			}
		}
	}
	if !sawCrossService {
		t.Errorf("expected a cross_service edge linking producer to consumer: edges=%+v", resp["edges"])
	}
}
