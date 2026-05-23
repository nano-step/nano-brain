package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
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

type mockEmbedQuerier struct {
	getPendingChunksFn  func(ctx context.Context, arg sqlc.GetPendingChunksParams) ([]sqlc.Chunk, error)
	insertEmbeddingFn   func(ctx context.Context, arg sqlc.InsertEmbeddingParams) (sqlc.Embedding, error)
	markChunkEmbeddedFn func(ctx context.Context, arg sqlc.MarkChunkEmbeddedParams) error
	countPendingFn      func(ctx context.Context, ws string) (int64, error)
	resetEmbedStatusFn  func(ctx context.Context, ws string) error
	resetCalled         bool
}

func (m *mockEmbedQuerier) GetPendingChunks(ctx context.Context, arg sqlc.GetPendingChunksParams) ([]sqlc.Chunk, error) {
	if m.getPendingChunksFn != nil {
		return m.getPendingChunksFn(ctx, arg)
	}
	return nil, nil
}

func (m *mockEmbedQuerier) InsertEmbedding(ctx context.Context, arg sqlc.InsertEmbeddingParams) (sqlc.Embedding, error) {
	if m.insertEmbeddingFn != nil {
		return m.insertEmbeddingFn(ctx, arg)
	}
	return sqlc.Embedding{}, nil
}

func (m *mockEmbedQuerier) MarkChunkEmbedded(ctx context.Context, arg sqlc.MarkChunkEmbeddedParams) error {
	if m.markChunkEmbeddedFn != nil {
		return m.markChunkEmbeddedFn(ctx, arg)
	}
	return nil
}

func (m *mockEmbedQuerier) CountPendingChunks(ctx context.Context, ws string) (int64, error) {
	if m.countPendingFn != nil {
		return m.countPendingFn(ctx, ws)
	}
	return 0, nil
}

func (m *mockEmbedQuerier) ResetEmbedStatus(ctx context.Context, ws string) error {
	m.resetCalled = true
	if m.resetEmbedStatusFn != nil {
		return m.resetEmbedStatusFn(ctx, ws)
	}
	return nil
}

func newEmbedContext(e *echo.Echo, body, workspace string) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/embed", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if workspace != "" {
		c.Set("workspace", workspace)
	}
	return c, rec
}

func makeChunks(n int, ws string) []sqlc.Chunk {
	chunks := make([]sqlc.Chunk, n)
	for i := range chunks {
		chunks[i] = sqlc.Chunk{
			ID:            uuid.New(),
			WorkspaceHash: ws,
			Content:       fmt.Sprintf("content-%d", i),
		}
	}
	return chunks
}

func TestTriggerEmbed_Success(t *testing.T) {
	chunks := makeChunks(3, "ws1")
	q := &mockEmbedQuerier{
		getPendingChunksFn: func(_ context.Context, arg sqlc.GetPendingChunksParams) ([]sqlc.Chunk, error) {
			return chunks, nil
		},
	}
	emb := &mockEmbedder{
		embedFn:   func(_ context.Context, _ string) ([]float32, error) { return testVector(), nil },
		dimension: 3,
	}

	e := echo.New()
	c, rec := newEmbedContext(e, `{"workspace":"ws1"}`, "ws1")

	h := handlers.TriggerEmbed(q, emb, "test-provider", "test-model", zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp handlers.EmbedResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Embedded != 3 {
		t.Errorf("embedded = %d, want 3", resp.Embedded)
	}
	if resp.Remaining != 0 {
		t.Errorf("remaining = %d, want 0", resp.Remaining)
	}
}

func TestTriggerEmbed_NoPending(t *testing.T) {
	q := &mockEmbedQuerier{}
	emb := &mockEmbedder{
		embedFn:   func(_ context.Context, _ string) ([]float32, error) { return testVector(), nil },
		dimension: 3,
	}

	e := echo.New()
	c, rec := newEmbedContext(e, `{"workspace":"ws1"}`, "ws1")

	h := handlers.TriggerEmbed(q, emb, "p", "m", zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	var resp handlers.EmbedResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Embedded != 0 {
		t.Errorf("embedded = %d, want 0", resp.Embedded)
	}
	if resp.Remaining != 0 {
		t.Errorf("remaining = %d, want 0", resp.Remaining)
	}
}

func TestTriggerEmbed_NoEmbedder(t *testing.T) {
	q := &mockEmbedQuerier{}

	e := echo.New()
	c, _ := newEmbedContext(e, `{"workspace":"ws1"}`, "ws1")

	h := handlers.TriggerEmbed(q, nil, "p", "m", zerolog.Nop())
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

func TestTriggerEmbed_Force(t *testing.T) {
	q := &mockEmbedQuerier{
		getPendingChunksFn: func(_ context.Context, _ sqlc.GetPendingChunksParams) ([]sqlc.Chunk, error) {
			return makeChunks(2, "ws1"), nil
		},
	}
	emb := &mockEmbedder{
		embedFn:   func(_ context.Context, _ string) ([]float32, error) { return testVector(), nil },
		dimension: 3,
	}

	e := echo.New()
	c, rec := newEmbedContext(e, `{"workspace":"ws1","force":true}`, "ws1")

	h := handlers.TriggerEmbed(q, emb, "p", "m", zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !q.resetCalled {
		t.Error("expected ResetEmbedStatus to be called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp handlers.EmbedResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Embedded != 2 {
		t.Errorf("embedded = %d, want 2", resp.Embedded)
	}
}

func TestTriggerEmbed_PartialFailure(t *testing.T) {
	chunks := makeChunks(3, "ws1")
	callCount := 0
	q := &mockEmbedQuerier{
		getPendingChunksFn: func(_ context.Context, _ sqlc.GetPendingChunksParams) ([]sqlc.Chunk, error) {
			return chunks, nil
		},
	}
	emb := &mockEmbedder{
		embedFn: func(_ context.Context, _ string) ([]float32, error) {
			callCount++
			if callCount == 2 {
				return nil, fmt.Errorf("provider error")
			}
			return testVector(), nil
		},
		dimension: 3,
	}

	e := echo.New()
	c, rec := newEmbedContext(e, `{"workspace":"ws1"}`, "ws1")

	h := handlers.TriggerEmbed(q, emb, "p", "m", zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp handlers.EmbedResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Embedded != 1 {
		t.Errorf("embedded = %d, want 1", resp.Embedded)
	}
	if resp.Remaining != 2 {
		t.Errorf("remaining = %d, want 2", resp.Remaining)
	}
}

func TestTriggerEmbed_HasMore(t *testing.T) {
	chunks := makeChunks(51, "ws1")
	q := &mockEmbedQuerier{
		getPendingChunksFn: func(_ context.Context, _ sqlc.GetPendingChunksParams) ([]sqlc.Chunk, error) {
			return chunks, nil
		},
		countPendingFn: func(_ context.Context, _ string) (int64, error) {
			return 120, nil
		},
	}
	emb := &mockEmbedder{
		embedFn:   func(_ context.Context, _ string) ([]float32, error) { return testVector(), nil },
		dimension: 3,
	}

	e := echo.New()
	c, rec := newEmbedContext(e, `{"workspace":"ws1"}`, "ws1")

	h := handlers.TriggerEmbed(q, emb, "p", "m", zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp handlers.EmbedResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Embedded != 50 {
		t.Errorf("embedded = %d, want 50", resp.Embedded)
	}
	if resp.Remaining != 120 {
		t.Errorf("remaining = %d, want 120", resp.Remaining)
	}
}
