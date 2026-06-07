package handlers

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type PageRankQuerier interface {
	ListCallEdges(ctx context.Context, workspaceHash string) ([]sqlc.ListCallEdgesRow, error)
	UpsertPageRankScore(ctx context.Context, arg sqlc.UpsertPageRankScoreParams) error
	DeletePageRankScores(ctx context.Context, workspaceHash string) error
}

func GraphPageRankCompute(q PageRankQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspace := c.Get("workspace").(string)
		ctx := c.Request().Context()

		rows, err := q.ListCallEdges(ctx, workspace)
		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Msg("pagerank: list edges failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list edges")
		}

		if len(rows) == 0 {
			return c.JSON(http.StatusOK, map[string]interface{}{
				"status":          "computed",
				"symbols_updated": 0,
			})
		}

		edges := make([]graph.Edge, 0, len(rows))
		for _, r := range rows {
			edges = append(edges, graph.Edge{
				SourceNode: r.SourceNode,
				TargetNode: r.TargetNode,
				Kind:       graph.EdgeCalls,
			})
		}

		scores := graph.ComputePageRank(edges, 0.85, 100, 1e-6)

		if err := q.DeletePageRankScores(ctx, workspace); err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Msg("pagerank: delete old scores failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to clear old scores")
		}

		count := 0
		for node, score := range scores {
			if err := q.UpsertPageRankScore(ctx, sqlc.UpsertPageRankScoreParams{
				WorkspaceHash: workspace,
				NodeName:      node,
				Score:         score,
			}); err != nil {
				logger.Warn().Err(err).Str("node", node).Msg("pagerank: upsert score failed")
				continue
			}
			count++
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":          "computed",
			"symbols_updated": count,
		})
	}
}
