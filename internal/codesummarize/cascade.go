package codesummarize

import (
	"context"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

const (
	maxCascadeInvalidations = 20
	maxQueueSize            = 1000
)

type CascadeInvalidator struct {
	queries GraphContextQuerier
	nullify interface {
		NullifyGraphContextHashBySymbols(ctx context.Context, arg sqlc.NullifyGraphContextHashBySymbolsParams) error
		GetSymbolChunksByGraphContextStale(ctx context.Context, arg sqlc.GetSymbolChunksByGraphContextStaleParams) ([]sqlc.GetSymbolChunksByGraphContextStaleRow, error)
	}
	logger zerolog.Logger
}

func NewCascadeInvalidator(queries ServiceQuerier, logger zerolog.Logger) *CascadeInvalidator {
	return &CascadeInvalidator{
		queries: queries,
		nullify: queries,
		logger:  logger.With().Str("component", "cascade-invalidator").Logger(),
	}
}

func (ci *CascadeInvalidator) InvalidateCallers(ctx context.Context, workspaceHash string, changedSymbolNodes []string) {
	if len(changedSymbolNodes) == 0 {
		return
	}

	callerMap, err := FetchGraphContext(ctx, ci.queries, workspaceHash, changedSymbolNodes)
	if err != nil {
		ci.logger.Warn().Err(err).Msg("failed to fetch callers for cascade invalidation")
		return
	}

	callerSet := make(map[string]struct{})
	for _, gc := range callerMap {
		if gc == nil {
			continue
		}
		for _, caller := range gc.Callers {
			callerSet[caller.Name] = struct{}{}
		}
	}

	if len(callerSet) == 0 {
		return
	}

	staleCount, err := ci.nullify.GetSymbolChunksByGraphContextStale(ctx, sqlc.GetSymbolChunksByGraphContextStaleParams{
		WorkspaceHash: workspaceHash,
		Limit:         int32(maxQueueSize),
	})
	if err != nil {
		ci.logger.Warn().Err(err).Msg("failed to check current stale queue size")
		return
	}

	currentQueueSize := len(staleCount)
	available := maxQueueSize - currentQueueSize
	if available <= 0 {
		ci.logger.Warn().
			Int("queue_size", currentQueueSize).
			Int("would_add", len(callerSet)).
			Msg("queue overflow, dropping cascade invalidations")
		return
	}

	callers := make([]string, 0, len(callerSet))
	for c := range callerSet {
		callers = append(callers, c)
	}

	if len(callers) > maxCascadeInvalidations {
		ci.logger.Warn().
			Int("total_callers", len(callers)).
			Int("cap", maxCascadeInvalidations).
			Msg("capping cascade invalidations")
		callers = callers[:maxCascadeInvalidations]
	}

	if len(callers) > available {
		ci.logger.Warn().
			Int("callers", len(callers)).
			Int("available", available).
			Msg("trimming callers to fit queue cap")
		callers = callers[:available]
	}

	if err := ci.nullify.NullifyGraphContextHashBySymbols(ctx, sqlc.NullifyGraphContextHashBySymbolsParams{
		WorkspaceHash: workspaceHash,
		Column2:       callers,
	}); err != nil {
		ci.logger.Warn().Err(err).Int("count", len(callers)).Msg("failed to nullify graph context hash for callers")
		return
	}

	ci.logger.Info().
		Int("invalidated", len(callers)).
		Msg("cascade invalidated caller summaries")
}
