//go:build integration

package mcp_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nano-brain/nano-brain/internal/config"
	internalmcp "github.com/nano-brain/nano-brain/internal/mcp"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/rs/zerolog"
)

// setupGraphMCP seeds graph edges following the real watcher convention
// (workspace-relative source/target nodes, e.g. filepath.Rel + ToSlash — see
// internal/watcher/watcher.go relPath/relFile). normalizeNodeForQuery always
// converts an agent-supplied node to this relative form before matching, so
// relative-stored edges (not absolute-prefixed ones) are what a real query
// actually finds.
func setupGraphMCP(t *testing.T) (context.Context, *sqlc.Queries, string, string, *mcpsdk.ClientSession, func(string, map[string]any) *mcpsdk.CallToolResult) {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })

	q := sqlc.New(db)
	wsHash := fmt.Sprintf("%x", sha256.Sum256([]byte("test_ws_"+uuid.New().String())))
	wsPath := "/tmp/test-ws-" + uuid.New().String()[:8]
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash, Name: "test-ws", Path: wsPath,
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	edges := []struct {
		source, target, etype string
	}{
		{"internal/storage/migrate.go::RunMigrations", "context", "calls"},
		{"internal/storage/migrate.go::RunMigrations", "internal/storage/migrate.go::GetCurrentVersion", "calls"},
		{"internal/storage/migrate.go", "internal/storage/migrate.go::RunMigrations", "contains"},
		{"cmd/main.go::startServer", "internal/storage/migrate.go::RunMigrations", "calls"},
	}
	for _, e := range edges {
		if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
			WorkspaceHash: wsHash,
			SourceNode:    e.source,
			TargetNode:    e.target,
			EdgeType:      e.etype,
			SourceFile:    "",
			Metadata:      []byte("{}"),
		}); err != nil {
			t.Fatalf("upsert edge: %v", err)
		}
	}

	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(q, db, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{}, config.FlowConfig{}, nil, zerolog.Nop())
	internalmcp.RegisterTools(server, adapter)

	ct, st := mcpsdk.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })

	callTool := func(name string, args map[string]any) *mcpsdk.CallToolResult {
		t.Helper()
		result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("CallTool(%s): %v", name, err)
		}
		return result
	}
	return ctx, q, wsHash, wsPath, session, callTool
}

func unmarshalGraphResp(t *testing.T, result *mcpsdk.CallToolResult) map[string]any {
	t.Helper()
	if result.IsError {
		t.Fatalf("error result: %s", result.Content[0].(*mcpsdk.TextContent).Text)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].(*mcpsdk.TextContent).Text), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

func TestMemoryGraph_RelativeNodeInputResolvesToAbsolute(t *testing.T) {
	_, _, wsHash, _, _, callTool := setupGraphMCP(t)

	rel := callTool("memory_graph", map[string]any{
		"workspace": wsHash,
		"node":      "internal/storage/migrate.go::RunMigrations",
		"direction": "out",
		"edge_type": "calls",
	})
	relResp := unmarshalGraphResp(t, rel)
	relCount := int(relResp["count"].(float64))
	if relCount != 2 {
		t.Fatalf("relative input: count=%d, want 2 (context + GetCurrentVersion)", relCount)
	}
}

func TestMemoryGraph_AbsoluteNodeInputUnchanged(t *testing.T) {
	ctx, _, wsHash, _, _, callTool := setupGraphMCP(t)
	_ = ctx

	abs := callTool("memory_graph", map[string]any{
		"workspace": wsHash,
		"node":      "/tmp/nonexistent-absolute-path/internal/storage/migrate.go::RunMigrations",
		"direction": "out",
	})
	absResp := unmarshalGraphResp(t, abs)
	if int(absResp["count"].(float64)) != 0 {
		t.Errorf("nonexistent absolute path should match nothing")
	}
}

