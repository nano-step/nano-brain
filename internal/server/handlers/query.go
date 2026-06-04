package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/search"
	"github.com/nano-brain/nano-brain/internal/telemetry"
	"github.com/rs/zerolog"
)

type HybridSearcher interface {
	HybridSearch(ctx context.Context, query string, workspace string, maxResults int, tags []string, timeRange *search.TimeRangeFilter, chunkType string) ([]search.Result, error)
	DefaultLimit() int
}

type QueryRequest struct {
	Query           string   `json:"query"`
	MaxResults      int      `json:"max_results,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	ChunkType       string   `json:"chunk_type,omitempty"`
	CreatedAfter    string   `json:"created_after,omitempty"`
	CreatedBefore   string   `json:"created_before,omitempty"`
	UpdatedAfter    string   `json:"updated_after,omitempty"`
	UpdatedBefore   string   `json:"updated_before,omitempty"`
}

func Query(searcher HybridSearcher, logger zerolog.Logger, rec ...*telemetry.Recorder) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req QueryRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}
		if req.Query == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "query is required")
		}

		maxResults := req.MaxResults
		if maxResults <= 0 {
			maxResults = searcher.DefaultLimit()
		}
		if maxResults > 100 {
			maxResults = 100
		}

		workspace, _ := c.Get("workspace").(string)
		if workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}

		start := time.Now()

		timeRange, paramName, rawValue, timeParseErr := search.ParseTimeRangeFilter(
			time.Now().UTC(),
			req.CreatedAfter,
			req.CreatedBefore,
			req.UpdatedAfter,
			req.UpdatedBefore,
		)
		if timeParseErr != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("invalid %s: %v", paramName, timeParseErr),
				"param": paramName,
				"value": rawValue,
			})
		}

		results, err := searcher.HybridSearch(c.Request().Context(), req.Query, workspace, maxResults, req.Tags, timeRange, req.ChunkType)
		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Msg("hybrid search failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "search failed")
		}

		out := make([]SearchResult, 0, len(results))
		for _, r := range results {
			out = append(out, SearchResult{
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
			rec[0].Record(c.Request().Context(), req.Query, len(out), elapsed, "", workspace)
		}

		reqLog := LoggerFromCtx(c, logger)
		reqLog.Info().
			Str("workspace", workspace).
			Int("results", len(out)).
			Int64("latency_ms", elapsed).
			Msg("hybrid search complete")

		return c.JSON(http.StatusOK, SearchResponse{
			Results: out,
			Total:   len(out),
			QueryMs: elapsed,
		})
	}
}
