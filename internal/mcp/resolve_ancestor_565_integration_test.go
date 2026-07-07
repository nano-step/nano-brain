//go:build integration

package mcp_test

import (
	"testing"
)

// Issue #565 (#542 F5): resolving a path under a registered ancestor workspace
// returns registered:false BUT surfaces covered_by pointing at the ancestor.
func TestMemoryWorkspacesResolve_CoveredByAncestor(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)

	ws, err := q.GetWorkspaceByHash(ctx, wsHash)
	if err != nil {
		t.Fatalf("get workspace: %v", err)
	}
	childPath := ws.Path + "/backend"

	resp := unmarshalGraphResp(t, callTool("memory_workspaces_resolve", map[string]any{
		"path": childPath,
	}))

	if reg, _ := resp["registered"].(bool); reg {
		t.Fatalf("child path unexpectedly reported registered: %+v", resp)
	}
	covered, ok := resp["covered_by"].(map[string]any)
	if !ok {
		t.Fatalf("covered_by missing; ancestor coverage not surfaced: %+v", resp)
	}
	if covered["workspace_hash"] != wsHash {
		t.Errorf("covered_by.workspace_hash = %v, want ancestor %s", covered["workspace_hash"], wsHash)
	}
	if covered["root_path"] != ws.Path {
		t.Errorf("covered_by.root_path = %v, want %s", covered["root_path"], ws.Path)
	}

	// Control: an unrelated path has no ancestor → no covered_by.
	miss := unmarshalGraphResp(t, callTool("memory_workspaces_resolve", map[string]any{
		"path": "/definitely/not/under/any/workspace",
	}))
	if _, ok := miss["covered_by"]; ok {
		t.Errorf("unrelated path must omit covered_by, got: %+v", miss["covered_by"])
	}
}
