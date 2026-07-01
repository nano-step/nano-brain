//go:build integration

package mcp_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nano-brain/nano-brain/internal/config"
	internalmcp "github.com/nano-brain/nano-brain/internal/mcp"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// TestStreamableHTTP_ConnectionDefaultWorkspace is the single most important
// test in Phase 9 (RESEARCH.md Pitfall 1): it drives a real HTTP request
// through the exact same wiring as routes.go —
// echo.WrapHandler(internalmcp.WrapStreamableHandler(streamableHandler)) —
// so it is the only test that would catch a regression where the middleware
// is re-wired as an echo.MiddlewareFunc (whose c.Set() values never reach
// the SDK's req.Context()). The in-memory-transport tests in
// tools_internal_test.go / tools_security_test.go bypass HTTP entirely and
// provably cannot catch this class of bug.
func TestStreamableHTTP_ConnectionDefaultWorkspace(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping postgres-dependent integration test in -short mode")
	}

	pgDB := setupMCPSecTestPG(t)

	// Register a real workspace so resolution succeeds end-to-end.
	wsName := "streamable-http-default-ws"
	wsHash := hex.EncodeToString(sha256.New().Sum([]byte(wsName)))
	q := sqlc.New(pgDB)
	if _, err := q.UpsertWorkspace(context.Background(), sqlc.UpsertWorkspaceParams{
		Hash: wsHash,
		Name: wsName,
		Path: "/tmp/" + wsName,
	}); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}

	// Stand up a real Echo instance registering /mcp EXACTLY as routes.go
	// does, so this test fails if that wiring ever regresses.
	mcpServer := internalmcp.NewMCPServer("test-streamable-http")
	adapter := internalmcp.NewAdapter(sqlc.New(pgDB), pgDB, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{}, config.FlowConfig{}, nil, zerolog.Nop())
	internalmcp.RegisterTools(mcpServer, adapter)

	streamableHandler := internalmcp.NewStreamableHTTPHandler(mcpServer)
	wrappedStreamable := internalmcp.WrapStreamableHandler(streamableHandler)

	e := echo.New()
	e.GET("/mcp", echo.WrapHandler(wrappedStreamable))
	e.POST("/mcp", echo.WrapHandler(wrappedStreamable))
	e.DELETE("/mcp", echo.WrapHandler(wrappedStreamable))

	ts := httptest.NewServer(e)
	defer ts.Close()

	ctx := context.Background()

	t.Run("query param default lets tool call omit workspace arg", func(t *testing.T) {
		transport := &mcpsdk.StreamableClientTransport{Endpoint: ts.URL + "/mcp?workspace=" + wsName}
		client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "streamable-http-client", Version: "v0.0.1"}, nil)
		session, err := client.Connect(ctx, transport, nil)
		if err != nil {
			t.Fatalf("client.Connect: %v", err)
		}
		defer session.Close()

		result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
			Name:      "memory_tags",
			Arguments: map[string]any{},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if result.IsError {
			text := result.Content[0].(*mcpsdk.TextContent).Text
			t.Fatalf("expected success via ?workspace= connection default, got error: %s", text)
		}
	})

	t.Run("no query param and no arg still requires workspace", func(t *testing.T) {
		transport := &mcpsdk.StreamableClientTransport{Endpoint: ts.URL + "/mcp"}
		client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "streamable-http-client-negative", Version: "v0.0.1"}, nil)
		session, err := client.Connect(ctx, transport, nil)
		if err != nil {
			t.Fatalf("client.Connect: %v", err)
		}
		defer session.Close()

		result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
			Name:      "memory_tags",
			Arguments: map[string]any{},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected \"workspace is required\" error with no query param and no arg (D-04 over HTTP)")
		}
		text := result.Content[0].(*mcpsdk.TextContent).Text
		if !strings.Contains(text, "workspace is required") {
			t.Errorf("expected \"workspace is required\", got: %s", text)
		}
	})
}
