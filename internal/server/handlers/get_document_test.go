package handlers_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
	"github.com/sqlc-dev/pqtype"
)

type mockGetDocQuerier struct {
	getByID         func(ctx context.Context, arg sqlc.GetDocumentByIDParams) (sqlc.Document, error)
	getBySourcePath func(ctx context.Context, arg sqlc.GetDocumentBySourcePathParams) (sqlc.Document, error)
}

func (m *mockGetDocQuerier) GetDocumentByID(ctx context.Context, arg sqlc.GetDocumentByIDParams) (sqlc.Document, error) {
	return m.getByID(ctx, arg)
}

func (m *mockGetDocQuerier) GetDocumentBySourcePath(ctx context.Context, arg sqlc.GetDocumentBySourcePathParams) (sqlc.Document, error) {
	return m.getBySourcePath(ctx, arg)
}

func sampleDoc(id uuid.UUID, sourcePath string) sqlc.Document {
	return sqlc.Document{
		ID:            id,
		WorkspaceHash: "ws123",
		ContentHash:   "hash",
		Title:         "Test Doc",
		Content:       "Some content",
		SourcePath:    sourcePath,
		Collection:    "memory",
		Tags:          []string{"tag1"},
		Metadata:      pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

func TestGetDocument_BySourcePath(t *testing.T) {
	docID := uuid.New()
	q := &mockGetDocQuerier{
		getBySourcePath: func(_ context.Context, arg sqlc.GetDocumentBySourcePathParams) (sqlc.Document, error) {
			if arg.SourcePath != "memory://test.md" {
				t.Errorf("expected source_path memory://test.md, got %s", arg.SourcePath)
			}
			if arg.WorkspaceHash != "ws123" {
				t.Errorf("expected workspace ws123, got %s", arg.WorkspaceHash)
			}
			return sampleDoc(docID, "memory://test.md"), nil
		},
	}

	e := echo.New()
	body := `{"path":"memory://test.md"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/get", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	h := handlers.GetDocument(q, zerolog.Nop())
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
	if resp["id"] != docID.String() {
		t.Errorf("id = %v, want %s", resp["id"], docID.String())
	}
	if resp["source_path"] != "memory://test.md" {
		t.Errorf("source_path = %v", resp["source_path"])
	}
}

func TestGetDocument_ByID(t *testing.T) {
	docID := uuid.New()
	q := &mockGetDocQuerier{
		getByID: func(_ context.Context, arg sqlc.GetDocumentByIDParams) (sqlc.Document, error) {
			if arg.ID != docID {
				t.Errorf("expected id %s, got %s", docID, arg.ID)
			}
			return sampleDoc(docID, "memory://test.md"), nil
		},
	}

	e := echo.New()
	body := fmt.Sprintf(`{"id":"%s"}`, docID.String())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/get", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	h := handlers.GetDocument(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestGetDocument_ByHashPrefix(t *testing.T) {
	docID := uuid.New()
	q := &mockGetDocQuerier{
		getByID: func(_ context.Context, arg sqlc.GetDocumentByIDParams) (sqlc.Document, error) {
			if arg.ID != docID {
				t.Errorf("expected id %s, got %s", docID, arg.ID)
			}
			return sampleDoc(docID, "memory://test.md"), nil
		},
	}

	e := echo.New()
	body := fmt.Sprintf(`{"path":"#%s"}`, docID.String())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/get", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	h := handlers.GetDocument(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestGetDocument_NotFound(t *testing.T) {
	q := &mockGetDocQuerier{
		getBySourcePath: func(_ context.Context, _ sqlc.GetDocumentBySourcePathParams) (sqlc.Document, error) {
			return sqlc.Document{}, sql.ErrNoRows
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/get", strings.NewReader(`{"path":"missing"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	h := handlers.GetDocument(q, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error for not found")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if he.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", he.Code)
	}
}

func TestGetDocument_MissingPathAndID(t *testing.T) {
	q := &mockGetDocQuerier{}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/get", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	h := handlers.GetDocument(q, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error for missing path/id")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", he.Code)
	}
}
