package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

type mockDocumentsQuerier struct {
	rows     []sqlc.ListDocumentsByWorkspaceRow
	deleted  int64
	deleteOK bool
}

func (m *mockDocumentsQuerier) ListDocumentsByWorkspace(_ context.Context, _ string) ([]sqlc.ListDocumentsByWorkspaceRow, error) {
	return m.rows, nil
}

func (m *mockDocumentsQuerier) ListDocumentsByWorkspacePaginated(_ context.Context, _ sqlc.ListDocumentsByWorkspacePaginatedParams) ([]sqlc.ListDocumentsByWorkspacePaginatedRow, error) {
	return nil, nil
}

func (m *mockDocumentsQuerier) DeleteDocumentByIDAndWorkspace(_ context.Context, _ sqlc.DeleteDocumentByIDAndWorkspaceParams) (int64, error) {
	if !m.deleteOK {
		return 0, errors.New("db error")
	}
	return m.deleted, nil
}

func newTestDocRow(title, collection string, tags []string) sqlc.ListDocumentsByWorkspaceRow {
	return sqlc.ListDocumentsByWorkspaceRow{
		ID:         uuid.New(),
		Title:      title,
		Collection: collection,
		SourcePath: "notes/" + title + ".md",
		Tags:       tags,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

func TestListDocuments_ResponseShape(t *testing.T) {
	q := &mockDocumentsQuerier{rows: []sqlc.ListDocumentsByWorkspaceRow{
		newTestDocRow("hello world", "code", []string{"go", "test"}),
	}}
	h := handlers.ListDocuments(q, nopLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents?workspace=abc", nil)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("workspace", "abc")

	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	docs, ok := resp["documents"].([]interface{})
	if !ok {
		t.Fatalf("documents is %T, want array", resp["documents"])
	}
	if len(docs) != 1 {
		t.Fatalf("got %d docs, want 1", len(docs))
	}

	doc := docs[0].(map[string]interface{})
	for _, field := range []string{"id", "title", "collection", "source_path", "tags", "created_at", "updated_at", "supersedes_id", "superseded_by_id"} {
		if _, ok := doc[field]; !ok {
			t.Errorf("missing field %q", field)
		}
	}
	if doc["title"] != "hello world" {
		t.Errorf("title = %v, want 'hello world'", doc["title"])
	}
}

func TestListDocuments_EmptyWorkspace(t *testing.T) {
	q := &mockDocumentsQuerier{rows: nil}
	h := handlers.ListDocuments(q, nopLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents?workspace=empty", nil)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("workspace", "empty")

	if err := h(c); err != nil {
		t.Fatal(err)
	}

	var resp struct {
		Documents []map[string]interface{} `json:"documents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Documents == nil {
		t.Error("documents must be empty array, not null")
	}
	if len(resp.Documents) != 0 {
		t.Errorf("got %d docs, want 0", len(resp.Documents))
	}
}

func TestListDocuments_FilterByCollection(t *testing.T) {
	q := &mockDocumentsQuerier{rows: []sqlc.ListDocumentsByWorkspaceRow{
		newTestDocRow("doc1", "code", nil),
		newTestDocRow("doc2", "notes", nil),
		newTestDocRow("doc3", "code", nil),
	}}
	h := handlers.ListDocuments(q, nopLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents?workspace=abc&collection=code", nil)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("workspace", "abc")

	if err := h(c); err != nil {
		t.Fatal(err)
	}

	var resp struct {
		Documents []map[string]interface{} `json:"documents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Documents) != 2 {
		t.Errorf("got %d docs, want 2 (filtered by code)", len(resp.Documents))
	}
	for _, d := range resp.Documents {
		if d["collection"] != "code" {
			t.Errorf("unexpected collection %v", d["collection"])
		}
	}
}

func TestListDocuments_FilterByText(t *testing.T) {
	q := &mockDocumentsQuerier{rows: []sqlc.ListDocumentsByWorkspaceRow{
		newTestDocRow("Hello World", "code", nil),
		newTestDocRow("Goodbye", "code", nil),
		newTestDocRow("HELLO again", "code", nil),
	}}
	h := handlers.ListDocuments(q, nopLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents?workspace=abc&text=hello", nil)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("workspace", "abc")

	if err := h(c); err != nil {
		t.Fatal(err)
	}

	var resp struct {
		Documents []map[string]interface{} `json:"documents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Documents) != 2 {
		t.Errorf("got %d docs, want 2 (case-insensitive 'hello')", len(resp.Documents))
	}
}

func TestListDocuments_FilterByTags(t *testing.T) {
	q := &mockDocumentsQuerier{rows: []sqlc.ListDocumentsByWorkspaceRow{
		newTestDocRow("doc1", "code", []string{"foo", "bar"}),
		newTestDocRow("doc2", "code", []string{"baz"}),
		newTestDocRow("doc3", "code", []string{"foo"}),
	}}
	h := handlers.ListDocuments(q, nopLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents?workspace=abc&tags=foo", nil)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("workspace", "abc")

	if err := h(c); err != nil {
		t.Fatal(err)
	}

	var resp struct {
		Documents []map[string]interface{} `json:"documents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Documents) != 2 {
		t.Errorf("got %d docs, want 2 (tag foo)", len(resp.Documents))
	}
}

func TestDeleteDocument_Success(t *testing.T) {
	q := &mockDocumentsQuerier{deleteOK: true, deleted: 1}
	h := handlers.DeleteDocument(q, nopLogger())

	docID := uuid.New().String()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/documents/"+docID, nil)
	rec := httptest.NewRecorder()
	e := echo.New()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/documents/:id")
	c.SetParamNames("id")
	c.SetParamValues(docID)
	c.Set("workspace", "abc")

	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["deleted_id"] != docID {
		t.Errorf("deleted_id = %v, want %v", resp["deleted_id"], docID)
	}
}

func TestDeleteDocument_NotFound(t *testing.T) {
	q := &mockDocumentsQuerier{deleteOK: true, deleted: 0}
	h := handlers.DeleteDocument(q, nopLogger())

	docID := uuid.New().String()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/documents/"+docID, nil)
	rec := httptest.NewRecorder()
	e := echo.New()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/documents/:id")
	c.SetParamNames("id")
	c.SetParamValues(docID)
	c.Set("workspace", "abc")

	err := h(c)
	if err == nil {
		t.Fatal("expected error for not-found")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %v", err)
	}
}

func TestDeleteDocument_InvalidID(t *testing.T) {
	q := &mockDocumentsQuerier{}
	h := handlers.DeleteDocument(q, nopLogger())

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/documents/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	e := echo.New()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/documents/:id")
	c.SetParamNames("id")
	c.SetParamValues("not-a-uuid")
	c.Set("workspace", "abc")

	err := h(c)
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %v", err)
	}
}
