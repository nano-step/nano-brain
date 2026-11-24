package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type mockCollectionQuerier struct {
	upsertFn           func(ctx context.Context, arg sqlc.UpsertCollectionParams) (sqlc.Collection, error)
	listFn             func(ctx context.Context, ws string) ([]sqlc.Collection, error)
	listWithDocCountFn func(ctx context.Context, ws string) ([]sqlc.ListCollectionsWithDocCountRow, error)
	getFn              func(ctx context.Context, arg sqlc.GetCollectionByNameParams) (sqlc.Collection, error)
	renameFn           func(ctx context.Context, arg sqlc.RenameCollectionParams) (sqlc.Collection, error)
	deleteFn           func(ctx context.Context, arg sqlc.DeleteCollectionParams) error
	countDocsFn        func(ctx context.Context, arg sqlc.CountDocumentsByCollectionParams) (int64, error)
	updateDocsFn       func(ctx context.Context, arg sqlc.UpdateDocumentsCollectionParams) error
}

func (m *mockCollectionQuerier) UpsertCollection(ctx context.Context, arg sqlc.UpsertCollectionParams) (sqlc.Collection, error) {
	return m.upsertFn(ctx, arg)
}
func (m *mockCollectionQuerier) ListCollections(ctx context.Context, ws string) ([]sqlc.Collection, error) {
	return m.listFn(ctx, ws)
}
func (m *mockCollectionQuerier) ListCollectionsWithDocCount(ctx context.Context, ws string) ([]sqlc.ListCollectionsWithDocCountRow, error) {
	return m.listWithDocCountFn(ctx, ws)
}
func (m *mockCollectionQuerier) GetCollectionByName(ctx context.Context, arg sqlc.GetCollectionByNameParams) (sqlc.Collection, error) {
	return m.getFn(ctx, arg)
}
func (m *mockCollectionQuerier) RenameCollection(ctx context.Context, arg sqlc.RenameCollectionParams) (sqlc.Collection, error) {
	return m.renameFn(ctx, arg)
}
func (m *mockCollectionQuerier) DeleteCollection(ctx context.Context, arg sqlc.DeleteCollectionParams) error {
	return m.deleteFn(ctx, arg)
}
func (m *mockCollectionQuerier) CountDocumentsByCollection(ctx context.Context, arg sqlc.CountDocumentsByCollectionParams) (int64, error) {
	return m.countDocsFn(ctx, arg)
}
func (m *mockCollectionQuerier) UpdateDocumentsCollection(ctx context.Context, arg sqlc.UpdateDocumentsCollectionParams) error {
	if m.updateDocsFn == nil {
		return nil
	}
	return m.updateDocsFn(ctx, arg)
}

func newCollectionContext(e *echo.Echo, method, target, body, workspace string) (echo.Context, *httptest.ResponseRecorder) {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if workspace != "" {
		c.Set("workspace", workspace)
	}
	return c, rec
}

