package graph_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func TestJSRedisPubSubEventRoles(t *testing.T) {
	extractor, err := graph.NewJSIntegrationExtractor()
	if err != nil {
		t.Fatal(err)
	}
	src := []byte(`
redis.publish("tradestate", payload)
socket.on("tradestate", (state) => io.emit("tradestate", state))
`)
	edges, err := extractor.ExtractEdges("socket.js", src)
	if err != nil {
		t.Fatal(err)
	}
	roles := map[string]bool{}
	for _, edge := range edges {
		if edge.Metadata == nil {
			continue
		}
		if role, ok := edge.Metadata["event_role"].(string); ok {
			roles[role] = true
		}
	}
	for _, role := range []string{"publish", "subscribe", "emit"} {
		if !roles[role] {
			t.Errorf("missing event role %q in extracted edges: %#v", role, edges)
		}
	}
}
