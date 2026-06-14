//go:build integration

package handlers_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/rs/zerolog"
)

func setupFlowTest(t *testing.T) (string, *sqlc.Queries) {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })

	q := sqlc.New(db)
	ctx := context.Background()

	wsHash := fmt.Sprintf("%x", sha256.Sum256([]byte("flow_test_ws")))
	wsPath := "/tmp/flow-test-ws"
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash,
		Name: "flow-test",
		Path: wsPath,
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	// Seed: http entry → handler, middleware chain, handler calls service
	edges := []struct {
		source, target, etype string
	}{
		// HTTP edge: entry point → handler
		{"POST /api/v1/items", "HandleItems", "http"},
		// Middleware edges
		{"AuthMiddleware", "HandleItems", "middleware"},
		// Handler calls service
		{"HandleItems", "ItemService.Create", "calls"},
		// Service calls repo
		{"ItemService.Create", "ItemRepo.Insert", "calls"},
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
			t.Fatalf("upsert edge %s->%s: %v", e.source, e.target, err)
		}
	}

	return wsHash, q
}

func callFlowHandler(t *testing.T, q handlers.FlowQuerier, flowCfg config.FlowConfig, wsHash string, body map[string]any) map[string]any {
	t.Helper()

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/graph/flow", bytes.NewReader(bodyBytes))
	req.Header.Set(echo.MIMEApplicationJSON, "application/json")
	rec := httptest.NewRecorder()

	e := echo.New()
	c := e.NewContext(req, rec)
	c.Request().Header.Set("Content-Type", "application/json")
	c.Set("workspace", wsHash)

	handler := handlers.GraphFlow(q, flowCfg, zerolog.Nop())
	if err := handler(c); err != nil {
		// If HTTPError, write it so we can inspect status
		if he, ok := err.(*echo.HTTPError); ok {
			rec.WriteHeader(he.Code)
		} else {
			t.Fatalf("handler error: %v", err)
		}
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v\nbody: %s", err, rec.Body.String())
	}
	return resp
}

func TestGraphFlow_KnownEntry_ReturnsMermaid(t *testing.T) {
	wsHash, q := setupFlowTest(t)

	flowCfg := config.FlowConfig{Enabled: true, MaxDepth: 5, MaxFanout: 10}
	resp := callFlowHandler(t, q, flowCfg, wsHash, map[string]any{
		"entry": "POST /api/v1/items",
	})

	if found, _ := resp["found"].(bool); !found {
		t.Fatalf("expected found=true, got resp=%v", resp)
	}
	if resp["entry"] != "POST /api/v1/items" {
		t.Errorf("entry = %v, want 'POST /api/v1/items'", resp["entry"])
	}
	if resp["mermaid"] == "" || resp["mermaid"] == nil {
		t.Error("expected non-empty mermaid field")
	}
	chain, ok := resp["chain"].([]any)
	if !ok || len(chain) == 0 {
		t.Errorf("expected non-empty chain, got %v", resp["chain"])
	}
}

func TestGraphFlow_UnknownEntry_NotFound(t *testing.T) {
	wsHash, q := setupFlowTest(t)

	flowCfg := config.FlowConfig{Enabled: true, MaxDepth: 5, MaxFanout: 10}
	resp := callFlowHandler(t, q, flowCfg, wsHash, map[string]any{
		"entry": "GET /no/such/route",
	})

	if found, _ := resp["found"].(bool); found {
		t.Fatalf("expected found=false for unknown entry, got resp=%v", resp)
	}
}

func TestGraphFlow_Disabled_ReturnsDisabledMessage(t *testing.T) {
	wsHash, q := setupFlowTest(t)

	flowCfg := config.FlowConfig{Enabled: false, MaxDepth: 5, MaxFanout: 10}
	resp := callFlowHandler(t, q, flowCfg, wsHash, map[string]any{
		"entry": "POST /api/v1/items",
	})

	if found, _ := resp["found"].(bool); found {
		t.Fatalf("expected found=false when disabled, got resp=%v", resp)
	}
	if msg, _ := resp["message"].(string); msg != "flow indexing disabled" {
		t.Errorf("message = %q, want 'flow indexing disabled'", msg)
	}
}

func TestGraphFlow_JSONFormat_OmitsMermaid(t *testing.T) {
	wsHash, q := setupFlowTest(t)

	flowCfg := config.FlowConfig{Enabled: true, MaxDepth: 5, MaxFanout: 10}
	resp := callFlowHandler(t, q, flowCfg, wsHash, map[string]any{
		"entry":  "POST /api/v1/items",
		"format": "json",
	})

	if found, _ := resp["found"].(bool); !found {
		t.Fatalf("expected found=true, got resp=%v", resp)
	}
	if _, hasMermaid := resp["mermaid"]; hasMermaid {
		t.Error("expected mermaid field to be absent in json format")
	}
}
