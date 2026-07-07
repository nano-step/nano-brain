//go:build integration

package mcp_test

import (
	"context"
	"crypto/sha256"
	"fmt"
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

// setupFlowMCP is setupFindingsMCP with flow indexing ENABLED so the
// memory_flow handler runs (setupFindingsMCP uses a zero FlowConfig → disabled).
func setupFlowMCP(t *testing.T) (context.Context, *sqlc.Queries, string, func(string, map[string]any) *mcpsdk.CallToolResult) {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })

	q := sqlc.New(db)
	wsHash := fmt.Sprintf("%x", sha256.Sum256([]byte("flow_ws_"+uuid.New().String())))
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash, Name: "flow-ws", Path: "/tmp/flow-ws-" + uuid.New().String()[:8],
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(q, db, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{},
		config.FlowConfig{Enabled: true, MaxDepth: 5, MaxFanout: 20}, nil, zerolog.Nop())
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
	return ctx, q, wsHash, callTool
}

// Issue #563 (#542 F1): the full mounted URL resolves to the router-local key.
func TestMemoryFlow_FullMountedURLResolvesToRouterLocalKey(t *testing.T) {
	ctx, q, wsHash, callTool := setupFlowMCP(t)

	// Route stored router-local, as the Express extractor writes it.
	if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
		WorkspaceHash: wsHash,
		SourceNode:    "POST /payment-intent",
		TargetNode:    "createPaymentIntent",
		EdgeType:      "http",
		SourceFile:    "routes/payments.js",
		Metadata:      []byte(`{"method":"POST","path":"/payment-intent"}`),
	}); err != nil {
		t.Fatalf("upsert http edge: %v", err)
	}

	// The agent supplies the real mounted URL (from the network tab / contract).
	resp := unmarshalGraphResp(t, callTool("memory_flow", map[string]any{
		"workspace": wsHash,
		"entry":     "POST /api/payments/payment-intent",
		"format":    "json",
	}))

	if found, _ := resp["found"].(bool); !found {
		t.Fatalf("found = false; full mounted URL did not resolve to the router-local key: %+v", resp)
	}
	if resp["resolved_via"] != "suffix-match" {
		t.Errorf("resolved_via = %v, want suffix-match", resp["resolved_via"])
	}
	if resp["requested_entry"] != "POST /api/payments/payment-intent" {
		t.Errorf("requested_entry = %v, want the full URL", resp["requested_entry"])
	}
	if resp["entry"] != "POST /payment-intent" {
		t.Errorf("entry (resolved key) = %v, want POST /payment-intent", resp["entry"])
	}
	// The handler must be present in the flow.
	var sawHandler bool
	if nodes, ok := resp["nodes"].([]any); ok {
		for _, n := range nodes {
			if nm, _ := n.(map[string]any)["name"].(string); nm == "createPaymentIntent" {
				sawHandler = true
			}
		}
	}
	if !sawHandler {
		t.Errorf("handler createPaymentIntent not in flow nodes: %+v", resp["nodes"])
	}

	// Control 1: an unknown URL still returns found:false.
	miss := unmarshalGraphResp(t, callTool("memory_flow", map[string]any{
		"workspace": wsHash, "entry": "POST /api/nope", "format": "json",
	}))
	if found, _ := miss["found"].(bool); found {
		t.Errorf("unknown route resolved unexpectedly: %+v", miss)
	}

	// Control 2: an EXACT router-local match must NOT carry the resolution
	// fields — they are added only when the requested entry was rewritten.
	exact := unmarshalGraphResp(t, callTool("memory_flow", map[string]any{
		"workspace": wsHash, "entry": "POST /payment-intent", "format": "json",
	}))
	if found, _ := exact["found"].(bool); !found {
		t.Fatalf("exact router-local entry not found: %+v", exact)
	}
	if _, ok := exact["resolved_via"]; ok {
		t.Errorf("exact match must omit resolved_via, got: %v", exact["resolved_via"])
	}
	if _, ok := exact["requested_entry"]; ok {
		t.Errorf("exact match must omit requested_entry, got: %v", exact["requested_entry"])
	}
}
