package search

import (
	"context"
	"testing"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type mockQuerier struct{}

func (m *mockQuerier) BM25Search(ctx context.Context, arg sqlc.BM25SearchParams) ([]sqlc.BM25SearchRow, error) {
	return []sqlc.BM25SearchRow{}, nil
}

func (m *mockQuerier) VectorSearch(ctx context.Context, arg sqlc.VectorSearchParams) ([]sqlc.VectorSearchRow, error) {
	return []sqlc.VectorSearchRow{}, nil
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

func TestUpdateConfig_ThreadSafe(t *testing.T) {
	logger := zerolog.Nop()
	initialCfg := config.SearchConfig{
		RrfK:                60,
		RecencyWeight:       0.3,
		RecencyHalfLifeDays: 180,
		Limit:               20,
	}

	service := NewSearchService(&mockQuerier{}, &mockEmbedder{}, initialCfg, logger)

	for i := 0; i < 10; i++ {
		newCfg := config.SearchConfig{
			RrfK:                float64(50 + i),
			RecencyWeight:       0.3,
			RecencyHalfLifeDays: 180,
			Limit:               20,
		}
		service.UpdateConfig(newCfg)
	}

	service.configMutex.RLock()
	defer service.configMutex.RUnlock()
	if service.config.RrfK != 59 {
		t.Errorf("final RrfK should be 59, got %v", service.config.RrfK)
	}
}
