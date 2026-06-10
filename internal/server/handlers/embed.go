package handlers

import (
	"context"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/embed"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	pgvector "github.com/pgvector/pgvector-go"
	"github.com/rs/zerolog"
)

const embedBatchLimit = 50

type EmbedQuerier interface {
	GetPendingChunks(ctx context.Context, arg sqlc.GetPendingChunksParams) ([]sqlc.Chunk, error)
	InsertEmbedding(ctx context.Context, arg sqlc.InsertEmbeddingParams) (sqlc.Embedding, error)
	MarkChunkEmbedded(ctx context.Context, arg sqlc.MarkChunkEmbeddedParams) error
	CountPendingChunks(ctx context.Context, workspaceHash string) (int64, error)
	ResetEmbedStatus(ctx context.Context, workspaceHash string) (int64, error)
}

type embedRequest struct {
	Force bool `json:"force"`
}

type EmbedResponse struct {
	Embedded  int   `json:"embedded"`
	Remaining int64 `json:"remaining"`
}

func TriggerEmbed(q EmbedQuerier, embedder embed.Embedder, provider, model string, maxChars int, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		if embedder == nil {
			return echo.NewHTTPError(http.StatusBadRequest, "embedding not configured")
		}

		var req embedRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}

		workspace, _ := c.Get("workspace").(string)
		if workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}
		ctx := c.Request().Context()

		if req.Force {
			if _, err := q.ResetEmbedStatus(ctx, workspace); err != nil {
				logger.Error().Err(err).Str("workspace", workspace).Msg("reset embed status failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to reset embed status")
			}
		}

		chunks, err := q.GetPendingChunks(ctx, sqlc.GetPendingChunksParams{
			WorkspaceHash: workspace,
			Limit:         embedBatchLimit + 1,
		})
		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Msg("get pending chunks failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get pending chunks")
		}

		hasMore := len(chunks) > embedBatchLimit
		if hasMore {
			chunks = chunks[:embedBatchLimit]
		}

		loopCtx, loopCancel := context.WithTimeout(ctx, 3*time.Minute)
		defer loopCancel()

		embedded := 0
		for _, chunk := range chunks {
			embedCtx, cancel := context.WithTimeout(loopCtx, 30*time.Second)
			content := truncateToMaxChars(chunk.Content, maxChars)
			if len(content) < len(chunk.Content) {
				logger.Warn().
					Str("chunk_id", chunk.ID.String()).
					Int("original_len", len(chunk.Content)).
					Int("truncated_len", len(content)).
					Msg("chunk truncated before embedding (exceeds context limit)")
			}
			vec, embedErr := embedder.Embed(embedCtx, content)
			cancel()
			if embedErr != nil {
				logger.Error().Err(embedErr).Str("chunk_id", chunk.ID.String()).Msg("embed failed")
				break
			}

			if _, insertErr := q.InsertEmbedding(loopCtx, sqlc.InsertEmbeddingParams{
				ChunkID:       chunk.ID,
				WorkspaceHash: chunk.WorkspaceHash,
				Provider:      provider,
				Model:         model,
				Embedding:     pgvector.NewVector(vec),
			}); insertErr != nil {
				logger.Error().Err(insertErr).Str("chunk_id", chunk.ID.String()).Msg("insert embedding failed")
				break
			}

			if markErr := q.MarkChunkEmbedded(loopCtx, sqlc.MarkChunkEmbeddedParams{
				ID:            chunk.ID,
				WorkspaceHash: chunk.WorkspaceHash,
			}); markErr != nil {
				logger.Error().Err(markErr).Str("chunk_id", chunk.ID.String()).Msg("mark embedded failed")
				break
			}

			embedded++
		}

		var remaining int64
		if hasMore {
			var countErr error
			remaining, countErr = q.CountPendingChunks(ctx, workspace)
			if countErr != nil {
				logger.Error().Err(countErr).Msg("failed to count remaining pending chunks")
			}
		} else {
			remaining = int64(len(chunks) - embedded)
			if remaining < 0 {
				remaining = 0
			}
		}

		reqLog := LoggerFromCtx(c, logger)
		reqLog.Info().
			Str("workspace", workspace).
			Int("embedded", embedded).
			Int64("remaining", remaining).
			Msg("embed triggered")

		return c.JSON(http.StatusOK, EmbedResponse{
			Embedded:  embedded,
			Remaining: remaining,
		})
	}
}

func truncateToMaxChars(s string, max int) string {
	if len(s) <= max {
		return s
	}
	truncated := s[:max]
	for len(truncated) > 0 && !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated
}
