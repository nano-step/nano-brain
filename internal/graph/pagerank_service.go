package graph

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type PageRankStore interface {
	ListCallEdges(ctx context.Context, workspaceHash string) ([]sqlc.ListCallEdgesRow, error)
	UpsertPageRankScore(ctx context.Context, arg sqlc.UpsertPageRankScoreParams) error
	DeletePageRankScores(ctx context.Context, workspaceHash string) error
}

type PageRankService struct {
	store     PageRankStore
	logger    zerolog.Logger
	getCfg    func() config.SearchConfig
	edgeCounts sync.Map
	computing  sync.Map
}

func NewPageRankService(store PageRankStore, getCfg func() config.SearchConfig, logger zerolog.Logger) *PageRankService {
	return &PageRankService{
		store:  store,
		logger: logger,
		getCfg: getCfg,
	}
}

func (s *PageRankService) IncrementEdgeCount(workspace string) {
	val, _ := s.edgeCounts.LoadOrStore(workspace, new(atomic.Int64))
	val.(*atomic.Int64).Add(1)

	cfg := s.getCfg()
	if !cfg.PageRankEnabled || cfg.PageRankEdgeThreshold <= 0 {
		return
	}

	count := val.(*atomic.Int64).Load()
	if count >= int64(cfg.PageRankEdgeThreshold) {
		val.(*atomic.Int64).Store(0)
		go s.Compute(context.Background(), workspace)
	}
}

func (s *PageRankService) Compute(ctx context.Context, workspace string) {
	if _, loaded := s.computing.LoadOrStore(workspace, struct{}{}); loaded {
		return
	}
	defer s.computing.Delete(workspace)

	rows, err := s.store.ListCallEdges(ctx, workspace)
	if err != nil {
		s.logger.Error().Err(err).Str("workspace", workspace).Msg("pagerank: list edges failed")
		return
	}

	if len(rows) == 0 {
		return
	}

	edges := make([]Edge, 0, len(rows))
	for _, r := range rows {
		edges = append(edges, Edge{
			SourceNode: r.SourceNode,
			TargetNode: r.TargetNode,
			Kind:       EdgeCalls,
		})
	}

	scores := ComputePageRank(edges, 0.85, 100, 1e-6)

	if err := s.store.DeletePageRankScores(ctx, workspace); err != nil {
		s.logger.Error().Err(err).Str("workspace", workspace).Msg("pagerank: delete old scores failed")
		return
	}

	count := 0
	for node, score := range scores {
		if err := s.store.UpsertPageRankScore(ctx, sqlc.UpsertPageRankScoreParams{
			WorkspaceHash: workspace,
			NodeName:      node,
			Score:         score,
		}); err != nil {
			s.logger.Warn().Err(err).Str("node", node).Msg("pagerank: upsert score failed")
			continue
		}
		count++
	}

	s.logger.Info().Str("workspace", workspace).Int("symbols_updated", count).Msg("pagerank: computation complete")
}

func (s *PageRankService) StartDailyTicker(ctx context.Context, workspaces func() []string) {
	cfg := s.getCfg()
	if !cfg.PageRankEnabled {
		return
	}

	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !s.getCfg().PageRankEnabled {
					continue
				}
				for _, ws := range workspaces() {
					s.Compute(ctx, ws)
				}
			}
		}
	}()
}
