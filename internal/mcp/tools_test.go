package mcp_test

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	internalmcp "github.com/nano-brain/nano-brain/internal/mcp"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/rs/zerolog"
)

func TestRegisterTools_CountAndNames(t *testing.T) {
	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(nil, nil, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{}, nil, zerolog.Nop())
	internalmcp.RegisterTools(server, adapter)

	ctx := context.Background()
	ct, st := mcpsdk.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer session.Close()

	result, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	if len(result.Tools) != 14 {
		t.Errorf("expected 14 tools, got %d", len(result.Tools))
		for _, tool := range result.Tools {
			t.Logf("  - %s", tool.Name)
		}
	}

	expected := []string{
		"memory_query",
		"memory_search",
		"memory_vsearch",
		"memory_get",
		"memory_write",
		"memory_tags",
		"memory_status",
		"memory_update",
		"memory_wake_up",
		"memory_symbols",
		"memory_graph",
		"memory_impact",
		"memory_trace",
		"memory_workspaces_resolve",
	}
	sort.Strings(expected)

	got := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		got = append(got, tool.Name)
	}
	sort.Strings(got)

	if len(got) != len(expected) {
		t.Fatalf("tool count mismatch: got %v, want %v", got, expected)
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Errorf("tool[%d] = %q, want %q", i, got[i], expected[i])
		}
	}
}

func setupTestClient(t *testing.T) (*mcpsdk.ClientSession, context.Context) {
	t.Helper()
	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(nil, nil, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{}, nil, zerolog.Nop())
	internalmcp.RegisterTools(server, adapter)

	ctx := context.Background()
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
	return session, ctx
}

func TestMemoryWrite_RejectsWorkspaceAll(t *testing.T) {
	session, ctx := setupTestClient(t)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_write",
		Arguments: map[string]any{
			"workspace": "all",
			"content":   "test content",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for workspace 'all' on write")
	}
	text := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "workspace_all_not_supported") {
		t.Errorf("unexpected error message: %s", text)
	}
}

func TestMemoryUpdate_RejectsWorkspaceAll(t *testing.T) {
	session, ctx := setupTestClient(t)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_update",
		Arguments: map[string]any{
			"workspace": "all",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for workspace 'all' on update")
	}
	text := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "workspace_all_not_supported") {
		t.Errorf("unexpected error message: %s", text)
	}
}

func TestMemoryTags_RejectsWorkspaceAll(t *testing.T) {
	session, ctx := setupTestClient(t)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_tags",
		Arguments: map[string]any{
			"workspace": "all",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for workspace 'all' on tags")
	}
	text := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "cross-workspace not supported") {
		t.Errorf("unexpected error message: %s", text)
	}
}

func TestMemoryWakeUp_RejectsWorkspaceAll(t *testing.T) {
	session, ctx := setupTestClient(t)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_wake_up",
		Arguments: map[string]any{
			"workspace": "all",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for workspace 'all' on wake_up")
	}
	text := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "cross-workspace not supported") {
		t.Errorf("unexpected error message: %s", text)
	}
}

func TestMemoryGet_RejectsWorkspaceAll(t *testing.T) {
	session, ctx := setupTestClient(t)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_get",
		Arguments: map[string]any{
			"workspace": "all",
			"path":      "some-path",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for workspace 'all'")
	}
	text := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "not valid for memory_get") {
		t.Errorf("unexpected error message: %s", text)
	}
}

func TestMemoryGet_RequiresPath(t *testing.T) {
	session, ctx := setupTestClient(t)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_get",
		Arguments: map[string]any{
			"workspace": "test-ws",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing path")
	}
	text := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "path is required") {
		t.Errorf("unexpected error message: %s", text)
	}
}

func TestMemoryGet_InvalidUUID(t *testing.T) {
	session, ctx := setupTestClient(t)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_get",
		Arguments: map[string]any{
			"workspace": "test-ws",
			"path":      "#not-a-uuid",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for invalid UUID")
	}
	text := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "invalid document ID") {
		t.Errorf("unexpected error message: %s", text)
	}
}

