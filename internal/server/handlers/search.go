package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
	pgvector_go "github.com/pgvector/pgvector-go"
)

// Embedder generates vector embeddings from text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Dimension() int
}

// VSearchQuerier is the database interface for vector search.
type VSearchQuerier interface {
	VectorSearch(ctx context.Context, arg sqlc.VectorSearchParams) ([]sqlc.VectorSearchRow, error)
}

type VSearchRequest struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"`
}

type SearchResult struct {
	ID            string  `json:"id"`
	Content       string  `json:"content"`
	Score         float64 `json:"score"`
	SourcePath    string  `json:"source_path"`
	Collection    string  `json:"collection"`
	Tags          string  `json:"tags,omitempty"`
	WorkspaceHash string  `json:"workspace_hash"`
	DocumentID    string  `json:"document_id"`
}

type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Total   int            `json:"total"`
	QueryMs int64          `json:"query_ms"`
}

func VectorSearch(q VSearchQuerier, embedder Embedder, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		if embedder == nil {
			return echo.NewHTTPError(http.StatusBadRequest, "vector search requires embedding provider")
		}

		var req VSearchRequest
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

		embedCtx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
		defer cancel()

		vec, err := embedder.Embed(embedCtx, req.Query)
		if err != nil {
			logger.Error().Err(err).Msg("embedding query failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to embed query")
		}

		rows, err := q.VectorSearch(c.Request().Context(), sqlc.VectorSearchParams{
			QueryEmbedding: pgvector_go.NewVector(vec),
			WorkspaceHash:  workspace,
			MaxResults:     int32(maxResults),
		})
		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Msg("vector search failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "vector search failed")
		}

		results := make([]SearchResult, 0, len(rows))
		for _, r := range rows {
			tags := ""
			if len(r.Tags) > 0 {
				tags = joinTags(r.Tags)
			}
			results = append(results, SearchResult{
				ID:            r.ID.String(),
				Content:       r.Content,
				Score:         r.Score,
				SourcePath:    r.SourcePath,
				Collection:    r.Collection,
				Tags:          tags,
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

func joinTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	result := tags[0]
	for _, t := range tags[1:] {
		result += "," + t
	}
	return result
}
