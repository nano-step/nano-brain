package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type BM25SearchQuerier interface {
	BM25Search(ctx context.Context, arg sqlc.BM25SearchParams) ([]sqlc.BM25SearchRow, error)
}

type BM25SearchRequest struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"`
}

const maxSnippetLen = 700

func BM25Search(q BM25SearchQuerier, logger zerolog.Logger) echo.HandlerFunc {
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

		rows, err := q.BM25Search(c.Request().Context(), sqlc.BM25SearchParams{
			Query:         req.Query,
			WorkspaceHash: workspace,
			MaxResults:    int32(maxResults),
		})
		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Msg("bm25 search failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "bm25 search failed")
		}

		results := make([]SearchResult, 0, len(rows))
		for _, r := range rows {
			content := r.Content
			if len(content) > maxSnippetLen {
				content = content[:maxSnippetLen]
			}
			results = append(results, SearchResult{
				ID:            r.ID.String(),
				Content:       content,
				Score:         float64(r.Score),
				SourcePath:    r.SourcePath,
				Collection:    r.Collection,
				Tags:          strings.Join(r.Tags, ","),
				WorkspaceHash: r.WorkspaceHash,
				DocumentID:    r.DocumentID.String(),
			})
		}

		return c.JSON(http.StatusOK, SearchResponse{
			Results: results,
			Total:   len(results),
			QueryMs: time.Since(start).Milliseconds(),
		})
	}
}
