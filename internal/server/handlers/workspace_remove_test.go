package handlers_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type mockRemoveQuerier struct {
	getWorkspaceFn           func(ctx context.Context, hash string) (sqlc.Workspace, error)
	countDocumentsFn         func(ctx context.Context, workspaceHash string) (int64, error)
	deleteDocumentsFn        func(ctx context.Context, workspaceHash string) error
	deleteWorkspaceFn        func(ctx context.Context, hash string) error
}

func (m *mockRemoveQuerier) GetWorkspaceByHash(ctx context.Context, hash string) (sqlc.Workspace, error) {
	return m.getWorkspaceFn(ctx, hash)
}

func (m *mockRemoveQuerier) CountDocumentsByWorkspace(ctx context.Context, workspaceHash string) (int64, error) {
	return m.countDocumentsFn(ctx, workspaceHash)
}

func (m *mockRemoveQuerier) DeleteDocumentsByWorkspace(ctx context.Context, workspaceHash string) error {
	return m.deleteDocumentsFn(ctx, workspaceHash)
}

func (m *mockRemoveQuerier) DeleteWorkspace(ctx context.Context, hash string) error {
	return m.deleteWorkspaceFn(ctx, hash)
}

func newDefaultRemoveQuerier() *mockRemoveQuerier {
	return &mockRemoveQuerier{
		getWorkspaceFn: func(_ context.Context, hash string) (sqlc.Workspace, error) {
			return sqlc.Workspace{ID: uuid.New(), Hash: hash, Name: "test", Path: "/tmp/test", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
		},
		countDocumentsFn: func(_ context.Context, _ string) (int64, error) {
			return 42, nil
		},
		deleteDocumentsFn: func(_ context.Context, _ string) error {
			return nil
		},
		deleteWorkspaceFn: func(_ context.Context, _ string) error {
			return nil
		},
	}
}

func TestWorkspaceRemove_Success(t *testing.T) {
	q := newDefaultRemoveQuerier()

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/abc123", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("hash")
	c.SetParamValues("abc123")

	h := handlers.RemoveWorkspace(q, nil, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp["workspace"] != "abc123" {
		t.Errorf("expected workspace=abc123, got %v", resp["workspace"])
	}
	if resp["deleted_docs"] != float64(42) {
		t.Errorf("expected deleted_docs=42, got %v", resp["deleted_docs"])
	}
	if resp["workspace_removed"] != true {
		t.Errorf("expected workspace_removed=true, got %v", resp["workspace_removed"])
	}
}

func TestWorkspaceRemove_NotFound(t *testing.T) {
	q := newDefaultRemoveQuerier()
	q.getWorkspaceFn = func(_ context.Context, _ string) (sqlc.Workspace, error) {
		return sqlc.Workspace{}, sql.ErrNoRows
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/nonexistent", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("hash")
	c.SetParamValues("nonexistent")

	h := handlers.RemoveWorkspace(q, nil, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error for missing workspace")
	}

	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if he.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", he.Code)
	}
}

func TestWorkspaceRemove_CascadeStats(t *testing.T) {
	var deletedDocs, deletedWorkspaces int

	q := newDefaultRemoveQuerier()
	q.countDocumentsFn = func(_ context.Context, _ string) (int64, error) {
		return 7, nil
	}
	q.deleteDocumentsFn = func(_ context.Context, _ string) error {
		deletedDocs++
		return nil
	}
	q.deleteWorkspaceFn = func(_ context.Context, _ string) error {
		deletedWorkspaces++
		return nil
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("hash")
	c.SetParamValues("ws1")

	h := handlers.RemoveWorkspace(q, nil, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if deletedDocs != 1 {
		t.Errorf("expected DeleteDocumentsByWorkspace called once, got %d", deletedDocs)
	}
	if deletedWorkspaces != 1 {
		t.Errorf("expected DeleteWorkspace called once, got %d", deletedWorkspaces)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["deleted_docs"] != float64(7) {
		t.Errorf("expected deleted_docs=7, got %v", resp["deleted_docs"])
	}
}

func TestWorkspaceRemove_MissingHash(t *testing.T) {
	q := newDefaultRemoveQuerier()

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.RemoveWorkspace(q, nil, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error for missing hash")
	}

	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", he.Code)
	}
}
