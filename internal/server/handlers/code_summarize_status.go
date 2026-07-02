package handlers

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type CodeSummarizationStatusQuerier interface {
	GetSummarizationStatus(ctx context.Context, workspaceHash string) (sqlc.GetSummarizationStatusRow, error)
}

type CodeSummarizeStatusResponse struct {
	TotalSymbols int `json:"total_symbols"`
	Summarized   int `json:"summarized"`
	Pending      int `json:"pending"`
	Failed       int `json:"failed"`
}

// GetCodeSummarizeStatus godoc
// @Summary      Get code summarization status for a workspace
// @Description  Reports total/summarized/pending/failed code symbol counts for the workspace
// @Tags         code-summarize
// @Produce      json
// @Success      200 {object} CodeSummarizeStatusResponse
// @Failure      400 {object} map[string]string
// @Security     WorkspaceRegisteredAuth
// @Security     CSRFToken
// @Router       /api/v1/code/summarize/status [get]
func GetCodeSummarizeStatus(
	queries CodeSummarizationStatusQuerier,
	logger zerolog.Logger,
) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspace, ok := c.Get("workspace").(string)
		if !ok || workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace not found in context")
		}

		ctx := c.Request().Context()
		status, err := queries.GetSummarizationStatus(ctx, workspace)
		if err != nil {
			reqLog := LoggerFromCtx(c, logger)
			reqLog.Error().Err(err).Str("workspace", workspace).Msg("failed to get summarization status")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get summarization status")
		}

		pending := int(status.TotalSymbols) - int(status.Summarized) - int(status.Failed)
		if pending < 0 {
			pending = 0
		}

		return c.JSON(http.StatusOK, CodeSummarizeStatusResponse{
			TotalSymbols: int(status.TotalSymbols),
			Summarized:   int(status.Summarized),
			Pending:      pending,
			Failed:       int(status.Failed),
		})
	}
}
