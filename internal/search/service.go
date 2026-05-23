package search

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	pgvector_go "github.com/pgvector/pgvector-go"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

type Querier interface {
	BM25Search(ctx context.Context, arg sqlc.BM25SearchParams) ([]sqlc.BM25SearchRow, error)
	BM25SearchAll(ctx context.Context, arg sqlc.BM25SearchAllParams) ([]sqlc.BM25SearchAllRow, error)
	VectorSearch(ctx context.Context, arg sqlc.VectorSearchParams) ([]sqlc.VectorSearchRow, error)
	VectorSearchAll(ctx context.Context, arg sqlc.VectorSearchAllParams) ([]sqlc.VectorSearchAllRow, error)
}

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Dimension() int
}

type SearchService struct {
	queries     Querier
	embedder    Embedder
	config      config.SearchConfig
	configMutex sync.RWMutex
	logger      zerolog.Logger
}

func NewSearchService(queries Querier, embedder Embedder, cfg config.SearchConfig, logger zerolog.Logger) *SearchService {
	return &SearchService{queries: queries, embedder: embedder, config: cfg, logger: logger}
}

// UpdateConfig updates the search configuration with thread-safe locking.
// TODO(story-8.2): validate cfg before accepting — callers must ensure
// RrfK >= 1, RecencyWeight in [0,1], RecencyHalfLifeDays >= 1, Limit >= 1.
func (s *SearchService) UpdateConfig(cfg config.SearchConfig) {
	s.configMutex.Lock()
	defer s.configMutex.Unlock()
	s.config = cfg
}

func (s *SearchService) HybridSearch(ctx context.Context, query string, workspace string, maxResults int) ([]Result, error) {
	// Read config under lock at the start, then release before I/O
	s.configMutex.RLock()
	rrfK := s.config.RrfK
	recencyWeight := s.config.RecencyWeight
	recencyHalfLifeDays := s.config.RecencyHalfLifeDays
	s.configMutex.RUnlock()

	fetchLimit := int32(maxResults * 3)
	if fetchLimit < 30 {
		fetchLimit = 30
	}

	var (
		bm25Results   []Result
		vectorResults []Result
		bm25Err       error
		vectorErr     error
	)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if workspace == "all" {
			rows, err := s.queries.BM25SearchAll(gctx, sqlc.BM25SearchAllParams{
				Query:      query,
				MaxResults: fetchLimit,
			})
			if err != nil {
				bm25Err = err
				s.logger.Warn().Err(err).Msg("bm25 leg failed, degrading")
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
		} else {
			rows, err := s.queries.BM25Search(gctx, sqlc.BM25SearchParams{
				Query:         query,
				WorkspaceHash: workspace,
				MaxResults:    fetchLimit,
			})
			if err != nil {
				bm25Err = err
				s.logger.Warn().Err(err).Msg("bm25 leg failed, degrading")
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
		}
		return nil
	})

	g.Go(func() error {
		if s.embedder == nil {
			return nil
		}
		vec, err := s.embedder.Embed(gctx, query)
		if err != nil {
			vectorErr = err
			s.logger.Warn().Err(err).Msg("embed failed, degrading")
			return nil
		}
		if workspace == "all" {
			rows, err := s.queries.VectorSearchAll(gctx, sqlc.VectorSearchAllParams{
				QueryEmbedding: pgvector_go.NewVector(vec),
				MaxResults:     fetchLimit,
			})
			if err != nil {
				vectorErr = err
				s.logger.Warn().Err(err).Msg("vector search leg failed, degrading")
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
		} else {
			rows, err := s.queries.VectorSearch(gctx, sqlc.VectorSearchParams{
				QueryEmbedding: pgvector_go.NewVector(vec),
				WorkspaceHash:  workspace,
				MaxResults:     fetchLimit,
			})
			if err != nil {
				vectorErr = err
				s.logger.Warn().Err(err).Msg("vector search leg failed, degrading")
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
		}
		return nil
	})

	_ = g.Wait()

	if bm25Err != nil && vectorErr != nil {
		return nil, fmt.Errorf("all search legs failed: bm25: %v, vector: %v", bm25Err, vectorErr)
	}

	merged := RRFMerge(bm25Results, vectorResults, rrfK)

	boosted := ApplyRecencyBoost(merged, recencyWeight, recencyHalfLifeDays, time.Now())

	if len(boosted) > maxResults {
		boosted = boosted[:maxResults]
	}

	return boosted, nil
}
