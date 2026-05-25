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

type mockEmbedder struct {
	embedFn    func(ctx context.Context, text string) ([]float32, error)
	dimension  int
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return m.embedFn(ctx, text)
}

func (m *mockEmbedder) Dimension() int { return m.dimension }

type mockVSearchQuerier struct {
	vectorSearchFn func(ctx context.Context, arg sqlc.VectorSearchParams) ([]sqlc.VectorSearchRow, error)
}

func (m *mockVSearchQuerier) VectorSearch(ctx context.Context, arg sqlc.VectorSearchParams) ([]sqlc.VectorSearchRow, error) {
	return m.vectorSearchFn(ctx, arg)
}

func newVSearchContext(e *echo.Echo, body string, workspace string) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vsearch", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if workspace != "" {
		c.Set("workspace", workspace)
	}
	return c, rec
}

func testVector() []float32 { return []float32{0.1, 0.2, 0.3} }

func TestVSearch_Success(t *testing.T) {
	docID := uuid.New()
	embID := uuid.New()
	now := time.Now().Truncate(time.Second)

	emb := &mockEmbedder{
		embedFn:   func(_ context.Context, _ string) ([]float32, error) { return testVector(), nil },
		dimension: 3,
	}
	q := &mockVSearchQuerier{
		vectorSearchFn: func(_ context.Context, arg sqlc.VectorSearchParams) ([]sqlc.VectorSearchRow, error) {
			if arg.WorkspaceHash != "ws1" {
				t.Errorf("expected workspace ws1, got %q", arg.WorkspaceHash)
			}
			if arg.MaxResults != 10 {
				t.Errorf("expected max_results 10, got %d", arg.MaxResults)
			}
			return []sqlc.VectorSearchRow{
				{
					ID:            embID,
					ChunkID:       uuid.New(),
					WorkspaceHash: "ws1",
					Content:       "test content",
					DocumentID:    docID,
					SourcePath:    "/test.md",
					Title:         "Test Doc",
					Collection:    "memory",
					Tags:          []string{"tag1", "tag2"},
					CreatedAt:     now,
					UpdatedAt:     now,
					Score:         0.95,
				},
			}, nil
		},
	}

	e := echo.New()
	body := `{"query":"hello","workspace":"ws1"}`
	c, rec := newVSearchContext(e, body, "ws1")

	h := handlers.VectorSearch(q, emb, zerolog.Nop())
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
	r := resp.Results[0]
	if r.ID != embID.String() {
		t.Errorf("expected id=%s, got %s", embID, r.ID)
	}
	if r.Title != "Test Doc" {
		t.Errorf("expected title=Test Doc, got %q", r.Title)
	}
	if r.Snippet != "test content" {
		t.Errorf("expected snippet=test content, got %q", r.Snippet)
	}
	if r.Score != 0.95 {
		t.Errorf("expected score=0.95, got %f", r.Score)
	}
	if len(r.Tags) != 2 || r.Tags[0] != "tag1" || r.Tags[1] != "tag2" {
		t.Errorf("expected tags=[tag1,tag2], got %v", r.Tags)
	}
	if r.DocumentID != docID.String() {
		t.Errorf("expected document_id=%s, got %s", docID, r.DocumentID)
	}
	if resp.QueryMs < 0 {
		t.Error("expected non-negative query_ms")
	}
}

func TestVSearch_EmptyResults(t *testing.T) {
	emb := &mockEmbedder{
		embedFn:   func(_ context.Context, _ string) ([]float32, error) { return testVector(), nil },
		dimension: 3,
	}
	q := &mockVSearchQuerier{
		vectorSearchFn: func(_ context.Context, _ sqlc.VectorSearchParams) ([]sqlc.VectorSearchRow, error) {
			return []sqlc.VectorSearchRow{}, nil
		},
	}

	e := echo.New()
	c, rec := newVSearchContext(e, `{"query":"nothing","workspace":"ws1"}`, "ws1")

	h := handlers.VectorSearch(q, emb, zerolog.Nop())
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
	if len(resp.Results) != 0 {
		t.Errorf("expected empty results, got %d", len(resp.Results))
	}
}

