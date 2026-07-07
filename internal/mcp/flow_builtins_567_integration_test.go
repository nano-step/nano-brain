//go:build integration

package mcp_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

// Issue #567 (#542 F8): memory_flow must drop RoleExternal nodes that are JS
// builtins/keywords (no workspace symbol), while keeping real leaf functions,
// mirroring memory_trace. include_external keeps them.
func TestMemoryFlow_DropsBuiltinExternalNodes(t *testing.T) {
	ctx, q, wsHash, callTool := setupFlowMCP(t)

	// GET /x -> ctrl (handler); ctrl calls a real workspace helper AND a builtin.
	edges := []struct {
		src, tgt, kind string
	}{
		{"GET /x", "ctrl", "http"},
		{"a.js::ctrl", "realHelper", "calls"}, // resolves (doc seeded) -> kept
		{"a.js::ctrl", "Number", "calls"},     // builtin, no doc -> dropped
	}
	for _, e := range edges {
		if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
			WorkspaceHash: wsHash, SourceNode: e.src, TargetNode: e.tgt,
			EdgeType: e.kind, SourceFile: "a.js", Metadata: []byte("{}"),
		}); err != nil {
			t.Fatalf("upsert edge %+v: %v", e, err)
		}
	}
	// Only realHelper has a symbol doc, so ResolveSymbolByName distinguishes it
	// from the builtin Number.
	upsertSymbolDoc(t, ctx, q, wsHash, "helpers.js", "realHelper", "function", "function realHelper(){}", "1", "1")

	names := func(resp map[string]any) map[string]bool {
		out := map[string]bool{}
		if nodes, ok := resp["nodes"].([]any); ok {
			for _, n := range nodes {
				if nm, _ := n.(map[string]any)["name"].(string); nm != "" {
					out[nm] = true
				}
			}
		}
		return out
	}

	edgeTo := func(resp map[string]any) map[string]bool {
		out := map[string]bool{}
		if es, ok := resp["edges"].([]any); ok {
			for _, e := range es {
				if to, _ := e.(map[string]any)["to"].(string); to != "" {
					out[to] = true
				}
			}
		}
		return out
	}

	// Default: builtin dropped, real helper kept.
	defResp := unmarshalGraphResp(t, callTool("memory_flow", map[string]any{
		"workspace": wsHash, "entry": "GET /x", "format": "json",
	}))
	def := names(defResp)
	if def["Number"] {
		t.Errorf("builtin 'Number' should be dropped by default: %v", def)
	}
	if !def["realHelper"] {
		t.Errorf("real leaf 'realHelper' should be kept: %v", def)
	}
	// The edge to the dropped builtin must be removed; the edge to the kept leaf survives.
	de := edgeTo(defResp)
	if de["Number"] {
		t.Errorf("edge to dropped builtin 'Number' should be removed: %+v", defResp["edges"])
	}
	if !de["realHelper"] {
		t.Errorf("edge to kept leaf 'realHelper' should survive: %+v", defResp["edges"])
	}

	// include_external=true: builtin reappears.
	inc := names(unmarshalGraphResp(t, callTool("memory_flow", map[string]any{
		"workspace": wsHash, "entry": "GET /x", "format": "json", "include_external": true,
	})))
	if !inc["Number"] {
		t.Errorf("builtin 'Number' should appear with include_external=true: %v", inc)
	}
}
