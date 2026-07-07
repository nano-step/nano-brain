package search

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/embed"
	"github.com/nano-brain/nano-brain/internal/search/hyde"
	"github.com/nano-brain/nano-brain/internal/search/preprocess"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	pgvector_go "github.com/pgvector/pgvector-go"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

var bm25StopWords = map[string]bool{
	"the": true, "a": true, "an": true, "is": true, "are": true,
	"was": true, "were": true, "what": true, "how": true, "why": true,
	"when": true, "where": true, "explain": true, "show": true, "find": true,
	"trace": true, "debug": true,
}

func buildORQuery(query string) string {
	words := strings.Fields(query)
	var filtered []string
	for _, w := range words {
		lower := strings.ToLower(w)
		if bm25StopWords[lower] {
			continue
		}
		cleaned := strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
				return r
			}
			return -1
		}, lower)
		if cleaned != "" {
			filtered = append(filtered, cleaned)
		}
	}
	if len(filtered) == 0 {
		return ""
	}
	return strings.Join(filtered, " | ")
}

type Querier interface {
	BM25Search(ctx context.Context, arg sqlc.BM25SearchParams) ([]sqlc.BM25SearchRow, error)
	BM25SearchAll(ctx context.Context, arg sqlc.BM25SearchAllParams) ([]sqlc.BM25SearchAllRow, error)
	BM25SearchWithTags(ctx context.Context, arg sqlc.BM25SearchWithTagsParams) ([]sqlc.BM25SearchWithTagsRow, error)
	BM25SearchAllWithTags(ctx context.Context, arg sqlc.BM25SearchAllWithTagsParams) ([]sqlc.BM25SearchAllWithTagsRow, error)
	BM25SearchOR(ctx context.Context, arg sqlc.BM25SearchORParams) ([]sqlc.BM25SearchORRow, error)
	BM25SearchAllOR(ctx context.Context, arg sqlc.BM25SearchAllORParams) ([]sqlc.BM25SearchAllORRow, error)
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
	preprocessor   *preprocess.Preprocessor
	hydeGenerator  *hyde.Generator
	reranker       Reranker
	// queryEmbedCache avoids a redundant embedding-provider round-trip when
	// the same query text is embedded more than once (retries, cursor
	// pagination, mode=debugging fan-out). It caches query embeddings only —
	// document/chunk embeddings go through embed.Queue directly and never
	// touch this cache. See issue #539 Finding 4.
	queryEmbedCache *embed.QueryCache
}

func NewSearchService(queries Querier, embedder Embedder, cfg config.SearchConfig, logger zerolog.Logger) *SearchService {
	return &SearchService{
		queries:         queries,
		embedder:        embedder,
		config:          cfg,
		logger:          logger,
		queryEmbedCache: embed.NewQueryCache(embed.DefaultQueryCacheSize),
	}
}

// embedQueryCached returns the embedding for text, serving from
// queryEmbedCache when available and falling back to the underlying
// Embedder on a miss.
func (s *SearchService) embedQueryCached(ctx context.Context, text string) ([]float32, error) {
	if vec, ok := s.queryEmbedCache.Get(text); ok {
		return vec, nil
	}
	vec, err := s.embedder.Embed(ctx, text)
	if err != nil {
		return nil, err
	}
	// Don't cache a degenerate/empty embedding — a later real result for the
	// same query would otherwise be shadowed by the empty one until eviction.
	if len(vec) == 0 {
		return vec, nil
	}
	s.queryEmbedCache.Put(text, vec)
	return vec, nil
}

func (s *SearchService) SetEntityQuerier(eq EntityQuerier) {
	s.entityQuerier = eq
}

func (s *SearchService) SetPageRankLoader(loader PageRankLoader) {
	s.pagerankLoader = loader
}

func (s *SearchService) SetPreprocessor(pp *preprocess.Preprocessor) {
	s.preprocessor = pp
}

func (s *SearchService) SetHydeGenerator(hg *hyde.Generator) {
	s.hydeGenerator = hg
}