func TestVSearch_NilResults(t *testing.T) {
	emb := &mockEmbedder{
		embedFn:   func(_ context.Context, _ string) ([]float32, error) { return testVector(), nil },
		dimension: 3,
	}
	q := &mockVSearchQuerier{
		vectorSearchFn: func(_ context.Context, _ sqlc.VectorSearchParams) ([]sqlc.VectorSearchRow, error) {
			return nil, nil
		},
	}

	e := echo.New()
	c, rec := newVSearchContext(e, `{"query":"nothing","workspace":"ws1"}`, "ws1")

	h := handlers.VectorSearch(q, emb, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	var resp handlers.SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Results == nil {
		t.Error("expected non-nil results even when DB returns nil")
	}
}

func TestVSearch_MissingQuery(t *testing.T) {
	emb := &mockEmbedder{
		embedFn:   func(_ context.Context, _ string) ([]float32, error) { return testVector(), nil },
		dimension: 3,
	}
	q := &mockVSearchQuerier{}

	e := echo.New()
	c, _ := newVSearchContext(e, `{"workspace":"ws1"}`, "ws1")

	h := handlers.VectorSearch(q, emb, zerolog.Nop())
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

func TestVSearch_NoEmbedder(t *testing.T) {
	q := &mockVSearchQuerier{}

	e := echo.New()
	c, _ := newVSearchContext(e, `{"query":"hello","workspace":"ws1"}`, "ws1")

	h := handlers.VectorSearch(q, nil, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error for nil embedder")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", he.Code)
	}
}

func TestVSearch_DefaultMaxResults(t *testing.T) {
	var capturedMax int32
	emb := &mockEmbedder{
		embedFn:   func(_ context.Context, _ string) ([]float32, error) { return testVector(), nil },
		dimension: 3,
	}
	q := &mockVSearchQuerier{
		vectorSearchFn: func(_ context.Context, arg sqlc.VectorSearchParams) ([]sqlc.VectorSearchRow, error) {
			capturedMax = arg.MaxResults
			return nil, nil
		},
	}

	e := echo.New()
	c, _ := newVSearchContext(e, `{"query":"test","workspace":"ws1"}`, "ws1")

	h := handlers.VectorSearch(q, emb, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if capturedMax != 10 {
		t.Errorf("expected default max_results=10, got %d", capturedMax)
	}
}

func TestVSearch_MaxResultsCapped(t *testing.T) {
	var capturedMax int32
	emb := &mockEmbedder{
		embedFn:   func(_ context.Context, _ string) ([]float32, error) { return testVector(), nil },
		dimension: 3,
	}
	q := &mockVSearchQuerier{
		vectorSearchFn: func(_ context.Context, arg sqlc.VectorSearchParams) ([]sqlc.VectorSearchRow, error) {
			capturedMax = arg.MaxResults
			return nil, nil
		},
	}

	e := echo.New()
	c, _ := newVSearchContext(e, `{"query":"test","max_results":200,"workspace":"ws1"}`, "ws1")

	h := handlers.VectorSearch(q, emb, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if capturedMax != 100 {
		t.Errorf("expected capped max_results=100, got %d", capturedMax)
	}
}

func TestVSearch_EmbedError(t *testing.T) {
	emb := &mockEmbedder{
		embedFn: func(_ context.Context, _ string) ([]float32, error) {
			return nil, errors.New("embed failed")
		},
		dimension: 3,
	}
	q := &mockVSearchQuerier{}

	e := echo.New()
	c, _ := newVSearchContext(e, `{"query":"test","workspace":"ws1"}`, "ws1")

	h := handlers.VectorSearch(q, emb, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error for embed failure")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if he.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", he.Code)
	}
}

func TestVSearch_DBError(t *testing.T) {
	emb := &mockEmbedder{
		embedFn:   func(_ context.Context, _ string) ([]float32, error) { return testVector(), nil },
		dimension: 3,
	}
	q := &mockVSearchQuerier{
		vectorSearchFn: func(_ context.Context, _ sqlc.VectorSearchParams) ([]sqlc.VectorSearchRow, error) {
			return nil, errors.New("db down")
		},
	}

	e := echo.New()
	c, _ := newVSearchContext(e, `{"query":"test","workspace":"ws1"}`, "ws1")

	h := handlers.VectorSearch(q, emb, zerolog.Nop())
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