func TestMemoryGet_NotFound_ByUUID(t *testing.T) {
	session, ctx := setupMockedTestClient(t)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_get",
		Arguments: map[string]any{
			"workspace": "test-ws",
			"path":      "#00000000-0000-0000-0000-000000000001",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for not-found document")
	}
	text := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "document not found") {
		t.Errorf("unexpected error message: %s", text)
	}
}

func TestMemoryGet_NotFound_BySourcePath(t *testing.T) {
	session, ctx := setupMockedTestClient(t)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_get",
		Arguments: map[string]any{
			"workspace": "test-ws",
			"path":      "/nonexistent/path.md",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for not-found document")
	}
	text := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "document not found") {
		t.Errorf("unexpected error message: %s", text)
	}
}

func TestMemoryGet_WithLineRange_StillReportsNotFound(t *testing.T) {
	session, ctx := setupMockedTestClient(t)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_get",
		Arguments: map[string]any{
			"workspace":  "test-ws",
			"path":       "/nonexistent/path.md",
			"start_line": float64(5),
			"end_line":   float64(10),
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for not-found document with line range")
	}
	text := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "document not found") {
		t.Errorf("unexpected error message: %s", text)
	}
}

func TestMemoryWrite_WithSupersedes_AcceptsField(t *testing.T) {
	session, ctx := setupMockedTestClient(t)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_write",
		Arguments: map[string]any{
			"workspace":  "test-ws",
			"content":    "new content that supersedes old",
			"supersedes": "#00000000-0000-0000-0000-000000000099",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	// The write will fail at the DB layer (fakedb), but the supersedes
	// UUID resolution should succeed (it's a direct parse, not a DB lookup).
	// The error should be about the DB operation, not about supersedes.
	if !result.IsError {
		t.Fatal("expected DB-level error from fakedb")
	}
	text := result.Content[0].(*mcpsdk.TextContent).Text
	if strings.Contains(text, "supersedes") {
		t.Errorf("supersedes should not cause an error, got: %s", text)
	}
}

func TestMemoryWrite_WithSupersedes_BySourcePath(t *testing.T) {
	session, ctx := setupMockedTestClient(t)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_write",
		Arguments: map[string]any{
			"workspace":  "test-ws",
			"content":    "new content",
			"supersedes": "/some/old/path.md",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	// Supersedes by source_path lookup will fail (fakedb) but should be
	// silently ignored. The error should be about the upsert, not supersedes.
	if !result.IsError {
		t.Fatal("expected DB-level error from fakedb")
	}
	text := result.Content[0].(*mcpsdk.TextContent).Text
	if strings.Contains(text, "supersedes") {
		t.Errorf("supersedes lookup failure should be silently ignored, got: %s", text)
	}
}

func TestKeepAliveInterval_Is30Seconds(t *testing.T) {
	if internalmcp.KeepAliveInterval != 30*time.Second {
		t.Errorf("KeepAliveInterval = %v, want 30s", internalmcp.KeepAliveInterval)
	}
}

func TestMemoryWorkspacesResolve_EmptyPath(t *testing.T) {
	session, ctx := setupTestClient(t)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_workspaces_resolve",
		Arguments: map[string]any{
			"path": "",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for empty path")
	}
	text := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "path is required") {
		t.Errorf("expected 'path is required' in error, got: %s", text)
	}
}

func TestMemoryWorkspacesResolve_MissingPath(t *testing.T) {
	session, ctx := setupTestClient(t)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "memory_workspaces_resolve",
		Arguments: map[string]any{},
	})
	if err == nil && !result.IsError {
		t.Fatal("expected error for missing path argument")
	}
}

func TestMemoryWorkspacesResolve_ToolRegistered(t *testing.T) {
	session, ctx := setupTestClient(t)

	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, tool := range list.Tools {
		if tool.Name == "memory_workspaces_resolve" {
			if tool.Description == "" {
				t.Error("memory_workspaces_resolve has empty description")
			}
			return
		}
	}
	t.Fatal("memory_workspaces_resolve not found in tool list")
}
