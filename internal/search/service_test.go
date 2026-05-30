package search

import (
	"context"
	"sync"
	"testing"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type mockQuerier struct {
	bm25WithTagsCalled        bool
	bm25AllWithTagsCalled     bool
	vectorWithTagsCalled      bool
	vectorAllWithTagsCalled   bool
	capturedTags              []string
}

func (m *mockQuerier) BM25Search(ctx context.Context, arg sqlc.BM25SearchParams) ([]sqlc.BM25SearchRow, error) {
	return []sqlc.BM25SearchRow{}, nil
}

func (m *mockQuerier) BM25SearchAll(ctx context.Context, arg sqlc.BM25SearchAllParams) ([]sqlc.BM25SearchAllRow, error) {
	return []sqlc.BM25SearchAllRow{}, nil
}

func (m *mockQuerier) BM25SearchWithTags(ctx context.Context, arg sqlc.BM25SearchWithTagsParams) ([]sqlc.BM25SearchWithTagsRow, error) {
	m.bm25WithTagsCalled = true
	m.capturedTags = arg.Tags
	return []sqlc.BM25SearchWithTagsRow{}, nil
}

func (m *mockQuerier) BM25SearchAllWithTags(ctx context.Context, arg sqlc.BM25SearchAllWithTagsParams) ([]sqlc.BM25SearchAllWithTagsRow, error) {
	m.bm25AllWithTagsCalled = true
	m.capturedTags = arg.Tags
	return []sqlc.BM25SearchAllWithTagsRow{}, nil
}

func (m *mockQuerier) VectorSearch(ctx context.Context, arg sqlc.VectorSearchParams) ([]sqlc.VectorSearchRow, error) {
	return []sqlc.VectorSearchRow{}, nil
}

func (m *mockQuerier) VectorSearchAll(ctx context.Context, arg sqlc.VectorSearchAllParams) ([]sqlc.VectorSearchAllRow, error) {
	return []sqlc.VectorSearchAllRow{}, nil
}

func (m *mockQuerier) VectorSearchWithTags(ctx context.Context, arg sqlc.VectorSearchWithTagsParams) ([]sqlc.VectorSearchWithTagsRow, error) {
	m.vectorWithTagsCalled = true
	return []sqlc.VectorSearchWithTagsRow{}, nil
}

func (m *mockQuerier) VectorSearchAllWithTags(ctx context.Context, arg sqlc.VectorSearchAllWithTagsParams) ([]sqlc.VectorSearchAllWithTagsRow, error) {
	m.vectorAllWithTagsCalled = true
	return []sqlc.VectorSearchAllWithTagsRow{}, nil
}

type mockEmbedder struct{}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return make([]float32, 1536), nil
}

func (m *mockEmbedder) Dimension() int {
	return 1536
}

func TestUpdateConfig_ChangesAppliedToSubsequentSearches(t *testing.T) {
	logger := zerolog.Nop()
	initialCfg := config.SearchConfig{
		RrfK:                60,
		RecencyWeight:       0.3,
		RecencyHalfLifeDays: 180,
		Limit:               20,
	}

	service := NewSearchService(&mockQuerier{}, &mockEmbedder{}, initialCfg, logger)

	service.configMutex.RLock()
	if service.config.RrfK != 60 {
		t.Errorf("initial RrfK should be 60, got %v", service.config.RrfK)
	}
	service.configMutex.RUnlock()

	newCfg := config.SearchConfig{
		RrfK:                45,
		RecencyWeight:       0.5,
		RecencyHalfLifeDays: 90,
		Limit:               15,
	}
	service.UpdateConfig(newCfg)

	service.configMutex.RLock()
	if service.config.RrfK != 45 {
		t.Errorf("updated RrfK should be 45, got %v", service.config.RrfK)
	}
	if service.config.RecencyWeight != 0.5 {
		t.Errorf("updated RecencyWeight should be 0.5, got %v", service.config.RecencyWeight)
	}
	if service.config.RecencyHalfLifeDays != 90 {
		t.Errorf("updated RecencyHalfLifeDays should be 90, got %d", service.config.RecencyHalfLifeDays)
	}
	if service.config.Limit != 15 {
		t.Errorf("updated Limit should be 15, got %d", service.config.Limit)
	}
	service.configMutex.RUnlock()
}