func (s *SearchService) SetReranker(rr Reranker) {
	s.reranker = rr
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

	// Apply query preprocessing if available
	if s.preprocessor != nil {
		result := s.preprocessor.Process(ctx, query)
		if result != nil && result.EnglishQuery != "" {
			query = result.EnglishQuery
		}
		// Apply time filter if extracted
		if result != nil && result.TimeFilter != nil {
			if timeRange == nil {
				timeRange = &TimeRangeFilter{}
			}
			if result.TimeFilter.After != nil && timeRange.UpdatedAfter == nil {
				timeRange.UpdatedAfter = result.TimeFilter.After
			}
			if result.TimeFilter.Before != nil && timeRange.UpdatedBefore == nil {
				timeRange.UpdatedBefore = result.TimeFilter.Before
			}
		}
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
		embedQuery := query
		if s.hydeGenerator != nil {
			s.configMutex.RLock()
			hydeEnabled := s.config.HyDE.Enabled
			s.configMutex.RUnlock()
			if hydeEnabled {
				hypothetical, err := s.hydeGenerator.Generate(gctx, query, workspace)
				if err == nil && hypothetical != "" {
					s.logger.Debug().Str("original", query).Str("hyde", hypothetical).Msg("hyde: generated hypothetical document")
					embedQuery = hypothetical
				}
			}
		}
		vec, err := s.embedQueryCached(gctx, embedQuery)
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

	orFallback := false
	if len(bm25Results) == 0 && bm25Err == nil && len(strings.Fields(query)) > 2 {
		orQuery := buildORQuery(query)
		if orQuery != "" {
			s.logger.Debug().Str("original", query).Str("or_query", orQuery).Msg("bm25 returned 0 results, retrying with OR")
			orFallback = true
			if workspace == "all" {
				orRows, err := s.queries.BM25SearchAllOR(ctx, sqlc.BM25SearchAllORParams{
					Query:      orQuery,
					ChunkType:  chunkTypeNullStr,
					MaxResults: fetchLimit,
				})
				if err == nil {
					bm25Results = make([]Result, 0, len(orRows))
					for _, r := range orRows {
						bm25Results = append(bm25Results, Result{
							ID: r.ID.String(), DocumentID: r.DocumentID.String(),
							WorkspaceHash: r.WorkspaceHash, Title: r.Title, Content: r.Content,
							Score: r.Score, Tags: r.Tags, Collection: r.Collection,
							SourcePath: r.SourcePath, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
						})
					}
				} else {
					s.logger.Warn().Err(err).Msg("bm25 OR fallback failed")
				}
			} else {
				orRows, err := s.queries.BM25SearchOR(ctx, sqlc.BM25SearchORParams{
					Query:         orQuery,
					WorkspaceHash: workspace,
					ChunkType:     chunkTypeNullStr,
					MaxResults:    fetchLimit,
				})
				if err == nil {
					bm25Results = make([]Result, 0, len(orRows))
					for _, r := range orRows {
						bm25Results = append(bm25Results, Result{
							ID: r.ID.String(), DocumentID: r.DocumentID.String(),
							WorkspaceHash: r.WorkspaceHash, Title: r.Title, Content: r.Content,
							Score: r.Score, Tags: r.Tags, Collection: r.Collection,
							SourcePath: r.SourcePath, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
						})
					}
				} else {
					s.logger.Warn().Err(err).Msg("bm25 OR fallback failed")
				}
			}
			_ = orFallback
		}
	}

	merged := DynamicRRFMerge(bm25Results, vectorResults, rrfK)
	deduped := DeduplicateResults(merged)
	codeAware := ApplyCodeAwareBoost(deduped, query, 1.2, 1.3)
	extBoosted := ApplyExtensionBoost(codeAware, 1.1, 0.9)

	boosted := ApplyRecencyBoost(extBoosted, recencyWeight, recencyHalfLifeDays, time.Now())

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

	// Apply reranking if enabled
	reranked := boosted
	if s.reranker != nil {
		s.configMutex.RLock()
		rerankEnabled := s.config.Reranking.Enabled
		rerankTopK := s.config.Reranking.TopK
		s.configMutex.RUnlock()
		if rerankEnabled && rerankTopK > 0 {
			var err error
			reranked, err = s.reranker.Rerank(ctx, query, boosted, rerankTopK)
			if err != nil {
				s.logger.Warn().Err(err).Msg("reranking failed, using boosted results")
				reranked = boosted
			}
		}
	}

	if len(reranked) > maxResults {
		reranked = reranked[:maxResults]
	}

	for i := range reranked {
		reranked[i].Snippet = ExtractRelevantSnippet(reranked[i].Content, query, MaxSnippetLen)
	}

	return reranked, nil
}

// DebugSearchMode is the mode parameter for debugging-aware search.
const DebugSearchMode = "debugging"

// tagSource returns results with Source set to source, in place. Applied to
// each DebugSearch leg before RRF merge so the label survives merge/dedup —
// both operate on Result values and copy the whole struct, so Source rides
// along without further plumbing (#543).
func tagSource(results []Result, source string) []Result {
	for i := range results {
		results[i].Source = source
	}
	return results
}

// DebugSearch runs 3 parallel searches (code, session, config), tags each
// leg's results with its Source ("code"/"session"/"config") before merging
// them with RRF fusion, and returns a flat, deduplicated list where each
// result carries the source label of the leg it came from.
//
// Tie rule: when a chunk is returned by more than one leg, RRFMerge keeps
// that chunk's metadata (including Source) from its first occurrence across
// the merge order below (code, then session, then config) — the same
// "first occurrence wins" convention RRFMerge already uses for all other
// fields (see RRFMerge doc comment). So a doc matching multiple legs is
// deterministically labeled with the earliest leg, in code > session > config
// priority order.
//
// Each sub-search has a 2s timeout. Partial failures are handled gracefully.
func (s *SearchService) DebugSearch(ctx context.Context, query string, workspace string, maxResults int, timeRange *TimeRangeFilter, chunkType string) ([]Result, error) {
	s.configMutex.RLock()
	rrfK := s.config.RrfK
	recencyWeight := s.config.RecencyWeight
	recencyHalfLifeDays := s.config.RecencyHalfLifeDays
	s.configMutex.RUnlock()

	type subResult struct {
		results []Result
	}

	g, gctx := errgroup.WithContext(ctx)

	var (
		codeResults   subResult
		sessionResults subResult
		configResults subResult
	)

	// Sub-search 1: code results (original query)
	g.Go(func() error {
		codeCtx, cancel := context.WithTimeout(gctx, 2*time.Second)
		defer cancel()
		results, err := s.hybridSearchInner(codeCtx, query, workspace, maxResults, nil, timeRange, chunkType)
		if err != nil {
			s.logger.Warn().Err(err).Msg("debug: code search leg failed")
			return nil
		}
		codeResults = subResult{results: tagSource(results, "code")}
		return nil
	})

	// Sub-search 2: session results (query + debug terms)
	g.Go(func() error {
		sessionCtx, cancel := context.WithTimeout(gctx, 2*time.Second)
		defer cancel()
		sessionQuery := query + " debug session error"
		results, err := s.hybridSearchInner(sessionCtx, sessionQuery, workspace, maxResults, nil, timeRange, chunkType)
		if err != nil {
			s.logger.Warn().Err(err).Msg("debug: session search leg failed")
			return nil
		}
		sessionResults = subResult{results: tagSource(results, "session")}
		return nil
	})

	// Sub-search 3: config results (query, filtered to config/memory collections)
	g.Go(func() error {
		configCtx, cancel := context.WithTimeout(gctx, 2*time.Second)
		defer cancel()
		configTags := []string{"config", "memory"}
		results, err := s.hybridSearchInner(configCtx, query, workspace, maxResults, configTags, timeRange, chunkType)
		if err != nil {
			s.logger.Warn().Err(err).Msg("debug: config search leg failed")
			return nil
		}
		configResults = subResult{results: tagSource(results, "config")}
		return nil
	})

	_ = g.Wait()

	var allResultSets [][]Result
	if len(codeResults.results) > 0 {
		allResultSets = append(allResultSets, codeResults.results)
	}
	if len(sessionResults.results) > 0 {
		allResultSets = append(allResultSets, sessionResults.results)
	}
	if len(configResults.results) > 0 {
		allResultSets = append(allResultSets, configResults.results)
	}

	if len(allResultSets) == 0 {
		return nil, nil
	}

	merged := allResultSets[0]
	for i := 1; i < len(allResultSets); i++ {
		merged = DynamicRRFMerge(merged, allResultSets[i], rrfK)
	}

	merged = DeduplicateResults(merged)
	codeAware := ApplyCodeAwareBoost(merged, query, 1.2, 1.3)
	extBoosted := ApplyExtensionBoost(codeAware, 1.1, 0.9)
	boosted := ApplyRecencyBoost(extBoosted, recencyWeight, recencyHalfLifeDays, time.Now())

	if len(boosted) > maxResults {
		boosted = boosted[:maxResults]
	}

	for i := range boosted {
		boosted[i].Snippet = ExtractRelevantSnippet(boosted[i].Content, query, MaxSnippetLen)
	}

	return boosted, nil
}

// hybridSearchInner is the core BM25+Vector search without full pipeline features
// (entity boost, pagerank, reranking). Used by DebugSearch for sub-searches.
func (s *SearchService) hybridSearchInner(ctx context.Context, query string, workspace string, maxResults int, tags []string, timeRange *TimeRangeFilter, chunkType string) ([]Result, error) {
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
		ca, cb, ua, ub := timeRange.ToSqlNullTimes()
		if workspace == "all" {
			if len(tags) > 0 {
				rows, err := s.queries.BM25SearchAllWithTags(gctx, sqlc.BM25SearchAllWithTagsParams{
					Query: query, Tags: tags, ChunkType: chunkTypeNullStr,
					MaxResults: fetchLimit, CreatedAfter: ca, CreatedBefore: cb,
					UpdatedAfter: ua, UpdatedBefore: ub,
				})
				if err != nil {
					bm25Err = err
					s.logger.Warn().Err(err).Msg("debug bm25 leg failed, degrading")
					return nil
				}
				bm25Results = make([]Result, 0, len(rows))
				for _, r := range rows {
					bm25Results = append(bm25Results, Result{
						ID: r.ID.String(), DocumentID: r.DocumentID.String(),
						WorkspaceHash: r.WorkspaceHash, Title: r.Title, Content: r.Content,
						Score: r.Score, Tags: r.Tags, Collection: r.Collection,
						SourcePath: r.SourcePath, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
					})
				}
			} else {
				rows, err := s.queries.BM25SearchAll(gctx, sqlc.BM25SearchAllParams{
					Query: query, ChunkType: chunkTypeNullStr,
					MaxResults: fetchLimit, CreatedAfter: ca, CreatedBefore: cb,
					UpdatedAfter: ua, UpdatedBefore: ub,
				})
				if err != nil {
					bm25Err = err
					s.logger.Warn().Err(err).Msg("debug bm25 leg failed, degrading")
					return nil
				}
				bm25Results = make([]Result, 0, len(rows))
				for _, r := range rows {
					bm25Results = append(bm25Results, Result{
						ID: r.ID.String(), DocumentID: r.DocumentID.String(),
						WorkspaceHash: r.WorkspaceHash, Title: r.Title, Content: r.Content,
						Score: r.Score, Tags: r.Tags, Collection: r.Collection,
						SourcePath: r.SourcePath, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
					})
				}
			}
		} else {
			if len(tags) > 0 {
				rows, err := s.queries.BM25SearchWithTags(gctx, sqlc.BM25SearchWithTagsParams{
					Query: query, WorkspaceHash: workspace, Tags: tags, ChunkType: chunkTypeNullStr,
					MaxResults: fetchLimit, CreatedAfter: ca, CreatedBefore: cb,
					UpdatedAfter: ua, UpdatedBefore: ub,
				})
				if err != nil {
					bm25Err = err
					s.logger.Warn().Err(err).Msg("debug bm25 leg failed, degrading")
					return nil
				}
				bm25Results = make([]Result, 0, len(rows))
				for _, r := range rows {
					bm25Results = append(bm25Results, Result{
						ID: r.ID.String(), DocumentID: r.DocumentID.String(),
						WorkspaceHash: r.WorkspaceHash, Title: r.Title, Content: r.Content,
						Score: r.Score, Tags: r.Tags, Collection: r.Collection,
						SourcePath: r.SourcePath, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
					})
				}
			} else {
				rows, err := s.queries.BM25Search(gctx, sqlc.BM25SearchParams{
					Query: query, WorkspaceHash: workspace, ChunkType: chunkTypeNullStr,
					MaxResults: fetchLimit, CreatedAfter: ca, CreatedBefore: cb,
					UpdatedAfter: ua, UpdatedBefore: ub,
				})
				if err != nil {
					bm25Err = err
					s.logger.Warn().Err(err).Msg("debug bm25 leg failed, degrading")
					return nil
				}
				bm25Results = make([]Result, 0, len(rows))
				for _, r := range rows {
					bm25Results = append(bm25Results, Result{
						ID: r.ID.String(), DocumentID: r.DocumentID.String(),
						WorkspaceHash: r.WorkspaceHash, Title: r.Title, Content: r.Content,
						Score: r.Score, Tags: r.Tags, Collection: r.Collection,
						SourcePath: r.SourcePath, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
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
		vec, err := s.embedQueryCached(gctx, query)
		if err != nil {
			vectorErr = err
			s.logger.Warn().Err(err).Msg("debug embed failed, degrading")
			return nil
		}
		ca, cb, ua, ub := timeRange.ToSqlNullTimes()
		if workspace == "all" {
			if len(tags) > 0 {
				rows, err := s.queries.VectorSearchAllWithTags(gctx, sqlc.VectorSearchAllWithTagsParams{
					QueryEmbedding: pgvector_go.NewVector(vec), Tags: tags,
					MaxResults: fetchLimit, CreatedAfter: ca, CreatedBefore: cb,
					UpdatedAfter: ua, UpdatedBefore: ub,
				})
				if err != nil {
					vectorErr = err
					s.logger.Warn().Err(err).Msg("debug vector leg failed, degrading")
					return nil
				}
				vectorResults = make([]Result, 0, len(rows))
				for _, r := range rows {
					vectorResults = append(vectorResults, Result{
						ID: r.ChunkID.String(), DocumentID: r.DocumentID.String(),
						WorkspaceHash: r.WorkspaceHash, Title: r.Title, Content: r.Content,
						Score: r.Score, Tags: r.Tags, Collection: r.Collection,
						SourcePath: r.SourcePath, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
					})
				}
			} else {
				rows, err := s.queries.VectorSearchAll(gctx, sqlc.VectorSearchAllParams{
					QueryEmbedding: pgvector_go.NewVector(vec),
					MaxResults: fetchLimit, CreatedAfter: ca, CreatedBefore: cb,
					UpdatedAfter: ua, UpdatedBefore: ub,
				})
				if err != nil {
					vectorErr = err
					s.logger.Warn().Err(err).Msg("debug vector leg failed, degrading")
					return nil
				}
				vectorResults = make([]Result, 0, len(rows))
				for _, r := range rows {
					vectorResults = append(vectorResults, Result{
						ID: r.ChunkID.String(), DocumentID: r.DocumentID.String(),
						WorkspaceHash: r.WorkspaceHash, Title: r.Title, Content: r.Content,
						Score: r.Score, Tags: r.Tags, Collection: r.Collection,
						SourcePath: r.SourcePath, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
					})
				}
			}
		} else {
			if len(tags) > 0 {
				rows, err := s.queries.VectorSearchWithTags(gctx, sqlc.VectorSearchWithTagsParams{
					QueryEmbedding: pgvector_go.NewVector(vec), WorkspaceHash: workspace, Tags: tags,
					MaxResults: fetchLimit, CreatedAfter: ca, CreatedBefore: cb,
					UpdatedAfter: ua, UpdatedBefore: ub,
				})
				if err != nil {
					vectorErr = err
					s.logger.Warn().Err(err).Msg("debug vector leg failed, degrading")
					return nil
				}
				vectorResults = make([]Result, 0, len(rows))
				for _, r := range rows {
					vectorResults = append(vectorResults, Result{
						ID: r.ChunkID.String(), DocumentID: r.DocumentID.String(),
						WorkspaceHash: r.WorkspaceHash, Title: r.Title, Content: r.Content,
						Score: r.Score, Tags: r.Tags, Collection: r.Collection,
						SourcePath: r.SourcePath, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
					})
				}
			} else {
				rows, err := s.queries.VectorSearch(gctx, sqlc.VectorSearchParams{
					QueryEmbedding: pgvector_go.NewVector(vec), WorkspaceHash: workspace,
					MaxResults: fetchLimit, CreatedAfter: ca, CreatedBefore: cb,
					UpdatedAfter: ua, UpdatedBefore: ub,
				})
				if err != nil {
					vectorErr = err
					s.logger.Warn().Err(err).Msg("debug vector leg failed, degrading")
					return nil
				}
				vectorResults = make([]Result, 0, len(rows))
				for _, r := range rows {
					vectorResults = append(vectorResults, Result{
						ID: r.ChunkID.String(), DocumentID: r.DocumentID.String(),
						WorkspaceHash: r.WorkspaceHash, Title: r.Title, Content: r.Content,
						Score: r.Score, Tags: r.Tags, Collection: r.Collection,
						SourcePath: r.SourcePath, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
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

	orFallback := false
	if len(bm25Results) == 0 && bm25Err == nil && len(strings.Fields(query)) > 2 {
		orQuery := buildORQuery(query)
		if orQuery != "" {
			s.logger.Debug().Str("original", query).Str("or_query", orQuery).Msg("bm25 returned 0 results, retrying with OR")
			orFallback = true
			if workspace == "all" {
				orRows, err := s.queries.BM25SearchAllOR(ctx, sqlc.BM25SearchAllORParams{
					Query:      orQuery,
					ChunkType:  chunkTypeNullStr,
					MaxResults: fetchLimit,
				})
				if err == nil {
					bm25Results = make([]Result, 0, len(orRows))
					for _, r := range orRows {
						bm25Results = append(bm25Results, Result{
							ID: r.ID.String(), DocumentID: r.DocumentID.String(),
							WorkspaceHash: r.WorkspaceHash, Title: r.Title, Content: r.Content,
							Score: r.Score, Tags: r.Tags, Collection: r.Collection,
							SourcePath: r.SourcePath, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
						})
					}
				} else {
					s.logger.Warn().Err(err).Msg("bm25 OR fallback failed")
				}
			} else {
				orRows, err := s.queries.BM25SearchOR(ctx, sqlc.BM25SearchORParams{
					Query:         orQuery,
					WorkspaceHash: workspace,
					ChunkType:     chunkTypeNullStr,
					MaxResults:    fetchLimit,
				})
				if err == nil {
					bm25Results = make([]Result, 0, len(orRows))
					for _, r := range orRows {
						bm25Results = append(bm25Results, Result{
							ID: r.ID.String(), DocumentID: r.DocumentID.String(),
							WorkspaceHash: r.WorkspaceHash, Title: r.Title, Content: r.Content,
							Score: r.Score, Tags: r.Tags, Collection: r.Collection,
							SourcePath: r.SourcePath, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
						})
					}
				} else {
					s.logger.Warn().Err(err).Msg("bm25 OR fallback failed")
				}
			}
			_ = orFallback
		}
	}

	merged := DynamicRRFMerge(bm25Results, vectorResults, rrfK)
	deduped := DeduplicateResults(merged)
	codeAware := ApplyCodeAwareBoost(deduped, query, 1.2, 1.3)
	extBoosted := ApplyExtensionBoost(codeAware, 1.1, 0.9)
	boosted := ApplyRecencyBoost(extBoosted, recencyWeight, recencyHalfLifeDays, time.Now())

	if len(boosted) > maxResults {
		boosted = boosted[:maxResults]
	}

	return boosted, nil
}
