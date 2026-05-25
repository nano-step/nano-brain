package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/telemetry"
	"github.com/rs/zerolog"
)

type BM25SearchQuerier interface {
	BM25Search(ctx context.Context, arg sqlc.BM25SearchParams) ([]sqlc.BM25SearchRow, error)
	BM25SearchWithTags(ctx context.Context, arg sqlc.BM25SearchWithTagsParams) ([]sqlc.BM25SearchWithTagsRow, error)
}

type BM25SearchRequest struct {
	Query      string   `json:"query"`
	MaxResults int      `json:"max_results,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

func BM25Search(q BM25SearchQuerier, logger zerolog.Logger, rec ...*telemetry.Recorder) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req BM25SearchRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}
		if req.Query == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "query is required")
		}

		maxResults := req.MaxResults
		if maxResults <= 0 {
			maxResults = 10
		}
		if maxResults > 100 {
			maxResults = 100
		}

		workspace, _ := c.Get("workspace").(string)
		if workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}

		start := time.Now()
		ctx := c.Request().Context()
		limit := int32(maxResults)

		type bm25Row struct {
			ID            string
			DocumentID    string
			WorkspaceHash string
			Title         string
			Content       string
			SourcePath    string
			Collection    string
			Tags          []string
			CreatedAt     time.Time
			UpdatedAt     time.Time
			Score         float64
		}

		var matched []bm25Row
		if len(req.Tags) > 0 {
			rows, err := q.BM25SearchWithTags(ctx, sqlc.BM25SearchWithTagsParams{
				Query:         req.Query,
				WorkspaceHash: workspace,
				Tags:          req.Tags,
				MaxResults:    limit,
			})
			if err != nil {
				logger.Error().Err(err).Str("workspace", workspace).Msg("bm25 search failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "bm25 search failed")
			}
			for _, r := range rows {
				matched = append(matched, bm25Row{
					ID: r.ID.String(), DocumentID: r.DocumentID.String(),
					WorkspaceHash: r.WorkspaceHash, Title: r.Title,
					Content: r.Content, SourcePath: r.SourcePath,
					Collection: r.Collection, Tags: r.Tags,
					CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
					Score: r.Score,
				})
			}
		} else {
			rows, err := q.BM25Search(ctx, sqlc.BM25SearchParams{
				Query:         req.Query,
				WorkspaceHash: workspace,
				MaxResults:    limit,
			})
			if err != nil {
				logger.Error().Err(err).Str("workspace", workspace).Msg("bm25 search failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "bm25 search failed")
			}
			for _, r := range rows {
				matched = append(matched, bm25Row{
					ID: r.ID.String(), DocumentID: r.DocumentID.String(),
					WorkspaceHash: r.WorkspaceHash, Title: r.Title,
					Content: r.Content, SourcePath: r.SourcePath,
					Collection: r.Collection, Tags: r.Tags,
					CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
					Score: r.Score,
				})
			}
		}

		results := make([]SearchResult, 0, len(matched))
		for _, r := range matched {
			results = append(results, SearchResult{
				ID:            r.ID,
				Title:         r.Title,
				Snippet:       truncateSnippet(r.Content, maxSnippetLen),
				Score:         r.Score,
				Tags:          r.Tags,
				Collection:    r.Collection,
				WorkspaceHash: r.WorkspaceHash,
				SourcePath:    r.SourcePath,
				DocumentID:    r.DocumentID,
				CreatedAt:     r.CreatedAt,
				UpdatedAt:     r.UpdatedAt,
			})
		}

		elapsed := time.Since(start).Milliseconds()

		if len(rec) > 0 && rec[0] != nil {
			rec[0].Record(c.Request().Context(), req.Query, len(results), elapsed, "", workspace)
		}

		reqLog := LoggerFromCtx(c, logger)
		reqLog.Info().
			Str("workspace", workspace).
			Int("results", len(results)).
			Int64("latency_ms", elapsed).
			Msg("bm25 search complete")

		return c.JSON(http.StatusOK, SearchResponse{
			Results: results,
			Total:   len(results),
			QueryMs: elapsed,
		})
	}
}
