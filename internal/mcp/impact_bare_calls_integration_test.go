//go:build integration

package mcp_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

// --- Issue #553: memory_impact direction:"in" misses calls-edge callers ---
//
// calls-edge targets are stored bare (e.g. "checkAccess"), not "file::symbol".
// Before the fix, registerMemoryImpact's "in" frontier only ever contained the
// caller-supplied qualified node, so GetImpactorsByTargets's
// target_node = ANY($2) never matched a bare-stored calls target and
// memory_impact(direction:"in") returned count:0 for real callers.

// AC-A1: a direct caller of a bare-target calls edge is returned.
func TestMemoryImpact_BareCallsTarget_DirectCallerReturned(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)

	if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
		WorkspaceHash: wsHash,
		SourceNode:    "a.go::A",
		TargetNode:    "B", // bare, as calls-edge extractors write it
		EdgeType:      "calls",
		SourceFile:    "a.go",
		Metadata:      []byte("{}"),
	}); err != nil {
		t.Fatalf("upsert edge: %v", err)
	}

	result := callTool("memory_impact", map[string]any{
		"workspace": wsHash,
		"node":      "b.go::B",
		"direction": "in",
	})
	resp := unmarshalGraphResp(t, result)
	impacted, _ := resp["impacted"].([]any)
	if len(impacted) != 1 {
		t.Fatalf("impacted count = %d, want 1 (caller A): %+v", len(impacted), impacted)
	}
	item := impacted[0].(map[string]any)
	if item["node"].(string) != "a.go::A" {
		t.Errorf("impacted[0].node = %q, want a.go::A", item["node"])
	}
	if got := int(resp["count"].(float64)); got != 1 {
		t.Errorf("count = %d, want 1", got)
	}
}

// AC-A2: transitive callers past depth 1 are also resolved — the bare-suffix
// seeding must apply to every "next" frontier batch built during the depth
// loop, not just the initial frontier.
func TestMemoryImpact_BareCallsTarget_TransitiveCallersAtDepth(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)

	// A calls B, B calls C — both calls-edge targets stored bare.
	edges := []struct{ source, target string }{
		{"a.go::A", "B"},
		{"b.go::B", "C"},
	}
	for _, e := range edges {
		if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
			WorkspaceHash: wsHash,
			SourceNode:    e.source,
			TargetNode:    e.target,
			EdgeType:      "calls",
			SourceFile:    "",
			Metadata:      []byte("{}"),
		}); err != nil {
			t.Fatalf("upsert edge %+v: %v", e, err)
		}
	}

	// depth=1 only resolves the direct caller of C (B).
	depth1 := callTool("memory_impact", map[string]any{
		"workspace": wsHash,
		"node":      "c.go::C",
		"direction": "in",
		"max_depth": float64(1),
	})
	depth1Resp := unmarshalGraphResp(t, depth1)
	depth1Impacted, _ := depth1Resp["impacted"].([]any)
	if len(depth1Impacted) != 1 {
		t.Fatalf("depth=1 impacted count = %d, want 1 (B only): %+v", len(depth1Impacted), depth1Impacted)
	}
	if depth1Impacted[0].(map[string]any)["node"].(string) != "b.go::B" {
		t.Errorf("depth=1 impacted[0].node = %q, want b.go::B", depth1Impacted[0].(map[string]any)["node"])
	}

	// depth=2 (and depth=3) must additionally resolve the transitive caller A.
	for _, depth := range []float64{2, 3} {
		result := callTool("memory_impact", map[string]any{
			"workspace": wsHash,
			"node":      "c.go::C",
			"direction": "in",
			"max_depth": depth,
		})
		resp := unmarshalGraphResp(t, result)
		impacted, _ := resp["impacted"].([]any)
		if len(impacted) != 2 {
			t.Fatalf("max_depth=%v impacted count = %d, want 2 (B at depth1, A at depth2): %+v", depth, len(impacted), impacted)
		}
		seen := map[string]int{}
		for _, im := range impacted {
			m := im.(map[string]any)
			seen[m["node"].(string)] = int(m["depth"].(float64))
		}
		if seen["b.go::B"] != 1 {
			t.Errorf("max_depth=%v: b.go::B depth = %d, want 1", depth, seen["b.go::B"])
		}
		if seen["a.go::A"] != 2 {
			t.Errorf("max_depth=%v: a.go::A depth = %d, want 2 (transitive caller must be found)", depth, seen["a.go::A"])
		}
	}
}

// AC-A3: no regression — direction:"out" is unaffected by the "in"-only
// frontier expansion, and an already-qualified imports-edge target on the
// "in" path keeps working exactly as before (single, non-bare match).
func TestMemoryImpact_BareCallsTarget_NoRegressionOutAndQualifiedImports(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)

	if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
		WorkspaceHash: wsHash,
		SourceNode:    "a.go::A",
		TargetNode:    "B", // bare calls target
		EdgeType:      "calls",
		SourceFile:    "a.go",
		Metadata:      []byte("{}"),
	}); err != nil {
		t.Fatalf("upsert calls edge: %v", err)
	}
	if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
		WorkspaceHash: wsHash,
		SourceNode:    "consumer.ts",
		TargetNode:    "lib.ts", // already-resolved import target, no "::" suffix
		EdgeType:      "imports",
		SourceFile:    "consumer.ts",
		Metadata:      []byte("{}"),
	}); err != nil {
		t.Fatalf("upsert imports edge: %v", err)
	}

	// direction:"out" from A must return exactly the bare target "B" itself —
	// unaffected by "in"-frontier bare-suffix expansion.
	outResult := callTool("memory_impact", map[string]any{
		"workspace": wsHash,
		"node":      "a.go::A",
		"direction": "out",
		"edge_type": "calls",
	})
	outResp := unmarshalGraphResp(t, outResult)
	outImpacted, _ := outResp["impacted"].([]any)
	if len(outImpacted) != 1 {
		t.Fatalf("direction=out impacted count = %d, want 1: %+v", len(outImpacted), outImpacted)
	}
	if outImpacted[0].(map[string]any)["node"].(string) != "B" {
		t.Errorf("direction=out impacted[0].node = %q, want B", outImpacted[0].(map[string]any)["node"])
	}

	// direction:"in" on an already-qualified (non-symbol) imports target
	// keeps returning exactly its one real importer — no spurious matches.
	inResult := callTool("memory_impact", map[string]any{
		"workspace": wsHash,
		"node":      "lib.ts",
		"direction": "in",
		"edge_type": "imports",
	})
	inResp := unmarshalGraphResp(t, inResult)
	inImpacted, _ := inResp["impacted"].([]any)
	if len(inImpacted) != 1 {
		t.Fatalf("direction=in imports impacted count = %d, want 1: %+v", len(inImpacted), inImpacted)
	}
	if inImpacted[0].(map[string]any)["node"].(string) != "consumer.ts" {
		t.Errorf("direction=in imports impacted[0].node = %q, want consumer.ts", inImpacted[0].(map[string]any)["node"])
	}
}
