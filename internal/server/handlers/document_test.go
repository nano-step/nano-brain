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
	upsertDocumentFn        func(ctx context.Context, arg sqlc.UpsertDocumentParams) (sqlc.UpsertDocumentRow, error)
	deleteChunksFn          func(ctx context.Context, arg sqlc.DeleteChunksByDocumentIDParams) error
	upsertChunkFn           func(ctx context.Context, arg sqlc.UpsertChunkParams) (uuid.UUID, error)
}

func (m *mockDocumentQuerier) UpsertDocument(ctx context.Context, arg sqlc.UpsertDocumentParams) (sqlc.UpsertDocumentRow, error) {
	return m.upsertDocumentFn(ctx, arg)
}

func (m *mockDocumentQuerier) DeleteChunksByDocumentID(ctx context.Context, arg sqlc.DeleteChunksByDocumentIDParams) error {
	if m.deleteChunksFn != nil {
		return m.deleteChunksFn(ctx, arg)
	}
	return nil
}

func (m *mockDocumentQuerier) UpsertChunk(ctx context.Context, arg sqlc.UpsertChunkParams) (uuid.UUID, error) {
	if m.upsertChunkFn != nil {
		return m.upsertChunkFn(ctx, arg)
	}
	return uuid.New(), nil
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

	h := handlers.WriteDocument(q, nil, nil, zerolog.Nop(), testMaxFileSize)
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
	if resp.ChunkCount < 1 {
		t.Errorf("expected chunk_count >= 1, got %d", resp.ChunkCount)
	}
}

func TestWriteDocument_ChunksCreated(t *testing.T) {
	docID := uuid.New()
	var upsertChunkCalls []sqlc.UpsertChunkParams
	var deleteChunkCalls []sqlc.DeleteChunksByDocumentIDParams

	q := &mockDocumentQuerier{
		upsertDocumentFn: func(_ context.Context, arg sqlc.UpsertDocumentParams) (sqlc.UpsertDocumentRow, error) {
			return sqlc.UpsertDocumentRow{
				ID:            docID,
				ContentHash:   arg.ContentHash,
				Collection:    arg.Collection,
				WorkspaceHash: arg.WorkspaceHash,
			}, nil
		},
		deleteChunksFn: func(_ context.Context, arg sqlc.DeleteChunksByDocumentIDParams) error {
			deleteChunkCalls = append(deleteChunkCalls, arg)
			return nil
		},
		upsertChunkFn: func(_ context.Context, arg sqlc.UpsertChunkParams) (uuid.UUID, error) {
			upsertChunkCalls = append(upsertChunkCalls, arg)
			return uuid.New(), nil
		},
	}

	e := echo.New()
	body := `{"content":"hello world","workspace":"ws1"}`
	c, rec := newWriteContext(e, body, "ws1")

	h := handlers.WriteDocument(q, nil, nil, zerolog.Nop(), testMaxFileSize)
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	if len(deleteChunkCalls) != 1 {
		t.Fatalf("expected 1 DeleteChunksByDocumentID call, got %d", len(deleteChunkCalls))
	}
	if deleteChunkCalls[0].DocumentID != docID {
		t.Errorf("delete called with wrong document_id: %v", deleteChunkCalls[0].DocumentID)
	}
	if deleteChunkCalls[0].WorkspaceHash != "ws1" {
		t.Errorf("delete called with wrong workspace_hash: %q", deleteChunkCalls[0].WorkspaceHash)
	}

	if len(upsertChunkCalls) == 0 {
		t.Fatal("expected at least 1 UpsertChunk call")
	}
	for i, cp := range upsertChunkCalls {
		if cp.DocumentID != docID {
			t.Errorf("chunk[%d]: wrong document_id", i)
		}
		if cp.WorkspaceHash != "ws1" {
			t.Errorf("chunk[%d]: wrong workspace_hash", i)
		}
		if cp.ContentHash == "" {
			t.Errorf("chunk[%d]: empty content_hash", i)
		}
		if cp.Content == "" {
			t.Errorf("chunk[%d]: empty content", i)
		}
		if !cp.StartLine.Valid {
			t.Errorf("chunk[%d]: start_line should be valid", i)
		}
		if !cp.EndLine.Valid {
			t.Errorf("chunk[%d]: end_line should be valid", i)
		}
	}

	var resp handlers.WriteResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.ChunkCount != len(upsertChunkCalls) {
		t.Errorf("expected chunk_count=%d, got %d", len(upsertChunkCalls), resp.ChunkCount)
	}
}

func TestWriteDocument_EmptyContent(t *testing.T) {
	q := &mockDocumentQuerier{}

	e := echo.New()
	body := `{"content":"","workspace":"ws1"}`
	c, _ := newWriteContext(e, body, "ws1")

	h := handlers.WriteDocument(q, nil, nil, zerolog.Nop(), testMaxFileSize)
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

	h := handlers.WriteDocument(q, nil, nil, zerolog.Nop(), testMaxFileSize)
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

	h := handlers.WriteDocument(q, nil, nil, zerolog.Nop(), testMaxFileSize)
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if capturedCollection != "memory" {
		t.Errorf("expected default collection=memory, got %q", capturedCollection)
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

	h := handlers.WriteDocument(q, nil, nil, zerolog.Nop(), testMaxFileSize)
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if capturedHash != expectedHash {
		t.Errorf("expected hash %s, got %s", expectedHash, capturedHash)
	}
}
