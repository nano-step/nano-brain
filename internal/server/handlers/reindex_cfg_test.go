package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type mockReindexCFGQuerier struct {
	collections   []sqlc.Collection
	documents     []sqlc.ListDocumentsByWorkspaceRow
	deleteCount   int
	upsertCount   int
	deleteErr     error
	upsertErr     error
}

func (m *mockReindexCFGQuerier) ListCollections(_ context.Context, _ string) ([]sqlc.Collection, error) {
	if m.collections != nil {
		return m.collections, nil
	}
	return nil, nil
}

func (m *mockReindexCFGQuerier) ListDocumentsByWorkspace(_ context.Context, _ string) ([]sqlc.ListDocumentsByWorkspaceRow, error) {
	if m.documents != nil {
		return m.documents, nil
	}
	return nil, nil
}

func (m *mockReindexCFGQuerier) DeleteFunctionFlowchartsByFile(_ context.Context, _ sqlc.DeleteFunctionFlowchartsByFileParams) error {
	m.deleteCount++
	return m.deleteErr
}

func (m *mockReindexCFGQuerier) DeleteAllFunctionFlowcharts(_ context.Context, _ string) error {
	return nil
}

func (m *mockReindexCFGQuerier) UpsertFunctionFlowchart(_ context.Context, _ sqlc.UpsertFunctionFlowchartParams) error {
	m.upsertCount++
	return m.upsertErr
}

func TestReindexCFG_NoJSTSFiles(t *testing.T) {
	e := echo.New()
	body := `{"workspace":"ws123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex-cfg", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	mq := &mockReindexCFGQuerier{
		collections: []sqlc.Collection{{Name: "code", Path: t.TempDir()}},
		documents: []sqlc.ListDocumentsByWorkspaceRow{
			{Collection: "code", SourcePath: filepath.Join(t.TempDir(), "readme.txt")},
		},
	}
	reg := graph.NewRegistry()

	h := handlers.ReindexCFG(mq, reg, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if mq.deleteCount != 0 {
		t.Errorf("expected 0 deletes, got %d", mq.deleteCount)
	}
	if mq.upsertCount != 0 {
		t.Errorf("expected 0 upserts, got %d", mq.upsertCount)
	}
}

func TestReindexCFG_WithJSFile(t *testing.T) {
	dir := t.TempDir()
	jsFile := filepath.Join(dir, "app.js")
	if err := os.WriteFile(jsFile, []byte("function hello() { return 1; }"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := echo.New()
	body := `{"workspace":"ws123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex-cfg", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	mq := &mockReindexCFGQuerier{
		collections: []sqlc.Collection{{Name: "code", Path: dir}},
		documents: []sqlc.ListDocumentsByWorkspaceRow{
			{Collection: "code", SourcePath: jsFile},
		},
	}
	reg := graph.NewRegistry()

	h := handlers.ReindexCFG(mq, reg, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if mq.deleteCount != 1 {
		t.Errorf("expected 1 delete, got %d", mq.deleteCount)
	}
}

func TestReindexCFG_NilGraphRegistry(t *testing.T) {
	e := echo.New()
	body := `{"workspace":"ws123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex-cfg", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	mq := &mockReindexCFGQuerier{}

	h := handlers.ReindexCFG(mq, nil, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error for nil graph registry")
	}
	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", httpErr.Code)
	}
}
