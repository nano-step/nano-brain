package mcp_test

import (
	"context"
	"sort"
	"strings"
	"testing"

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

	if len(result.Tools) != 9 {
		t.Errorf("expected 9 tools, got %d", len(result.Tools))
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
	if !strings.Contains(text, "not valid for write") {
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
	if !strings.Contains(text, "not valid for write") {
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

func TestMemoryGet_StillWorksAsStub(t *testing.T) {
	session, ctx := setupTestClient(t)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_get",
		Arguments: map[string]any{
			"workspace": "test-ws",
			"id":        "some-id",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result from stub")
	}
	text := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "not yet implemented") {
		t.Errorf("unexpected message: %s", text)
	}
}
