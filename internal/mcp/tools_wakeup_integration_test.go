//go:build integration

package mcp_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"encoding/json"
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

// Regression for #356 — MCP memory_wake_up must filter recent_memories to
// memory + session-summary collections (HTTP handler already did this since #338).
func TestMemoryWakeUp_OnlyReturnsMemoryAndSessionSummaryDocs(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })

	queries := sqlc.New(db)

	wsHash := fmt.Sprintf("%x", sha256.Sum256([]byte("test_ws_"+uuid.New().String())))
	if _, err := queries.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash,
		Name: "test-ws",
		Path: "/tmp/" + wsHash,
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	insertDoc := func(t *testing.T, collection, title string) {
		t.Helper()
		_, err := db.ExecContext(ctx,
			`INSERT INTO documents (id, workspace_hash, content_hash, title, source_path, collection)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			uuid.New(), wsHash, uuid.New().String(), title, "/tmp/"+title+".md", collection)
		if err != nil {
			t.Fatalf("insert %s doc %q: %v", collection, title, err)
		}
	}

	insertDoc(t, "memory", "memory-doc-1")
	insertDoc(t, "memory", "memory-doc-2")
	insertDoc(t, "session-summary", "summary-doc-1")
	insertDoc(t, "code", "code-doc-1")
	insertDoc(t, "code", "code-doc-2")
	insertDoc(t, "code", "code-doc-3")

	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(queries, db, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{}, config.FlowConfig{}, nil, zerolog.Nop())
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

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_wake_up",
		Arguments: map[string]any{
			"workspace": wsHash,
			"limit":     10,
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].(*mcpsdk.TextContent).Text)
	}

	text := result.Content[0].(*mcpsdk.TextContent).Text

	var payload struct {
		RecentMemories []struct {
			Title string `json:"title"`
		} `json:"recent_memories"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("unmarshal response: %v\nraw: %s", err, text)
	}

	if len(payload.RecentMemories) != 3 {
		t.Errorf("recent_memories len = %d, want 3; got: %+v", len(payload.RecentMemories), payload.RecentMemories)
	}

	for _, m := range payload.RecentMemories {
		if strings.HasPrefix(m.Title, "code-doc-") {
			t.Errorf("recent_memories must not contain code collection docs, got: %q", m.Title)
		}
	}
}
