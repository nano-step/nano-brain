//go:build integration

package mcp_test

import (
	"path/filepath"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

// Issue #685: memory_get rejected workspace-relative paths even though agents
// naturally pass them; stored source_paths are absolute.
func TestMemoryGet_RelativePathResolvesAgainstWorkspaceRoot(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)

	// The workspace root registered by setupFindingsMCP is a synthetic
	// "/tmp/test-ws-…" path; source_paths are stored absolute under it.
	ws, err := q.GetWorkspaceByHash(ctx, wsHash)
	if err != nil {
		t.Fatalf("get workspace: %v", err)
	}
	absPath := filepath.Join(ws.Path, "internal", "search", "rrf.go")
	if _, err := q.UpsertDocument(ctx, sqlc.UpsertDocumentParams{
		WorkspaceHash: wsHash,
		ContentHash:   "hash-rel-1",
		Title:         "rrf.go",
		Content:       "package search\n\nfunc RRFMerge() {}",
		SourcePath:    absPath,
		Collection:    "code",
	}); err != nil {
		t.Fatalf("upsert document: %v", err)
	}

	res := callTool("memory_get", map[string]any{"workspace": wsHash, "path": "internal/search/rrf.go"})
	if res.IsError {
		t.Fatalf("relative path lookup failed: %s", res.Content[0].(*mcpsdk.TextContent).Text)
	}
	resp := unmarshalGraphResp(t, res)
	if !strings.Contains(resp["content"].(string), "RRFMerge") {
		t.Errorf("content = %q, want the stored document body", resp["content"])
	}

	// A still-missing relative path keeps the clean error.
	missing := callTool("memory_get", map[string]any{"workspace": wsHash, "path": "internal/nope.go"})
	if !missing.IsError {
		t.Fatal("expected error for a missing relative path")
	}
}

// The file part of a "file::Symbol" graph node may also be workspace-relative.
func TestMemoryGet_RelativeGraphNodeResolvesSymbol(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)

	ws, err := q.GetWorkspaceByHash(ctx, wsHash)
	if err != nil {
		t.Fatalf("get workspace: %v", err)
	}
	absFile := filepath.Join(ws.Path, "internal", "graph", "node.go")
	parentContent := "line1\nline2 func Foo() {}\nline3"
	if _, err := q.UpsertDocument(ctx, sqlc.UpsertDocumentParams{
		WorkspaceHash: wsHash,
		ContentHash:   "hash-rel-node",
		Title:         "node.go",
		Content:       parentContent,
		SourcePath:    absFile,
		Collection:    "code",
	}); err != nil {
		t.Fatalf("upsert parent doc: %v", err)
	}
	upsertSymbolDoc(t, ctx, q, wsHash, absFile, "Foo", "function", "func Foo() {}", "2", "2")

	res := callTool("memory_get", map[string]any{"workspace": wsHash, "path": "internal/graph/node.go::Foo"})
	if res.IsError {
		t.Fatalf("relative graph node lookup failed: %s", res.Content[0].(*mcpsdk.TextContent).Text)
	}
	resp := unmarshalGraphResp(t, res)
	if !strings.Contains(resp["content"].(string), "func Foo() {}") {
		t.Errorf("content = %q, want the symbol body", resp["content"])
	}
}
