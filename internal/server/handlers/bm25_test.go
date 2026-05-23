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
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type mockBM25Querier struct {
	bm25SearchFn         func(ctx context.Context, arg sqlc.BM25SearchParams) ([]sqlc.BM25SearchRow, error)
	bm25SearchWithTagsFn func(ctx context.Context, arg sqlc.BM25SearchWithTagsParams) ([]sqlc.BM25SearchWithTagsRow, error)
}

func (m *mockBM25Querier) BM25Search(ctx context.Context, arg sqlc.BM25SearchParams) ([]sqlc.BM25SearchRow, error) {
	return m.bm25SearchFn(ctx, arg)
}

func (m *mockBM25Querier) BM25SearchWithTags(ctx context.Context, arg sqlc.BM25SearchWithTagsParams) ([]sqlc.BM25SearchWithTagsRow, error) {
	return m.bm25SearchWithTagsFn(ctx, arg)
}

func newBM25Context(e *echo.Echo, body string, workspace string) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if workspace != "" {
		c.Set("workspace", workspace)
	}
	return c, rec
}

func TestBM25Search_Success(t *testing.T) {
	chunkID := uuid.New()
	docID := uuid.New()
	now := time.Now()

	q := &mockBM25Querier{
		bm25SearchFn: func(_ context.Context, arg sqlc.BM25SearchParams) ([]sqlc.BM25SearchRow, error) {
			if arg.WorkspaceHash != "ws1" {
				t.Errorf("expected workspace ws1, got %q", arg.WorkspaceHash)
			}
			if arg.Query != "hello world" {
				t.Errorf("expected query 'hello world', got %q", arg.Query)
			}
			if arg.MaxResults != 10 {
				t.Errorf("expected max_results 10, got %d", arg.MaxResults)
			}
			return []sqlc.BM25SearchRow{
				{
					ID:            chunkID,
					DocumentID:    docID,
					WorkspaceHash: "ws1",
					Content:       "test bm25 content",
					ChunkIndex:    0,
					SourcePath:    "/test.md",
					Title:         "Test Doc",
					Collection:    "memory",
					Tags:          []string{"tag1"},
					CreatedAt:     now,
					UpdatedAt:     now,
					Score:         0.85,
				},
			}, nil
		},
	}

	e := echo.New()
	c, rec := newBM25Context(e, `{"query":"hello world","workspace":"ws1"}`, "ws1")

	h := handlers.BM25Search(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp handlers.SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 {
		t.Errorf("expected total=1, got %d", resp.Total)
	}
	if resp.Results[0].ID != chunkID.String() {
		t.Errorf("expected id=%s, got %s", chunkID, resp.Results[0].ID)
	}
	if resp.Results[0].Score < 0.84 || resp.Results[0].Score > 0.86 {
		t.Errorf("expected score ~0.85, got %f", resp.Results[0].Score)
	}
	if resp.Results[0].DocumentID != docID.String() {
		t.Errorf("expected document_id=%s, got %s", docID, resp.Results[0].DocumentID)
	}
	if resp.QueryMs < 0 {
		t.Error("expected non-negative query_ms")
	}
}

func TestBM25Search_EmptyResults(t *testing.T) {
	q := &mockBM25Querier{
		bm25SearchFn: func(_ context.Context, _ sqlc.BM25SearchParams) ([]sqlc.BM25SearchRow, error) {
			return []sqlc.BM25SearchRow{}, nil
		},
	}

	e := echo.New()
	c, rec := newBM25Context(e, `{"query":"nothing","workspace":"ws1"}`, "ws1")

	h := handlers.BM25Search(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp handlers.SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 0 {
		t.Errorf("expected total=0, got %d", resp.Total)
	}
	if resp.Results == nil {
		t.Error("expected non-nil results array")
	}
}

func TestBM25Search_MissingQuery(t *testing.T) {
	q := &mockBM25Querier{}

	e := echo.New()
	c, _ := newBM25Context(e, `{"workspace":"ws1"}`, "ws1")

	h := handlers.BM25Search(q, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error for missing query")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", he.Code)
	}
}

func TestBM25Search_DefaultMaxResults(t *testing.T) {
	var capturedMax int32
	q := &mockBM25Querier{
		bm25SearchFn: func(_ context.Context, arg sqlc.BM25SearchParams) ([]sqlc.BM25SearchRow, error) {
			capturedMax = arg.MaxResults
			return nil, nil
		},
	}

	e := echo.New()
	c, _ := newBM25Context(e, `{"query":"test","workspace":"ws1"}`, "ws1")

	h := handlers.BM25Search(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if capturedMax != 10 {
		t.Errorf("expected default max_results=10, got %d", capturedMax)
	}
}

func TestBM25Search_MaxResultsCapped(t *testing.T) {
	var capturedMax int32
	q := &mockBM25Querier{
		bm25SearchFn: func(_ context.Context, arg sqlc.BM25SearchParams) ([]sqlc.BM25SearchRow, error) {
			capturedMax = arg.MaxResults
			return nil, nil
		},
	}

	e := echo.New()
	c, _ := newBM25Context(e, `{"query":"test","max_results":200,"workspace":"ws1"}`, "ws1")

	h := handlers.BM25Search(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if capturedMax != 100 {
		t.Errorf("expected capped max_results=100, got %d", capturedMax)
	}
}

func TestBM25Search_DBError(t *testing.T) {
	q := &mockBM25Querier{
		bm25SearchFn: func(_ context.Context, _ sqlc.BM25SearchParams) ([]sqlc.BM25SearchRow, error) {
			return nil, errors.New("db down")
		},
	}

	e := echo.New()
	c, _ := newBM25Context(e, `{"query":"test","workspace":"ws1"}`, "ws1")

	h := handlers.BM25Search(q, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error for DB failure")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if he.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", he.Code)
	}
}

func TestBM25Search_SnippetTruncation(t *testing.T) {
	longContent := strings.Repeat("a", 1000)

	q := &mockBM25Querier{
		bm25SearchFn: func(_ context.Context, _ sqlc.BM25SearchParams) ([]sqlc.BM25SearchRow, error) {
			return []sqlc.BM25SearchRow{
				{
					ID:            uuid.New(),
					DocumentID:    uuid.New(),
					WorkspaceHash: "ws1",
					Content:       longContent,
					SourcePath:    "/long.md",
					Collection:    "default",
					Tags:          nil,
					Score:         0.5,
				},
			}, nil
		},
	}

	e := echo.New()
	c, rec := newBM25Context(e, `{"query":"test","workspace":"ws1"}`, "ws1")

	h := handlers.BM25Search(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	var resp handlers.SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results[0].Content) != 700 {
		t.Errorf("expected content truncated to 700 chars, got %d", len(resp.Results[0].Content))
	}
}

func TestBM25Search_MissingWorkspace(t *testing.T) {
	q := &mockBM25Querier{}

	e := echo.New()
	c, _ := newBM25Context(e, `{"query":"test"}`, "")

	h := handlers.BM25Search(q, zerolog.Nop())
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

func TestBM25Search_WithTags(t *testing.T) {
	chunkID := uuid.New()
	docID := uuid.New()
	var capturedTags []string

	q := &mockBM25Querier{
		bm25SearchWithTagsFn: func(_ context.Context, arg sqlc.BM25SearchWithTagsParams) ([]sqlc.BM25SearchWithTagsRow, error) {
			capturedTags = arg.Tags
			return []sqlc.BM25SearchWithTagsRow{
				{
					ID:            chunkID,
					DocumentID:    docID,
					WorkspaceHash: "ws1",
					Content:       "tagged content",
					SourcePath:    "/tagged.md",
					Collection:    "memory",
					Tags:          []string{"decision"},
					Score:         0.9,
				},
			}, nil
		},
	}

	e := echo.New()
	c, rec := newBM25Context(e, `{"query":"test","tags":["decision"],"workspace":"ws1"}`, "ws1")

	h := handlers.BM25Search(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if len(capturedTags) != 1 || capturedTags[0] != "decision" {
		t.Errorf("expected tags=[decision], got %v", capturedTags)
	}

	var resp handlers.SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 {
		t.Errorf("expected total=1, got %d", resp.Total)
	}
	if resp.Results[0].Tags != "decision" {
		t.Errorf("expected tags=decision, got %q", resp.Results[0].Tags)
	}
}

func TestBM25Search_UTF8Truncation(t *testing.T) {
	content := strings.Repeat("\u4e16\u754c", 400)

	q := &mockBM25Querier{
		bm25SearchFn: func(_ context.Context, _ sqlc.BM25SearchParams) ([]sqlc.BM25SearchRow, error) {
			return []sqlc.BM25SearchRow{
				{
					ID:            uuid.New(),
					DocumentID:    uuid.New(),
					WorkspaceHash: "ws1",
					Content:       content,
					SourcePath:    "/utf8.md",
					Collection:    "default",
					Score:         0.5,
				},
			}, nil
		},
	}

	e := echo.New()
	c, rec := newBM25Context(e, `{"query":"test","workspace":"ws1"}`, "ws1")

	h := handlers.BM25Search(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	var resp handlers.SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	runes := []rune(resp.Results[0].Content)
	if len(runes) != 700 {
		t.Errorf("expected 700 runes after truncation, got %d", len(runes))
	}
}
