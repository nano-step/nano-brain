package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type mockQuerier struct {
	upsertWorkspaceFn          func(ctx context.Context, arg sqlc.UpsertWorkspaceParams) (sqlc.Workspace, error)
	upsertCollectionFn         func(ctx context.Context, arg sqlc.UpsertCollectionParams) (sqlc.Collection, error)
	listWorkspacesWithStatsFn  func(ctx context.Context) ([]sqlc.ListWorkspacesWithStatsRow, error)
}

func (m *mockQuerier) UpsertWorkspace(ctx context.Context, arg sqlc.UpsertWorkspaceParams) (sqlc.Workspace, error) {
	return m.upsertWorkspaceFn(ctx, arg)
}

func (m *mockQuerier) UpsertCollection(ctx context.Context, arg sqlc.UpsertCollectionParams) (sqlc.Collection, error) {
	return m.upsertCollectionFn(ctx, arg)
}

func (m *mockQuerier) ListWorkspacesWithStats(ctx context.Context) ([]sqlc.ListWorkspacesWithStatsRow, error) {
	return m.listWorkspacesWithStatsFn(ctx)
}

func TestWorkspaceHashDeterministic(t *testing.T) {
	h1, err := storage.WorkspaceHash("/home/user/project")
	if err != nil {
		t.Fatalf("WorkspaceHash: %v", err)
	}
	h2, err := storage.WorkspaceHash("/home/user/project")
	if err != nil {
		t.Fatalf("WorkspaceHash: %v", err)
	}
	if h1 != h2 {
		t.Errorf("expected same hash, got %q and %q", h1, h2)
	}
}

func TestWorkspaceHashDifferentPaths(t *testing.T) {
	h1, err := storage.WorkspaceHash("/home/user/project-a")
	if err != nil {
		t.Fatalf("WorkspaceHash: %v", err)
	}
	h2, err := storage.WorkspaceHash("/home/user/project-b")
	if err != nil {
		t.Fatalf("WorkspaceHash: %v", err)
	}
	if h1 == h2 {
		t.Errorf("expected different hashes for different paths, got %q", h1)
	}
}

func TestInitWorkspaceHandler(t *testing.T) {
	fixedHash, err := storage.WorkspaceHash("/tmp/test-project")
	if err != nil {
		t.Fatalf("WorkspaceHash: %v", err)
	}
	q := &mockQuerier{
		upsertWorkspaceFn: func(_ context.Context, arg sqlc.UpsertWorkspaceParams) (sqlc.Workspace, error) {
			return sqlc.Workspace{
				ID:        uuid.New(),
				Hash:      arg.Hash,
				Name:      arg.Name,
				Path:      arg.Path,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}, nil
		},
		upsertCollectionFn: func(_ context.Context, arg sqlc.UpsertCollectionParams) (sqlc.Collection, error) {
			return sqlc.Collection{
				ID:            uuid.New(),
				WorkspaceHash: arg.WorkspaceHash,
				Name:          arg.Name,
				Path:          arg.Path,
				GlobPattern:   arg.GlobPattern,
				UpdateMode:    arg.UpdateMode,
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
			}, nil
		},
	}

	e := echo.New()
	body := `{"root_path":"/tmp/test-project"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/init", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.InitWorkspace(q, nil, zerolog.Nop())
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

	if resp["workspace_hash"] != fixedHash {
		t.Errorf("expected hash %q, got %v", fixedHash, resp["workspace_hash"])
	}
	if _, ok := resp["root_path"]; !ok {
		t.Error("missing root_path in response")
	}
	if _, ok := resp["agents_snippet"]; !ok {
		t.Error("missing agents_snippet in response")
	}
}

func TestInitWorkspaceHandlerMissingRootPath(t *testing.T) {
	q := &mockQuerier{}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/init", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.InitWorkspace(q, nil, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error for missing root_path")
	}

	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", he.Code)
	}
}

func TestListWorkspacesHandler(t *testing.T) {
	now := time.Now()
	q := &mockQuerier{
		listWorkspacesWithStatsFn: func(_ context.Context) ([]sqlc.ListWorkspacesWithStatsRow, error) {
			return []sqlc.ListWorkspacesWithStatsRow{
				{ID: uuid.New(), Hash: "abc123", Name: "myproject", Path: "/home/user/myproject", CreatedAt: now, UpdatedAt: now, DocumentCount: 5, LastDocumentUpdated: now},
			}, nil
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.ListWorkspaces(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var items []map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	item := items[0]
	for _, field := range []string{"workspace_hash", "root_path", "name", "document_count", "last_document_updated", "created_at", "updated_at"} {
		if _, ok := item[field]; !ok {
			t.Errorf("missing field %q in response item", field)
		}
	}

	if item["workspace_hash"] != "abc123" {
		t.Errorf("expected workspace_hash=abc123, got %v", item["workspace_hash"])
	}
	if item["document_count"] != float64(5) {
		t.Errorf("expected document_count=5, got %v", item["document_count"])
	}
}

func TestListWorkspacesHandlerEmpty(t *testing.T) {
	q := &mockQuerier{
		listWorkspacesWithStatsFn: func(_ context.Context) ([]sqlc.ListWorkspacesWithStatsRow, error) {
			return []sqlc.ListWorkspacesWithStatsRow{}, nil
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.ListWorkspaces(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var items []map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}

	if len(items) != 0 {
		t.Errorf("expected empty list, got %d items", len(items))
	}
}
