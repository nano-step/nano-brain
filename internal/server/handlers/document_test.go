package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type mockDocumentQuerier struct {
	upsertDocumentFn func(ctx context.Context, arg sqlc.UpsertDocumentParams) (sqlc.UpsertDocumentRow, error)
}

func (m *mockDocumentQuerier) UpsertDocument(ctx context.Context, arg sqlc.UpsertDocumentParams) (sqlc.UpsertDocumentRow, error) {
	return m.upsertDocumentFn(ctx, arg)
}

const testMaxFileSize int64 = 307200

func newWriteContext(e *echo.Echo, body string, workspace string) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/write", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if workspace != "" {
		c.Set("workspace", workspace)
	}
	return c, rec
}

func TestWriteDocument_Success(t *testing.T) {
	q := &mockDocumentQuerier{
		upsertDocumentFn: func(_ context.Context, arg sqlc.UpsertDocumentParams) (sqlc.UpsertDocumentRow, error) {
			return sqlc.UpsertDocumentRow{
				ID:            uuid.New(),
				ContentHash:   arg.ContentHash,
				Collection:    arg.Collection,
				WorkspaceHash: arg.WorkspaceHash,
			}, nil
		},
	}

	e := echo.New()
	body := `{"content":"hello world","workspace":"ws1"}`
	c, rec := newWriteContext(e, body, "ws1")

	h := handlers.WriteDocument(q, nil, zerolog.Nop(), testMaxFileSize)
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp handlers.WriteResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.ID == "" {
		t.Error("expected non-empty id")
	}
	if resp.Hash == "" {
		t.Error("expected non-empty hash")
	}
	if resp.WorkspaceHash != "ws1" {
		t.Errorf("expected workspace_hash=ws1, got %q", resp.WorkspaceHash)
	}
}

func TestWriteDocument_EmptyContent(t *testing.T) {
	q := &mockDocumentQuerier{}

	e := echo.New()
	body := `{"content":"","workspace":"ws1"}`
	c, _ := newWriteContext(e, body, "ws1")

	h := handlers.WriteDocument(q, nil, zerolog.Nop(), testMaxFileSize)
	err := h(c)
	if err == nil {
		t.Fatal("expected error for empty content")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", he.Code)
	}
}

func TestWriteDocument_ContentTooLarge(t *testing.T) {
	q := &mockDocumentQuerier{}

	e := echo.New()
	large := strings.Repeat("x", int(testMaxFileSize)+1)
	body := `{"content":"` + large + `","workspace":"ws1"}`
	c, _ := newWriteContext(e, body, "ws1")

	h := handlers.WriteDocument(q, nil, zerolog.Nop(), testMaxFileSize)
	err := h(c)
	if err == nil {
		t.Fatal("expected error for oversized content")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", he.Code)
	}
}

func TestWriteDocument_DefaultCollection(t *testing.T) {
	var capturedCollection string
	q := &mockDocumentQuerier{
		upsertDocumentFn: func(_ context.Context, arg sqlc.UpsertDocumentParams) (sqlc.UpsertDocumentRow, error) {
			capturedCollection = arg.Collection
			return sqlc.UpsertDocumentRow{
				ID:            uuid.New(),
				ContentHash:   arg.ContentHash,
				Collection:    arg.Collection,
				WorkspaceHash: arg.WorkspaceHash,
			}, nil
		},
	}

	e := echo.New()
	body := `{"content":"hello","workspace":"ws1"}`
	c, _ := newWriteContext(e, body, "ws1")

	h := handlers.WriteDocument(q, nil, zerolog.Nop(), testMaxFileSize)
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if capturedCollection != "memory" {
		t.Errorf("expected default collection=memory, got %q", capturedCollection)
	}
}

func TestWriteDocument_MissingWorkspace(t *testing.T) {
	q := &mockDocumentQuerier{}

	e := echo.New()
	body := `{"content":"hello"}`
	c, _ := newWriteContext(e, body, "")

	h := handlers.WriteDocument(q, nil, zerolog.Nop(), testMaxFileSize)
	err := h(c)
	if err == nil {
		t.Fatal("expected error for missing workspace")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", he.Code)
	}
}

func TestWriteDocument_HashVerification(t *testing.T) {
	content := "hello world"
	expectedHash := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"

	var capturedHash string
	q := &mockDocumentQuerier{
		upsertDocumentFn: func(_ context.Context, arg sqlc.UpsertDocumentParams) (sqlc.UpsertDocumentRow, error) {
			capturedHash = arg.ContentHash
			return sqlc.UpsertDocumentRow{
				ID:            uuid.New(),
				ContentHash:   arg.ContentHash,
				Collection:    arg.Collection,
				WorkspaceHash: arg.WorkspaceHash,
			}, nil
		},
	}

	e := echo.New()
	body := `{"content":"` + content + `"}`
	c, _ := newWriteContext(e, body, "ws1")

	h := handlers.WriteDocument(q, nil, zerolog.Nop(), testMaxFileSize)
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if capturedHash != expectedHash {
		t.Errorf("expected hash %s, got %s", expectedHash, capturedHash)
	}
}
