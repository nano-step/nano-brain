package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/codesummarize"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/rs/zerolog"
)

type CodeSummarizeRequest struct {
	Workspace string `json:"workspace"`
}

type CodeSummarizeResponse struct {
	Processed int    `json:"processed"`
	Skipped   int    `json:"skipped"`
	Errors    int    `json:"errors"`
	Message   string `json:"message,omitempty"`
}

type CodeSummarizer interface {
	RunOnce(ctx context.Context, workspaceHash string) (processed, skipped, errors int, err error)
}

// TriggerCodeSummarize godoc
// @Summary      Summarize pending code symbols for a workspace
// @Description  Runs one batch of code-symbol summarization for the workspace, subject to daily budget limits
// @Tags         code-summarize
// @Accept       json
// @Produce      json
// @Param        request body CodeSummarizeRequest true "Code summarize options"
// @Success      200 {object} CodeSummarizeResponse
// @Failure      400 {object} map[string]string
// @Security     WorkspaceRegisteredAuth
// @Security     CSRFToken
// @Router       /api/v1/code/summarize [post]
func TriggerCodeSummarize(
	getSummarizer func() CodeSummarizer,
	cfg config.CodeSummarizationConfig,
	logger zerolog.Logger,
) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req CodeSummarizeRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}

		workspace, ok := c.Get("workspace").(string)
		if !ok || workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace not found in context")
		}

		if !cfg.Enabled {
			return echo.NewHTTPError(http.StatusBadRequest, "code summarization is disabled")
		}

		summarizer := getSummarizer()
		if summarizer == nil {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "code summarization not configured")
		}

		ctx := c.Request().Context()
		processed, skipped, errs, err := summarizer.RunOnce(ctx, workspace)
		if err != nil {
			if errors.Is(err, codesummarize.ErrBudgetExhausted) {
				return c.JSON(http.StatusOK, CodeSummarizeResponse{
					Processed: processed,
					Skipped:   skipped,
					Errors:    errs,
					Message:   "daily code summarization budget exhausted",
				})
			}

			reqLog := LoggerFromCtx(c, logger)
			reqLog.Error().Err(err).Str("workspace", workspace).Msg("code summarization failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "code summarization failed")
		}

		reqLog := LoggerFromCtx(c, logger)
		reqLog.Info().
			Str("workspace", workspace).
			Int("processed", processed).
			Int("skipped", skipped).
			Int("errors", errs).
			Msg("code summarization triggered")

		return c.JSON(http.StatusOK, CodeSummarizeResponse{
			Processed: processed,
			Skipped:   skipped,
			Errors:    errs,
		})
	}
}
