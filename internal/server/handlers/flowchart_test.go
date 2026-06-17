package handlers_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type mockFCQuerier struct {
	ListAllEdgesByWorkspaceFn         func(ctx context.Context, workspaceHash string) ([]sqlc.GraphEdge, error)
	GetFunctionFlowchartByHandlerFn   func(ctx context.Context, arg sqlc.GetFunctionFlowchartByHandlerParams) (sqlc.FunctionFlowchart, error)
}

func (m *mockFCQuerier) ListAllEdgesByWorkspace(ctx context.Context, workspaceHash string) ([]sqlc.GraphEdge, error) {
	return m.ListAllEdgesByWorkspaceFn(ctx, workspaceHash)
}

func (m *mockFCQuerier) GetFunctionFlowchartByHandler(ctx context.Context, arg sqlc.GetFunctionFlowchartByHandlerParams) (sqlc.FunctionFlowchart, error) {
	if m.GetFunctionFlowchartByHandlerFn != nil {
		return m.GetFunctionFlowchartByHandlerFn(ctx, arg)
	}
	return sqlc.FunctionFlowchart{}, sql.ErrNoRows
}

func callFlowchartHandler(t *testing.T, q handlers.FlowchartQuerier, flowCfg config.FlowConfig, wsHash string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/graph/flowchart", bytes.NewReader(bodyBytes))
	req.Header.Set(echo.MIMEApplicationJSON, "application/json")
	rec := httptest.NewRecorder()

	e := echo.New()
	c := e.NewContext(req, rec)
	c.Request().Header.Set("Content-Type", "application/json")
	c.Set("workspace", wsHash)

	handler := handlers.GraphFlowchart(q, flowCfg, zerolog.Nop())
	if err := handler(c); err != nil {
		if he, ok := err.(*echo.HTTPError); ok {
			rec.WriteHeader(he.Code)
		} else {
			t.Fatalf("handler error: %v", err)
		}
	}
	return rec
}

func TestGraphFlowchart_Disabled(t *testing.T) {
	rec := callFlowchartHandler(t, &mockFCQuerier{}, config.FlowConfig{Enabled: false}, "test-hash", map[string]any{"entry": "POST /test"})
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["found"] != false {
		t.Errorf("found = %v, want false", resp["found"])
	}
}

func TestGraphFlowchart_EntryNotFound(t *testing.T) {
	q := &mockFCQuerier{
		ListAllEdgesByWorkspaceFn: func(_ context.Context, _ string) ([]sqlc.GraphEdge, error) {
			return []sqlc.GraphEdge{}, nil
		},
		GetFunctionFlowchartByHandlerFn: func(_ context.Context, _ sqlc.GetFunctionFlowchartByHandlerParams) (sqlc.FunctionFlowchart, error) {
			return sqlc.FunctionFlowchart{}, sql.ErrNoRows
		},
	}
	rec := callFlowchartHandler(t, q, config.FlowConfig{Enabled: true}, "test-hash", map[string]any{"entry": "POST /unknown"})
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["found"] != false {
		t.Errorf("found = %v, want false", resp["found"])
	}
}

func TestGraphFlowchart_ReturnsCFG(t *testing.T) {
	q := &mockFCQuerier{
		ListAllEdgesByWorkspaceFn: func(_ context.Context, _ string) ([]sqlc.GraphEdge, error) {
			return []sqlc.GraphEdge{
				{SourceNode: "POST /api/game", TargetNode: "handleGame", EdgeType: "http", SourceFile: "routes.ts", Metadata: json.RawMessage(`{"method":"POST","path":"/api/game"}`)},
			}, nil
		},
		GetFunctionFlowchartByHandlerFn: func(_ context.Context, arg sqlc.GetFunctionFlowchartByHandlerParams) (sqlc.FunctionFlowchart, error) {
			return sqlc.FunctionFlowchart{
				Entry:      "routes.ts::handleGame",
				SourceFile: "routes.ts",
				StartLine:  15,
				EndLine:    42,
				Status:     "complete",
				Cfg:        json.RawMessage(`{"nodes":[{"id":"n0","type":"start","label":"routes.ts::handleGame","line":15}],"edges":[],"status":"complete"}`),
			}, nil
		},
	}
	rec := callFlowchartHandler(t, q, config.FlowConfig{Enabled: true}, "test-hash", map[string]any{"entry": "POST /api/game"})
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["found"] != true {
		t.Errorf("found = %v, want true", resp["found"])
	}
	if resp["method"] != "POST" {
		t.Errorf("method = %v, want POST", resp["method"])
	}
	if resp["cfg"] == nil {
		t.Error("cfg is nil, expected CFG object")
	}
}

func TestGraphFlowchart_HandlerNotFound(t *testing.T) {
	q := &mockFCQuerier{
		ListAllEdgesByWorkspaceFn: func(_ context.Context, _ string) ([]sqlc.GraphEdge, error) {
			return []sqlc.GraphEdge{
				{SourceNode: "POST /api/game", TargetNode: "handleGame", EdgeType: "http"},
			}, nil
		},
		GetFunctionFlowchartByHandlerFn: func(_ context.Context, _ sqlc.GetFunctionFlowchartByHandlerParams) (sqlc.FunctionFlowchart, error) {
			return sqlc.FunctionFlowchart{}, sql.ErrNoRows
		},
	}
	rec := callFlowchartHandler(t, q, config.FlowConfig{Enabled: true}, "test-hash", map[string]any{"entry": "POST /api/game"})
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["found"] != false {
		t.Errorf("found = %v, want false", resp["found"])
	}
}

func TestGraphFlowchart_EmptyEntry(t *testing.T) {
	rec := callFlowchartHandler(t, &mockFCQuerier{}, config.FlowConfig{Enabled: true}, "test-hash", map[string]any{"entry": ""})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestGraphFlowchart_DBError(t *testing.T) {
	q := &mockFCQuerier{
		ListAllEdgesByWorkspaceFn: func(_ context.Context, _ string) ([]sqlc.GraphEdge, error) {
			return nil, errors.New("db error")
		},
	}
	rec := callFlowchartHandler(t, q, config.FlowConfig{Enabled: true}, "test-hash", map[string]any{"entry": "POST /api/game"})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestGraphFlowchart_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/graph/flowchart", bytes.NewReader([]byte(`{invalid json`)))
	req.Header.Set(echo.MIMEApplicationJSON, "application/json")
	rec := httptest.NewRecorder()

	e := echo.New()
	c := e.NewContext(req, rec)
	c.Request().Header.Set("Content-Type", "application/json")
	c.Set("workspace", "test-hash")

	handler := handlers.GraphFlowchart(&mockFCQuerier{}, config.FlowConfig{Enabled: true}, zerolog.Nop())
	if err := handler(c); err != nil {
		if he, ok := err.(*echo.HTTPError); ok {
			rec.WriteHeader(he.Code)
		} else {
			t.Fatalf("handler error: %v", err)
		}
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
