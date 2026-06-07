package search

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	pgvector_go "github.com/pgvector/pgvector-go"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

type Querier interface {
	BM25Search(ctx context.Context, arg sqlc.BM25SearchParams) ([]sqlc.BM25SearchRow, error)
	BM25SearchAll(ctx context.Context, arg sqlc.BM25SearchAllParams) ([]sqlc.BM25SearchAllRow, error)
	BM25SearchWithTags(ctx context.Context, arg sqlc.BM25SearchWithTagsParams) ([]sqlc.BM25SearchWithTagsRow, error)
	BM25SearchAllWithTags(ctx context.Context, arg sqlc.BM25SearchAllWithTagsParams) ([]sqlc.BM25SearchAllWithTagsRow, error)
	VectorSearch(ctx context.Context, arg sqlc.VectorSearchParams) ([]sqlc.VectorSearchRow, error)
	VectorSearchAll(ctx context.Context, arg sqlc.VectorSearchAllParams) ([]sqlc.VectorSearchAllRow, error)
	VectorSearchWithTags(ctx context.Context, arg sqlc.VectorSearchWithTagsParams) ([]sqlc.VectorSearchWithTagsRow, error)
	VectorSearchAllWithTags(ctx context.Context, arg sqlc.VectorSearchAllWithTagsParams) ([]sqlc.VectorSearchAllWithTagsRow, error)
}

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Dimension() int
}

type PageRankLoader interface {
	LoadScores(ctx context.Context, workspace string) (map[string]float64, error)
}

type EntityQuerier interface {
	GetChunkIDsByEntityNames(ctx context.Context, arg sqlc.GetChunkIDsByEntityNamesParams) ([]uuid.UUID, error)
}

type SearchService struct {
	queries        Querier
	embedder       Embedder
	entityQuerier  EntityQuerier
	config         config.SearchConfig
	configMutex    sync.RWMutex
	logger         zerolog.Logger
	pagerankLoader PageRankLoader
}

func NewSearchService(queries Querier, embedder Embedder, cfg config.SearchConfig, logger zerolog.Logger) *SearchService {
	return &SearchService{queries: queries, embedder: embedder, config: cfg, logger: logger}
}

func (s *SearchService) SetEntityQuerier(eq EntityQuerier) {
	s.entityQuerier = eq
}

func (s *SearchService) SetPageRankLoader(loader PageRankLoader) {
	s.pagerankLoader = loader
}

// UpdateConfig updates the search configuration with thread-safe locking.
// TODO(story-8.2): validate cfg before accepting — callers must ensure
// RrfK >= 1, RecencyWeight in [0,1], RecencyHalfLifeDays >= 1, Limit >= 1.
func (s *SearchService) UpdateConfig(cfg config.SearchConfig) {
	s.configMutex.Lock()
	defer s.configMutex.Unlock()
	s.config = cfg
}

func (s *SearchService) DefaultLimit() int {
	s.configMutex.RLock()
	defer s.configMutex.RUnlock()
	return s.config.Limit
}

