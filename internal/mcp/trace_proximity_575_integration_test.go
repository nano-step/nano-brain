//go:build integration

package mcp_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

// Issue #575 (#542 F2): a bare call that collides across sub-repos must resolve
// to the definition nearest the caller, not fan out to every same-named symbol.
func TestMemoryTrace_ResolvesBareCallToNearestSubtree(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)

	// Main (backend) calls bare "foo"; "foo" is defined in both backend and frontend.
	upsertSymbolDoc(t, ctx, q, wsHash, "backend/svc.go", "foo", "function", "func foo() {}", "1", "1")
	upsertSymbolDoc(t, ctx, q, wsHash, "frontend/util.js", "foo", "function", "function foo() {}", "1", "1")
	if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
		WorkspaceHash: wsHash,
		SourceNode:    "backend/ctrl.go::Main",
		TargetNode:    "foo", // bare calls target
		EdgeType:      "calls",
		SourceFile:    "backend/ctrl.go", // the caller file — drives proximity
		Metadata:      []byte("{}"),
	}); err != nil {
		t.Fatalf("upsert edge: %v", err)
	}

	resp := unmarshalGraphResp(t, callTool("memory_trace", map[string]any{
		"workspace": wsHash,
		"node":      "backend/ctrl.go::Main",
		"max_depth": float64(2),
		"paths":     "relative",
	}))
	chain, _ := resp["chain"].([]any)

	var fooNodes []string
	ambiguousSeen := false
	for _, c := range chain {
		cm := c.(map[string]any)
		if cm["name"] == "foo" {
			fooNodes = append(fooNodes, cm["node"].(string))
			if amb, _ := cm["ambiguous"].(bool); amb {
				ambiguousSeen = true
			}
		}
	}
	if len(fooNodes) != 1 {
		t.Fatalf("expected exactly 1 'foo' node (nearest subtree), got %v", fooNodes)
	}
	if fooNodes[0] != "backend/svc.go::foo" {
		t.Errorf("resolved foo = %q, want backend/svc.go::foo (frontend should be dropped)", fooNodes[0])
	}
	if ambiguousSeen {
		t.Errorf("uniquely-resolved call should not be flagged ambiguous: %+v", chain)
	}
}