func fixedCollection(name, path string) sqlc.Collection {
	return sqlc.Collection{
		ID:            uuid.New(),
		WorkspaceHash: "ws-test",
		Name:          name,
		Path:          path,
		GlobPattern:   "**/*",
		UpdateMode:    "auto",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

func TestAddCollection_Success(t *testing.T) {
	q := &mockCollectionQuerier{
		upsertFn: func(_ context.Context, arg sqlc.UpsertCollectionParams) (sqlc.Collection, error) {
			return fixedCollection(arg.Name, arg.Path), nil
		},
	}

	e := echo.New()
	body := `{"workspace":"ws-test","name":"codebase","path":"/tmp","glob_pattern":"**/*.go"}`
	c, rec := newCollectionContext(e, http.MethodPost, "/api/v1/collections", body, "ws-test")

	h := handlers.AddCollection(q, nil, config.WatcherConfig{}, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp handlers.CollectionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Name != "codebase" {
		t.Errorf("expected name=codebase, got %s", resp.Name)
	}
}

// TestAddCollection_LogicalNameReserved verifies "memory"/"sessions" cannot be
// registered as user collections — they are DB-backed logical collections
// (#714).
func TestAddCollection_LogicalNameReserved(t *testing.T) {
	for _, name := range []string{"memory", "sessions"} {
		q := &mockCollectionQuerier{
			upsertFn: func(_ context.Context, arg sqlc.UpsertCollectionParams) (sqlc.Collection, error) {
				t.Fatalf("UpsertCollection must not be called for reserved name %q", arg.Name)
				return sqlc.Collection{}, nil
			},
		}
		e := echo.New()
		body := `{"workspace":"ws-test","name":"` + name + `","path":"/tmp"}`
		c, _ := newCollectionContext(e, http.MethodPost, "/api/v1/collections", body, "ws-test")

		h := handlers.AddCollection(q, nil, config.WatcherConfig{}, zerolog.Nop())
		err := h(c)
		if err == nil {
			t.Fatalf("expected error adding reserved collection %q", name)
		}
		if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for reserved name %q, got %v", name, err)
		}
	}
}

func TestAddCollection_InvalidPath(t *testing.T) {
	q := &mockCollectionQuerier{}
	e := echo.New()
	body := `{"workspace":"ws-test","name":"bad","path":"/nonexistent-path-xyz-12345"}`
	c, _ := newCollectionContext(e, http.MethodPost, "/api/v1/collections", body, "ws-test")

	h := handlers.AddCollection(q, nil, config.WatcherConfig{}, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", he.Code)
	}
}

func TestAddCollection_MissingName(t *testing.T) {
	q := &mockCollectionQuerier{}
	e := echo.New()
	body := `{"workspace":"ws-test","path":"/tmp"}`
	c, _ := newCollectionContext(e, http.MethodPost, "/api/v1/collections", body, "ws-test")

	h := handlers.AddCollection(q, nil, config.WatcherConfig{}, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", he.Code)
	}
}

func TestListCollections_Success(t *testing.T) {
	col := fixedCollection("codebase", "/tmp")
	q := &mockCollectionQuerier{
		listWithDocCountFn: func(_ context.Context, _ string) ([]sqlc.ListCollectionsWithDocCountRow, error) {
			return []sqlc.ListCollectionsWithDocCountRow{{
				ID:            col.ID,
				WorkspaceHash: col.WorkspaceHash,
				Name:          col.Name,
				Path:          col.Path,
				GlobPattern:   col.GlobPattern,
				UpdateMode:    col.UpdateMode,
				CreatedAt:     col.CreatedAt,
				UpdatedAt:     col.UpdatedAt,
				DocumentCount: 42,
			}}, nil
		},
	}

	e := echo.New()
	c, rec := newCollectionContext(e, http.MethodGet, "/api/v1/collections?workspace=ws-test", "", "ws-test")

	h := handlers.ListCollectionsHandler(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var items []handlers.CollectionResponse
	if err := json.NewDecoder(rec.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].DocumentCount != 42 {
		t.Errorf("expected document_count=42, got %d", items[0].DocumentCount)
	}
}

func TestRenameCollection_Success(t *testing.T) {
	q := &mockCollectionQuerier{
		renameFn: func(_ context.Context, arg sqlc.RenameCollectionParams) (sqlc.Collection, error) {
			return fixedCollection(arg.Name_2, "/tmp"), nil
		},
	}

	e := echo.New()
	body := `{"workspace":"ws-test","new_name":"renamed"}`
	c, rec := newCollectionContext(e, http.MethodPut, "/api/v1/collections/old-name", body, "ws-test")
	c.SetParamNames("name")
	c.SetParamValues("old-name")

	h := handlers.RenameCollectionHandler(q, nil, config.WatcherConfig{}, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp handlers.CollectionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Name != "renamed" {
		t.Errorf("expected name=renamed, got %s", resp.Name)
	}
}

// TestRenameCollection_LogicalNamesReserved verifies logical collection names
// are rejected both as the rename source and as new_name (#714).
func TestRenameCollection_LogicalNamesReserved(t *testing.T) {
	// Renaming FROM a logical name is rejected before any DB call.
	q := &mockCollectionQuerier{
		renameFn: func(_ context.Context, _ sqlc.RenameCollectionParams) (sqlc.Collection, error) {
			t.Fatal("RenameCollection must not be called for a logical source name")
			return sqlc.Collection{}, nil
		},
	}
	e := echo.New()
	c, _ := newCollectionContext(e, http.MethodPut, "/api/v1/collections/memory", `{"workspace":"ws-test","new_name":"renamed"}`, "ws-test")
	c.SetParamNames("name")
	c.SetParamValues("memory")

	h := handlers.RenameCollectionHandler(q, nil, config.WatcherConfig{}, zerolog.Nop())
	if err := h(c); err == nil {
		t.Fatal("expected error renaming a logical collection")
	} else if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %v", err)
	}

	// Renaming TO a logical name is rejected before any DB call.
	q2 := &mockCollectionQuerier{
		renameFn: func(_ context.Context, _ sqlc.RenameCollectionParams) (sqlc.Collection, error) {
			t.Fatal("RenameCollection must not be called for a logical new_name")
			return sqlc.Collection{}, nil
		},
	}
	c2, _ := newCollectionContext(e, http.MethodPut, "/api/v1/collections/old-name", `{"workspace":"ws-test","new_name":"sessions"}`, "ws-test")
	c2.SetParamNames("name")
	c2.SetParamValues("old-name")

	h2 := handlers.RenameCollectionHandler(q2, nil, config.WatcherConfig{}, zerolog.Nop())
	if err := h2(c2); err == nil {
		t.Fatal("expected error renaming to a logical collection name")
	} else if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %v", err)
	}
}

func TestRemoveCollection_Success(t *testing.T) {
	q := &mockCollectionQuerier{
		getFn: func(_ context.Context, _ sqlc.GetCollectionByNameParams) (sqlc.Collection, error) {
			return fixedCollection("doomed", "/tmp"), nil
		},
		deleteFn: func(_ context.Context, _ sqlc.DeleteCollectionParams) error {
			return nil
		},
	}

	e := echo.New()
	c, rec := newCollectionContext(e, http.MethodDelete, "/api/v1/collections/doomed?workspace=ws-test", "", "ws-test")
	c.SetParamNames("name")
	c.SetParamValues("doomed")

	h := handlers.RemoveCollection(q, nil, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRemoveCollection_NotFound(t *testing.T) {
	q := &mockCollectionQuerier{
		getFn: func(_ context.Context, _ sqlc.GetCollectionByNameParams) (sqlc.Collection, error) {
			return sqlc.Collection{}, errors.New("sql: no rows in result set")
		},
	}

	e := echo.New()
	c, _ := newCollectionContext(e, http.MethodDelete, "/api/v1/collections/missing?workspace=ws-test", "", "ws-test")
	c.SetParamNames("name")
	c.SetParamValues("missing")

	h := handlers.RemoveCollection(q, nil, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error for missing collection")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if he.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", he.Code)
	}
}