func (s *SearchService) HybridSearch(ctx context.Context, query string, workspace string, maxResults int, tags []string, timeRange *TimeRangeFilter, chunkType string) ([]Result, error) {
	s.configMutex.RLock()
	rrfK := s.config.RrfK
	recencyWeight := s.config.RecencyWeight
	recencyHalfLifeDays := s.config.RecencyHalfLifeDays
	s.configMutex.RUnlock()

	var chunkTypeNullStr sql.NullString
	if chunkType != "" {
		chunkTypeNullStr = sql.NullString{String: chunkType, Valid: true}
	}

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
			if len(tags) > 0 {
				ca, cb, ua, ub := timeRange.ToSqlNullTimes()
				rows, err := s.queries.BM25SearchAllWithTags(gctx, sqlc.BM25SearchAllWithTagsParams{
					Query:         query,
					Tags:          tags,
					ChunkType:     chunkTypeNullStr,
					MaxResults:    fetchLimit,
					CreatedAfter:  ca,
					CreatedBefore: cb,
					UpdatedAfter:  ua,
					UpdatedBefore: ub,
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
				ca, cb, ua, ub := timeRange.ToSqlNullTimes()
				rows, err := s.queries.BM25SearchAll(gctx, sqlc.BM25SearchAllParams{
					Query:         query,
					ChunkType:     chunkTypeNullStr,
					MaxResults:    fetchLimit,
					CreatedAfter:  ca,
					CreatedBefore: cb,
					UpdatedAfter:  ua,
					UpdatedBefore: ub,
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
		} else {
			if len(tags) > 0 {
				ca, cb, ua, ub := timeRange.ToSqlNullTimes()
				rows, err := s.queries.BM25SearchWithTags(gctx, sqlc.BM25SearchWithTagsParams{
					Query:         query,
					WorkspaceHash: workspace,
					Tags:          tags,
					ChunkType:     chunkTypeNullStr,
					MaxResults:    fetchLimit,
					CreatedAfter:  ca,
					CreatedBefore: cb,
					UpdatedAfter:  ua,
					UpdatedBefore: ub,
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
				ca, cb, ua, ub := timeRange.ToSqlNullTimes()
				rows, err := s.queries.BM25Search(gctx, sqlc.BM25SearchParams{
					Query:         query,
					WorkspaceHash: workspace,
					ChunkType:     chunkTypeNullStr,
					MaxResults:    fetchLimit,
					CreatedAfter:  ca,
					CreatedBefore: cb,
					UpdatedAfter:  ua,
					UpdatedBefore: ub,
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
			if len(tags) > 0 {
				ca, cb, ua, ub := timeRange.ToSqlNullTimes()
				rows, err := s.queries.VectorSearchAllWithTags(gctx, sqlc.VectorSearchAllWithTagsParams{
					QueryEmbedding: pgvector_go.NewVector(vec),
					Tags:           tags,
					MaxResults:     fetchLimit,
					CreatedAfter:   ca,
					CreatedBefore:  cb,
					UpdatedAfter:   ua,
					UpdatedBefore:  ub,
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
				ca, cb, ua, ub := timeRange.ToSqlNullTimes()
				rows, err := s.queries.VectorSearchAll(gctx, sqlc.VectorSearchAllParams{
					QueryEmbedding: pgvector_go.NewVector(vec),
					MaxResults:     fetchLimit,
					CreatedAfter:   ca,
					CreatedBefore:  cb,
					UpdatedAfter:   ua,
					UpdatedBefore:  ub,
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
		} else {
			if len(tags) > 0 {
				ca, cb, ua, ub := timeRange.ToSqlNullTimes()
				rows, err := s.queries.VectorSearchWithTags(gctx, sqlc.VectorSearchWithTagsParams{
					QueryEmbedding: pgvector_go.NewVector(vec),
					WorkspaceHash:  workspace,
					Tags:           tags,
					MaxResults:     fetchLimit,
					CreatedAfter:   ca,
					CreatedBefore:  cb,
					UpdatedAfter:   ua,
					UpdatedBefore:  ub,
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
				ca, cb, ua, ub := timeRange.ToSqlNullTimes()
				rows, err := s.queries.VectorSearch(gctx, sqlc.VectorSearchParams{
					QueryEmbedding: pgvector_go.NewVector(vec),
					WorkspaceHash:  workspace,
					MaxResults:     fetchLimit,
					CreatedAfter:   ca,
					CreatedBefore:  cb,
					UpdatedAfter:   ua,
					UpdatedBefore:  ub,
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
		}
		return nil
	})

	_ = g.Wait()

	if bm25Err != nil && vectorErr != nil {
		return nil, fmt.Errorf("all search legs failed: bm25: %v, vector: %v", bm25Err, vectorErr)
	}

	merged := RRFMerge(bm25Results, vectorResults, rrfK)

	boosted := ApplyRecencyBoost(merged, recencyWeight, recencyHalfLifeDays, time.Now())

	s.configMutex.RLock()
	entityEnabled := s.config.EntityBoostEnabled
	entityFactor := s.config.EntityBoostFactor
	prEnabled := s.config.PageRankEnabled
	prWeight := s.config.PageRankWeight
	s.configMutex.RUnlock()

	if entityEnabled && s.entityQuerier != nil && workspace != "all" {
		queryEntities := ExtractQueryEntities(query)
		if len(queryEntities) > 0 {
			chunkIDs, err := s.entityQuerier.GetChunkIDsByEntityNames(ctx, sqlc.GetChunkIDsByEntityNamesParams{
				WorkspaceHash: workspace,
				Column2:       queryEntities,
			})
			if err != nil {
				s.logger.Warn().Err(err).Msg("entity lookup failed, skipping boost")
			} else if len(chunkIDs) > 0 {
				matchedChunkIDs := make(map[string]int, len(chunkIDs))
				for _, id := range chunkIDs {
					matchedChunkIDs[id.String()]++
				}
				boosted = ApplyEntityBoost(boosted, matchedChunkIDs, entityFactor)
			}
		}
	}

	if prEnabled && s.pagerankLoader != nil && workspace != "all" {
		scores, err := s.pagerankLoader.LoadScores(ctx, workspace)
		if err != nil {
			s.logger.Warn().Err(err).Msg("pagerank scores load failed, skipping boost")
		} else if len(scores) > 0 {
			boosted = ApplyPageRankBoost(boosted, scores, prWeight)
		}
	}

	if len(boosted) > maxResults {
		boosted = boosted[:maxResults]
	}

	return boosted, nil
}
