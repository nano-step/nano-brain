package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
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

	h := handlers.InitWorkspace(q, nil, nil, config.WatcherConfig{}, zerolog.Nop())
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

	h := handlers.InitWorkspace(q, nil, nil, config.WatcherConfig{}, zerolog.Nop())
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
				{ID: uuid.New(), Hash: "abc123", Name: "myproject", Path: "/home/user/myproject", CreatedAt: now, UpdatedAt: now, DocumentCount: 5, ChunkCount: 42, LastDocumentUpdated: now},
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

	var resp struct {
		Workspaces []map[string]interface{} `json:"workspaces"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if len(resp.Workspaces) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Workspaces))
	}

	item := resp.Workspaces[0]
	for _, field := range []string{"hash", "root_path", "name", "doc_count", "chunk_count", "last_document_updated", "created_at", "updated_at"} {
		if _, ok := item[field]; !ok {
			t.Errorf("missing field %q in response item; got keys: %v", field, mapKeys(item))
		}
	}

	if item["hash"] != "abc123" {
		t.Errorf("expected hash=abc123, got %v", item["hash"])
	}
	if item["doc_count"] != float64(5) {
		t.Errorf("expected doc_count=5, got %v", item["doc_count"])
	}
}

func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestInitWorkspaceCreatesCodeCollection(t *testing.T) {
	var collectionNames []string
	q := &mockQuerier{
		upsertWorkspaceFn: func(_ context.Context, arg sqlc.UpsertWorkspaceParams) (sqlc.Workspace, error) {
			return sqlc.Workspace{
				ID: uuid.New(), Hash: arg.Hash, Name: arg.Name, Path: arg.Path,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}, nil
		},
		upsertCollectionFn: func(_ context.Context, arg sqlc.UpsertCollectionParams) (sqlc.Collection, error) {
			collectionNames = append(collectionNames, arg.Name)
			return sqlc.Collection{
				ID: uuid.New(), WorkspaceHash: arg.WorkspaceHash,
				Name: arg.Name, Path: arg.Path, GlobPattern: arg.GlobPattern,
				UpdateMode: arg.UpdateMode, CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}, nil
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/init", strings.NewReader(`{"root_path":"/tmp/test-project"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.InitWorkspace(q, nil, nil, config.WatcherConfig{}, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Expect exactly three collections: memory, sessions, code.
	if len(collectionNames) != 3 {
		t.Fatalf("expected 3 UpsertCollection calls, got %d: %v", len(collectionNames), collectionNames)
	}
	found := false
	for _, n := range collectionNames {
		if n == "code" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'code' collection to be created, got: %v", collectionNames)
	}
}

func TestInitWorkspaceCodeCollectionIdempotent(t *testing.T) {
	var callCount atomic.Int32
	q := &mockQuerier{
		upsertWorkspaceFn: func(_ context.Context, arg sqlc.UpsertWorkspaceParams) (sqlc.Workspace, error) {
			return sqlc.Workspace{
				ID: uuid.New(), Hash: arg.Hash, Name: arg.Name, Path: arg.Path,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}, nil
		},
		upsertCollectionFn: func(_ context.Context, arg sqlc.UpsertCollectionParams) (sqlc.Collection, error) {
			callCount.Add(1)
			return sqlc.Collection{
				ID: uuid.New(), WorkspaceHash: arg.WorkspaceHash,
				Name: arg.Name, Path: arg.Path, GlobPattern: arg.GlobPattern,
				UpdateMode: arg.UpdateMode, CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}, nil
		},
	}

	e := echo.New()
	h := handlers.InitWorkspace(q, nil, nil, config.WatcherConfig{}, zerolog.Nop())

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/init", strings.NewReader(`{"root_path":"/tmp/test-project"}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		if err := h(c); err != nil {
			t.Fatalf("handler error on call %d: %v", i+1, err)
		}
	}

	// Two calls × 3 collections each = 6 total UpsertCollection invocations.
	// UpsertCollection uses ON CONFLICT DO UPDATE, so duplicate rows are not a concern at the DB level.
	if callCount.Load() != 6 {
		t.Errorf("expected 6 UpsertCollection calls (2 inits × 3 collections), got %d", callCount.Load())
	}
}

func TestInitWorkspaceCodeCollectionErrorRollback(t *testing.T) {
	var collectionCallCount int
	q := &mockQuerier{
		upsertWorkspaceFn: func(_ context.Context, arg sqlc.UpsertWorkspaceParams) (sqlc.Workspace, error) {
			return sqlc.Workspace{
				ID: uuid.New(), Hash: arg.Hash, Name: arg.Name, Path: arg.Path,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}, nil
		},
		upsertCollectionFn: func(_ context.Context, arg sqlc.UpsertCollectionParams) (sqlc.Collection, error) {
			collectionCallCount++
			if arg.Name == "code" {
				return sqlc.Collection{}, errors.New("db error")
			}
			return sqlc.Collection{
				ID: uuid.New(), WorkspaceHash: arg.WorkspaceHash,
				Name: arg.Name, Path: arg.Path, GlobPattern: arg.GlobPattern,
				UpdateMode: arg.UpdateMode, CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}, nil
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/init", strings.NewReader(`{"root_path":"/tmp/test-project"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.InitWorkspace(q, nil, nil, config.WatcherConfig{}, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error when code collection upsert fails")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if he.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", he.Code)
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

	var resp struct {
		Workspaces []map[string]interface{} `json:"workspaces"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp.Workspaces == nil {
		t.Errorf("expected workspaces field to be empty array, got nil (must serialize as [])")
	}
	if len(resp.Workspaces) != 0 {
		t.Errorf("expected empty list, got %d items", len(resp.Workspaces))
	}
}