// TestMemoryGraph_RelativeOutputStripsPrefix verifies the paths=relative strip
// logic. normalizeNodeForQuery always normalizes the query NODE to relative
// before matching, so a query can never reach an edge whose SOURCE is stored
// absolute — but a TARGET can still carry a legacy absolute prefix (e.g. data
// indexed before the relative convention). Seed exactly that shape.
func TestMemoryGraph_RelativeOutputStripsPrefix(t *testing.T) {
	ctx, q, wsHash, wsPath, _, callTool := setupGraphMCP(t)

	legacyTarget := wsPath + "/internal/storage/migrate.go::LegacyHelper"
	if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
		WorkspaceHash: wsHash,
		SourceNode:    "internal/storage/migrate.go::RunMigrations",
		TargetNode:    legacyTarget,
		EdgeType:      "calls",
		SourceFile:    "",
		Metadata:      []byte("{}"),
	}); err != nil {
		t.Fatalf("upsert legacy edge: %v", err)
	}

	abs := callTool("memory_graph", map[string]any{
		"workspace": wsHash,
		"node":      "internal/storage/migrate.go::RunMigrations",
		"direction": "out",
		"edge_type": "calls",
	})
	absResp := unmarshalGraphResp(t, abs)
	edgesAbs, _ := absResp["edges"].([]any)
	if len(edgesAbs) != 3 {
		t.Fatalf("default output: edges=%d, want 3 (context + GetCurrentVersion + legacy)", len(edgesAbs))
	}
	var sawLegacyAbs bool
	for _, e := range edgesAbs {
		em := e.(map[string]any)
		if em["target"].(string) == legacyTarget {
			sawLegacyAbs = true
		}
	}
	if !sawLegacyAbs {
		t.Errorf("default (absolute) should return the legacy target unstripped: %+v", edgesAbs)
	}

	rel := callTool("memory_graph", map[string]any{
		"workspace": wsHash,
		"node":      "internal/storage/migrate.go::RunMigrations",
		"direction": "out",
		"edge_type": "calls",
		"paths":     "relative",
	})
	relResp := unmarshalGraphResp(t, rel)
	edgesRel, _ := relResp["edges"].([]any)
	if len(edgesRel) != 3 {
		t.Fatalf("relative output: edges=%d, want 3", len(edgesRel))
	}
	var contextSeen, getCurrentSeen, legacySeen bool
	for _, e := range edgesRel {
		em := e.(map[string]any)
		src := em["source"].(string)
		tgt := em["target"].(string)
		if strings.HasPrefix(src, "/tmp/") || strings.HasPrefix(tgt, "/tmp/") {
			t.Errorf("relative output: edge still has workspace prefix: source=%q target=%q", src, tgt)
		}
		if src != "internal/storage/migrate.go::RunMigrations" {
			t.Errorf("relative output: source = %q, want stripped", src)
		}
		switch tgt {
		case "context":
			contextSeen = true
		case "internal/storage/migrate.go::GetCurrentVersion":
			getCurrentSeen = true
		case "internal/storage/migrate.go::LegacyHelper":
			legacySeen = true
		}
	}
	if !contextSeen {
		t.Error("relative output: import 'context' should pass through unchanged")
	}
	if !getCurrentSeen {
		t.Error("relative output: workspace-local symbol should have prefix stripped")
	}
	if !legacySeen {
		t.Error("relative output: legacy absolute-prefixed target should have prefix stripped")
	}
}

func TestMemoryGraph_InvalidWorkspaceHashErrorsClearly(t *testing.T) {
	_, _, _, _, _, callTool := setupGraphMCP(t)

	result := callTool("memory_graph", map[string]any{
		"workspace": "nonexistent_workspace_hash_xxxxxx",
		"node":      "internal/storage/migrate.go::RunMigrations",
		"direction": "out",
	})
	if !result.IsError {
		t.Fatal("expected error result for invalid workspace hash")
	}
	msg := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(strings.ToLower(msg), "workspace") {
		t.Errorf("error message should mention workspace, got: %s", msg)
	}
}

func TestMemoryTrace_RelativeInputAndOutput(t *testing.T) {
	_, _, wsHash, _, _, callTool := setupGraphMCP(t)

	result := callTool("memory_trace", map[string]any{
		"workspace": wsHash,
		"node":      "internal/storage/migrate.go::RunMigrations",
		"max_depth": float64(2),
		"paths":     "relative",
	})
	resp := unmarshalGraphResp(t, result)
	entry := resp["entry"].(string)
	if entry != "internal/storage/migrate.go::RunMigrations" {
		t.Errorf("entry not stripped: %q", entry)
	}
	chain := resp["chain"].([]any)
	if len(chain) == 0 {
		t.Fatal("expected at least one chain entry")
	}
	for _, c := range chain {
		cm := c.(map[string]any)
		node := cm["node"].(string)
		via := cm["via"].(string)
		if strings.HasPrefix(node, "/tmp/") {
			t.Errorf("chain node still has prefix: %q", node)
		}
		if strings.HasPrefix(via, "/tmp/") {
			t.Errorf("chain via still has prefix: %q", via)
		}
	}
}

func TestMemoryImpact_RelativeInputAndOutput(t *testing.T) {
	_, _, wsHash, _, _, callTool := setupGraphMCP(t)

	result := callTool("memory_impact", map[string]any{
		"workspace": wsHash,
		"node":      "internal/storage/migrate.go::RunMigrations",
		"edge_type": "calls",
		"max_depth": float64(1),
		"paths":     "relative",
	})
	resp := unmarshalGraphResp(t, result)
	impacted := resp["impacted"].([]any)
	if len(impacted) != 1 {
		t.Fatalf("impacted count = %d, want 1 (startServer)", len(impacted))
	}
	im := impacted[0].(map[string]any)
	node := im["node"].(string)
	if node != "cmd/main.go::startServer" {
		t.Errorf("impacted node = %q, want stripped 'cmd/main.go::startServer'", node)
	}
}
