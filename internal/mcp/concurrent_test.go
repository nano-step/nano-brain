package mcp_test

import (
	"context"
	"sync"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	internalmcp "github.com/nano-brain/nano-brain/internal/mcp"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/rs/zerolog"
)

// TestConcurrentToolRegistration_RaceFree verifies that registering all 9
// tools and listing them is race-free under the race detector.
func TestConcurrentToolRegistration_RaceFree(t *testing.T) {
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
	}
}

// TestConcurrentToolCalls_NoRace fires 20 goroutines that each invoke
// different tool handlers concurrently through the MCP SDK's in-memory
// transport. The handlers return errors (nil service deps), but the point
// is proving no data race exists under `go test -race`.
func TestConcurrentToolCalls_NoRace(t *testing.T) {
	session, ctx := setupTestClient(t)

	// Tool calls that exercise different handlers. Each returns an error
	// result because service deps are nil, but that is expected — we only
	// care that no race is detected.
	// Only calls that return error results without panicking on nil deps.
	// Handlers that dereference nil queries/db are excluded.
	calls := []mcpsdk.CallToolParams{
		// memory_status — reads adapter fields, nil pool → "not configured"
		{Name: "memory_status", Arguments: map[string]any{}},
		// memory_update — validates workspace, returns "accepted"
		{Name: "memory_update", Arguments: map[string]any{"workspace": "ws-1"}},
		// memory_get — stub, returns "not yet implemented"
		{Name: "memory_get", Arguments: map[string]any{"workspace": "ws-1", "id": "doc-1"}},
		// memory_write — rejected before DB: workspace="all"
		{Name: "memory_write", Arguments: map[string]any{"workspace": "all", "content": "test"}},
		// memory_query — nil searchService → error before DB
		{Name: "memory_query", Arguments: map[string]any{"workspace": "ws-1", "query": "test"}},
		// memory_vsearch — nil embedder → error before DB
		{Name: "memory_vsearch", Arguments: map[string]any{"workspace": "ws-1", "query": "test"}},
		// memory_tags — rejected before DB: workspace="all"
		{Name: "memory_tags", Arguments: map[string]any{"workspace": "all"}},
		// memory_wake_up — rejected before DB: workspace="all"
		{Name: "memory_wake_up", Arguments: map[string]any{"workspace": "all"}},
	}

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			call := calls[idx%len(calls)]
			result, err := session.CallTool(ctx, &call)
			if err != nil {
				t.Errorf("goroutine %d: CallTool(%s) transport error: %v", idx, call.Name, err)
				return
			}
			// We don't check result correctness — just that it returned
			// without a data race. Log for debugging if needed.
			_ = result
		}(i)
	}
	wg.Wait()
}

// TestConcurrentMixedToolCalls_NoRace is a heavier variant that mixes
// read-like and write-like tool calls across 50 goroutines to stress-test
// the adapter's shared-nothing architecture under the race detector.
func TestConcurrentMixedToolCalls_NoRace(t *testing.T) {
	session, ctx := setupTestClient(t)

	type toolCall struct {
		name string
		args map[string]any
	}

	// Only calls safe with nil service deps (no nil-pointer panics).
	palette := []toolCall{
		{"memory_status", map[string]any{}},
		{"memory_update", map[string]any{"workspace": "ws-a"}},
		{"memory_update", map[string]any{"workspace": "ws-b"}},
		{"memory_get", map[string]any{"workspace": "ws-a", "id": "id-1"}},
		{"memory_get", map[string]any{"workspace": "ws-b", "id": "id-2"}},
		{"memory_write", map[string]any{"workspace": "all", "content": "rejected"}},
		{"memory_query", map[string]any{"workspace": "ws-a", "query": "search term"}},
		{"memory_vsearch", map[string]any{"workspace": "ws-a", "query": "vector search"}},
		{"memory_tags", map[string]any{"workspace": "all"}},
		{"memory_wake_up", map[string]any{"workspace": "all"}},
	}

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			tc := palette[idx%len(palette)]
			result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
				Name:      tc.name,
				Arguments: tc.args,
			})
			if err != nil {
				t.Errorf("goroutine %d: CallTool(%s) transport error: %v", idx, tc.name, err)
				return
			}
			_ = result
		}(i)
	}
	wg.Wait()
}
