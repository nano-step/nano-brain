package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/search"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/telemetry"
	pgvector_go "github.com/pgvector/pgvector-go"
	"github.com/rs/zerolog"
)

// Embedder generates vector embeddings from text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Dimension() int
}

// VSearchQuerier is the database interface for vector search.
type VSearchQuerier interface {
	VectorSearch(ctx context.Context, arg sqlc.VectorSearchParams) ([]sqlc.VectorSearchRow, error)
	VectorSearchWithTags(ctx context.Context, arg sqlc.VectorSearchWithTagsParams) ([]sqlc.VectorSearchWithTagsRow, error)
}

type VSearchRequest struct {
	Query      string   `json:"query"`
	MaxResults int      `json:"max_results,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

const maxSnippetLen = 700

type SearchResult struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Snippet       string    `json:"snippet"`
	Score         float64   `json:"score"`
	Tags          []string  `json:"tags,omitempty"`
	Collection    string    `json:"collection"`
	WorkspaceHash string    `json:"workspace_hash"`
	SourcePath    string    `json:"source_path"`
	DocumentID    string    `json:"document_id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func truncateSnippet(content string, maxLen int) string {
	return search.TruncateSnippet(content, maxLen)
}

type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Total   int            `json:"total"`
	QueryMs int64          `json:"query_ms"`
}

func VectorSearch(q VSearchQuerier, embedder Embedder, logger zerolog.Logger, rec ...*telemetry.Recorder) echo.HandlerFunc {
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

		var results []SearchResult
		if len(req.Tags) > 0 {
			rows, err := q.VectorSearchWithTags(c.Request().Context(), sqlc.VectorSearchWithTagsParams{
				QueryEmbedding: pgvector_go.NewVector(vec),
				WorkspaceHash:  workspace,
				Tags:           req.Tags,
				MaxResults:     int32(maxResults),
			})
			if err != nil {
				logger.Error().Err(err).Str("workspace", workspace).Msg("vector search failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "vector search failed")
			}
			results = make([]SearchResult, 0, len(rows))
			for _, r := range rows {
				results = append(results, SearchResult{
					ID:            r.ID.String(),
					Title:         r.Title,
					Snippet:       truncateSnippet(r.Content, maxSnippetLen),
					Score:         r.Score,
					Tags:          r.Tags,
					Collection:    r.Collection,
					WorkspaceHash: r.WorkspaceHash,
					SourcePath:    r.SourcePath,
					DocumentID:    r.DocumentID.String(),
					CreatedAt:     r.CreatedAt,
					UpdatedAt:     r.UpdatedAt,
				})
			}
		} else {
			rows, err := q.VectorSearch(c.Request().Context(), sqlc.VectorSearchParams{
				QueryEmbedding: pgvector_go.NewVector(vec),
				WorkspaceHash:  workspace,
				MaxResults:     int32(maxResults),
			})
			if err != nil {
				logger.Error().Err(err).Str("workspace", workspace).Msg("vector search failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "vector search failed")
			}
			results = make([]SearchResult, 0, len(rows))
			for _, r := range rows {
				results = append(results, SearchResult{
					ID:            r.ID.String(),
					Title:         r.Title,
					Snippet:       truncateSnippet(r.Content, maxSnippetLen),
					Score:         r.Score,
					Tags:          r.Tags,
					Collection:    r.Collection,
					WorkspaceHash: r.WorkspaceHash,
					SourcePath:    r.SourcePath,
					DocumentID:    r.DocumentID.String(),
					CreatedAt:     r.CreatedAt,
					UpdatedAt:     r.UpdatedAt,
				})
			}
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
			Msg("vector search complete")

		return c.JSON(http.StatusOK, SearchResponse{
			Results: results,
			Total:   len(results),
			QueryMs: elapsed,
		})
	}
}
