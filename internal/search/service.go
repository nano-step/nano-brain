package search

import (
	"context"
	"time"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	pgvector_go "github.com/pgvector/pgvector-go"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

type Querier interface {
	BM25Search(ctx context.Context, arg sqlc.BM25SearchParams) ([]sqlc.BM25SearchRow, error)
	VectorSearch(ctx context.Context, arg sqlc.VectorSearchParams) ([]sqlc.VectorSearchRow, error)
}

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Dimension() int
}

type SearchService struct {
	queries  Querier
	embedder Embedder
	config   config.SearchConfig
	logger   zerolog.Logger
}

func NewSearchService(queries Querier, embedder Embedder, cfg config.SearchConfig, logger zerolog.Logger) *SearchService {
	return &SearchService{queries: queries, embedder: embedder, config: cfg, logger: logger}
}

func (s *SearchService) HybridSearch(ctx context.Context, query string, workspace string, maxResults int) ([]Result, error) {
	fetchLimit := int32(maxResults * 3)
	if fetchLimit < 30 {
		fetchLimit = 30
	}

	var (
		bm25Results   []Result
		vectorResults []Result
	)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		rows, err := s.queries.BM25Search(gctx, sqlc.BM25SearchParams{
			Query:         query,
			WorkspaceHash: workspace,
			MaxResults:    fetchLimit,
		})
		if err != nil {
			s.logger.Warn().Err(err).Msg("bm25 leg failed, continuing with vector only")
			return nil
		}
		bm25Results = make([]Result, 0, len(rows))
		for _, r := range rows {
			bm25Results = append(bm25Results, Result{
				ID:            r.ID.String(),
				DocumentID:    r.DocumentID.String(),
				WorkspaceHash: r.WorkspaceHash,
				Title:         r.Title,
				Content:       r.Content,
				Score:         r.Score,
				Tags:          r.Tags,
				Collection:    r.Collection,
				SourcePath:    r.SourcePath,
				CreatedAt:     r.CreatedAt,
				UpdatedAt:     r.UpdatedAt,
			})
		}
		return nil
	})

	g.Go(func() error {
		if s.embedder == nil {
			s.logger.Warn().Msg("no embedder configured, skipping vector leg")
			return nil
		}
		vec, err := s.embedder.Embed(gctx, query)
		if err != nil {
			s.logger.Warn().Err(err).Msg("embed failed, continuing with bm25 only")
			return nil
		}
		rows, err := s.queries.VectorSearch(gctx, sqlc.VectorSearchParams{
			QueryEmbedding: pgvector_go.NewVector(vec),
			WorkspaceHash:  workspace,
			MaxResults:     fetchLimit,
		})
		if err != nil {
			s.logger.Warn().Err(err).Msg("vector search leg failed, continuing with bm25 only")
			return nil
		}
		vectorResults = make([]Result, 0, len(rows))
		for _, r := range rows {
			vectorResults = append(vectorResults, Result{
				ID:            r.ChunkID.String(),
				DocumentID:    r.DocumentID.String(),
				WorkspaceHash: r.WorkspaceHash,
				Title:         r.Title,
				Content:       r.Content,
				Score:         r.Score,
				Tags:          r.Tags,
				Collection:    r.Collection,
				SourcePath:    r.SourcePath,
				CreatedAt:     r.CreatedAt,
				UpdatedAt:     r.UpdatedAt,
			})
		}
		return nil
	})

	_ = g.Wait()

	merged := RRFMerge(bm25Results, vectorResults, s.config.RrfK)

	boosted := ApplyRecencyBoost(merged, s.config.RecencyWeight, s.config.RecencyHalfLifeDays, time.Now())

	if len(boosted) > maxResults {
		boosted = boosted[:maxResults]
	}

	return boosted, nil
}
