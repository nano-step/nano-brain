//go:build integration

package mcp_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

// --- Issue #561 (split from #542 F3): memory_trace ignores max_depth ---------
//
// Root cause: resolved calls-edge targets were qualified from
// documents.source_path, which is ABSOLUTE in production
// (e.g. "/root/mid.go?symbol=mid"). The BFS enqueued that absolute key and
// looked it up against graph_edges.source_node, which is workspace-RELATIVE
// ("mid.go::mid"). The match failed on every hop after the first, so trace
// only ever returned the entry's direct callees regardless of max_depth.
//
// The pre-existing trace tests seeded a RELATIVE source_path, so the absolute
// key was never produced and the bug was invisible. This test seeds an
// absolute source_path (matching prod) under the workspace root and asserts
// the chain recurses to depth 2.
func TestMemoryTrace_RecursesPastDepth1_AbsoluteSourcePath(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)

	// The workspace root is what documents.source_path is prefixed with in prod.
	ws, err := q.GetWorkspaceByHash(ctx, wsHash)
	if err != nil {
		t.Fatalf("get workspace: %v", err)
	}
	root := ws.Path

	// Mid + Leaf symbol docs carry ABSOLUTE source_path, exactly like the
	// watcher writes them (filePart = <root>/<relfile>). Main is the entry and
	// needs no doc (normalizeNodeForQuery leaves a relative path untouched).
	upsertSymbolDoc(t, ctx, q, wsHash, root+"/mid.go", "mid", "function", "func mid() {}", "1", "1")
	upsertSymbolDoc(t, ctx, q, wsHash, root+"/leaf.go", "leaf", "function", "func leaf() {}", "1", "1")

	// Main -> mid -> leaf; source_node is workspace-RELATIVE as the watcher stores it.
	edges := []struct{ source, target string }{
		{"entry.go::Main", "mid"}, // depth 1
		{"mid.go::mid", "leaf"},   // depth 2 — only reachable if the mid hop matches
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

	// paths=relative so the emitted nodes are the clean "file::symbol" form.
	resp := unmarshalGraphResp(t, callTool("memory_trace", map[string]any{
		"workspace": wsHash,
		"node":      "entry.go::Main",
		"max_depth": float64(3),
		"paths":     "relative",
	}))

	chain, _ := resp["chain"].([]any)
	byName := map[string]int{}
	for _, c := range chain {
		cm := c.(map[string]any)
		byName[cm["name"].(string)] = int(cm["depth"].(float64))
	}
	// Before the fix: chain == [mid@1] only (leaf never reached). After: both.
	if len(chain) != 2 {
		t.Fatalf("chain length = %d, want 2 (mid@1, leaf@2); trace did not recurse past depth 1: %+v", len(chain), chain)
	}
	if byName["mid"] != 1 {
		t.Errorf("mid depth = %d, want 1", byName["mid"])
	}
	if byName["leaf"] != 2 {
		t.Errorf("leaf depth = %d, want 2 (the transitive hop); got %v", byName["leaf"], chain)
	}
}

// A cycle back to the entry symbol, with ABSOLUTE source_path, must not re-list
// the entry (nor loop). Before the seen-key was normalized to the relative form,
// the cycle target qualified absolute ("<root>/entry.go::Main") and missed the
// relative entry seed ("entry.go::Main"), so Main was re-listed. (R88 + Gemini.)
func TestMemoryTrace_CycleBackToEntry_NotReListed_AbsoluteSourcePath(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)
	ws, err := q.GetWorkspaceByHash(ctx, wsHash)
	if err != nil {
		t.Fatalf("get workspace: %v", err)
	}
	root := ws.Path

	// Both symbols carry absolute source_path; mid calls back to Main.
	upsertSymbolDoc(t, ctx, q, wsHash, root+"/entry.go", "Main", "function", "func Main() {}", "1", "1")
	upsertSymbolDoc(t, ctx, q, wsHash, root+"/mid.go", "mid", "function", "func mid() {}", "1", "1")
	for _, e := range []struct{ source, target string }{
		{"entry.go::Main", "mid"}, // Main -> mid
		{"mid.go::mid", "Main"},   // mid -> Main (cycle)
	} {
		if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
			WorkspaceHash: wsHash, SourceNode: e.source, TargetNode: e.target,
			EdgeType: "calls", SourceFile: "", Metadata: []byte("{}"),
		}); err != nil {
			t.Fatalf("upsert edge %+v: %v", e, err)
		}
	}

	resp := unmarshalGraphResp(t, callTool("memory_trace", map[string]any{
		"workspace": wsHash, "node": "entry.go::Main", "max_depth": float64(5), "paths": "relative",
	}))
	chain, _ := resp["chain"].([]any)
	for _, c := range chain {
		if c.(map[string]any)["name"].(string) == "Main" {
			t.Fatalf("entry 'Main' re-listed in its own trace (cycle not deduped): %+v", chain)
		}
	}
	// Only mid is reachable; the mid->Main hop is correctly skipped as the entry.
	if len(chain) != 1 {
		t.Fatalf("chain length = %d, want 1 (just mid; Main is the entry): %+v", len(chain), chain)
	}
}