func TestUpdateConfig_ConcurrentReadersAndWriters(t *testing.T) {
	logger := zerolog.Nop()
	initialCfg := config.SearchConfig{
		RrfK:                60,
		RecencyWeight:       0.3,
		RecencyHalfLifeDays: 180,
		Limit:               20,
	}

	service := NewSearchService(&mockQuerier{}, &mockEmbedder{}, initialCfg, logger)
	ctx := context.Background()

	var wg sync.WaitGroup
	errors := make(chan error, 20)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			newCfg := config.SearchConfig{
				RrfK:                float64(50 + idx),
				RecencyWeight:       0.3,
				RecencyHalfLifeDays: 180,
				Limit:               20,
			}
			service.UpdateConfig(newCfg)
		}(i)
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.HybridSearch(ctx, "test", "workspace", 10, nil)
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent operation failed: %v", err)
	}
}

func TestHybridSearch_WithTags_DispatchesToWithTagsQueries(t *testing.T) {
	logger := zerolog.Nop()
	cfg := config.SearchConfig{RrfK: 60, RecencyWeight: 0.3, RecencyHalfLifeDays: 180, Limit: 20}
	q := &mockQuerier{}
	service := NewSearchService(q, &mockEmbedder{}, cfg, logger)

	_, err := service.HybridSearch(context.Background(), "test", "ws1", 10, []string{"decision", "auth"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !q.bm25WithTagsCalled {
		t.Error("expected BM25SearchWithTags to be called when tags provided with workspace")
	}
	if q.bm25AllWithTagsCalled {
		t.Error("BM25SearchAllWithTags should not be called for specific workspace")
	}
	if len(q.capturedTags) != 2 || q.capturedTags[0] != "decision" || q.capturedTags[1] != "auth" {
		t.Errorf("expected tags=[decision,auth], got %v", q.capturedTags)
	}
	if !q.vectorWithTagsCalled {
		t.Error("expected VectorSearchWithTags to be called when tags provided with workspace")
	}
	if q.vectorAllWithTagsCalled {
		t.Error("VectorSearchAllWithTags should not be called for specific workspace")
	}
}

func TestHybridSearch_WithTags_ScopeAll_DispatchesToAllWithTagsQueries(t *testing.T) {
	logger := zerolog.Nop()
	cfg := config.SearchConfig{RrfK: 60, RecencyWeight: 0.3, RecencyHalfLifeDays: 180, Limit: 20}
	q := &mockQuerier{}
	service := NewSearchService(q, &mockEmbedder{}, cfg, logger)

	_, err := service.HybridSearch(context.Background(), "test", "all", 10, []string{"decision"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !q.bm25AllWithTagsCalled {
		t.Error("expected BM25SearchAllWithTags to be called when tags provided with scope=all")
	}
	if q.bm25WithTagsCalled {
		t.Error("BM25SearchWithTags should not be called for scope=all")
	}
	if !q.vectorAllWithTagsCalled {
		t.Error("expected VectorSearchAllWithTags to be called when tags provided with scope=all")
	}
	if q.vectorWithTagsCalled {
		t.Error("VectorSearchWithTags should not be called for scope=all")
	}
}

func TestHybridSearch_NoTags_DispatchesToBaseQueries(t *testing.T) {
	logger := zerolog.Nop()
	cfg := config.SearchConfig{RrfK: 60, RecencyWeight: 0.3, RecencyHalfLifeDays: 180, Limit: 20}
	q := &mockQuerier{}
	service := NewSearchService(q, &mockEmbedder{}, cfg, logger)

	_, err := service.HybridSearch(context.Background(), "test", "ws1", 10, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if q.bm25WithTagsCalled || q.bm25AllWithTagsCalled {
		t.Error("WithTags queries should not be called when tags is nil")
	}
	if q.vectorWithTagsCalled || q.vectorAllWithTagsCalled {
		t.Error("VectorWithTags queries should not be called when tags is nil")
	}
}
