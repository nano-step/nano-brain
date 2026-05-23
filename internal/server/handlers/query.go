package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/search"
	"github.com/rs/zerolog"
)

type HybridSearcher interface {
	HybridSearch(ctx context.Context, query string, workspace string, maxResults int) ([]search.Result, error)
}

type QueryRequest struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"`
}

func Query(searcher HybridSearcher, defaultLimit int, logger zerolog.Logger) echo.HandlerFunc {
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
			maxResults = defaultLimit
		}
		if maxResults > 100 {
			maxResults = 100
		}

		workspace, _ := c.Get("workspace").(string)
		if workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}

		start := time.Now()

		results, err := searcher.HybridSearch(c.Request().Context(), req.Query, workspace, maxResults)
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

		return c.JSON(http.StatusOK, SearchResponse{
			Results: out,
			Total:   len(out),
			QueryMs: time.Since(start).Milliseconds(),
		})
	}
}
