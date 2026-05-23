package mcp_test

import (
	"context"
	"sort"
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
