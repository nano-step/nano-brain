package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/codesummarize"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type CodeSummarizeRetryQuerier interface {
	GetUnresolvedFailures(ctx context.Context, workspaceHash string) ([]sqlc.GetUnresolvedFailuresRow, error)
	ResolveFailure(ctx context.Context, id uuid.UUID) error
	UpdateCodeSummarizationFailure(ctx context.Context, arg sqlc.UpdateCodeSummarizationFailureParams) error
}

type RetryRequest struct {
	Workspace  string   `json:"workspace"`
	FailureIDs []string `json:"failure_ids,omitempty"`
}

type RetryResponse struct {
	Retried   int    `json:"retried"`
	Succeeded int    `json:"succeeded"`
	Failed    int    `json:"failed"`
	Message   string `json:"message,omitempty"`
}

// RetryCodeSummarize retries specific failed code summarizations.
//
// @Summary      Retry specific failed code summarizations
// @Description  Retries code summarization for the given failure_ids
// @Tags         code-summarize
// @Accept       json
// @Produce      json
// @Param        request body RetryRequest true "Retry options (failure_ids required)"
// @Success      200 {object} RetryResponse
// @Failure      400 {object} map[string]string
// @Security     WorkspaceRegisteredAuth
// @Security     CSRFToken
// @Router       /api/v1/code/summarize/retry [post]
func RetryCodeSummarize(
	getSummarizer func() CodeSummarizer,
	queries CodeSummarizeRetryQuerier,
	cfg config.CodeSummarizationConfig,
	logger zerolog.Logger,
) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req RetryRequest
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
		reqLog := LoggerFromCtx(c, logger)

		if len(req.FailureIDs) == 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "failure_ids must be provided")
		}

		failureIDs := make([]uuid.UUID, 0, len(req.FailureIDs))
		for _, idStr := range req.FailureIDs {
			id, err := uuid.Parse(idStr)
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "invalid failure_id format")
			}
			failureIDs = append(failureIDs, id)
		}

		failures, err := queries.GetUnresolvedFailures(ctx, workspace)
		if err != nil {
			reqLog.Error().Err(err).Msg("failed to get unresolved failures")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get unresolved failures")
		}

		failuresToRetry := make(map[uuid.UUID]sqlc.GetUnresolvedFailuresRow)
		for _, f := range failures {
			for _, id := range failureIDs {
				if f.ID == id {
					failuresToRetry[id] = f
					break
				}
			}
		}

		if len(failuresToRetry) == 0 {
			return c.JSON(http.StatusOK, RetryResponse{
				Retried:   0,
				Succeeded: 0,
				Failed:    0,
				Message:   "no matching failures found",
			})
		}

		hashes := make([]string, 0, len(failuresToRetry))
		for _, f := range failuresToRetry {
			hashes = append(hashes, f.ContentHash)
		}
		_, _, _, err = summarizer.RetryContentHashes(ctx, workspace, hashes)
		if err != nil {
			if errors.Is(err, codesummarize.ErrBudgetExhausted) {
				return c.JSON(http.StatusOK, RetryResponse{
					Retried:   len(failuresToRetry),
					Succeeded: 0,
					Failed:    len(failuresToRetry),
					Message:   "daily code summarization budget exhausted",
				})
			}

			reqLog.Error().Err(err).Msg("code summarization retry failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "code summarization retry failed")
		}

		remaining, err := queries.GetUnresolvedFailures(ctx, workspace)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to verify retry results")
		}
		stillFailed := make(map[uuid.UUID]bool, len(remaining))
		for _, f := range remaining {
			stillFailed[f.ID] = true
		}
		succeeded := 0
		for id := range failuresToRetry {
			if !stillFailed[id] {
				succeeded++
			}
		}
		failed := len(failuresToRetry) - succeeded

		reqLog.Info().
			Int("retried", len(failuresToRetry)).
			Int("succeeded", succeeded).
			Int("failed", failed).
			Msg("code summarization retry completed")

		return c.JSON(http.StatusOK, RetryResponse{
			Retried:   len(failuresToRetry),
			Succeeded: succeeded,
			Failed:    failed,
		})
	}
}

// RetryAllCodeSummarize retries all failed code summarizations for a workspace.
//
// @Summary      Retry all failed code summarizations
// @Description  Retries code summarization for every unresolved failure in the workspace
// @Tags         code-summarize
// @Accept       json
// @Produce      json
// @Param        request body RetryRequest true "Retry options"
// @Success      200 {object} RetryResponse
// @Failure      400 {object} map[string]string
// @Security     WorkspaceRegisteredAuth
// @Security     CSRFToken
// @Router       /api/v1/code/summarize/retry-all [post]
func RetryAllCodeSummarize(
	getSummarizer func() CodeSummarizer,
	queries CodeSummarizeRetryQuerier,
	cfg config.CodeSummarizationConfig,
	logger zerolog.Logger,
) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req RetryRequest
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
		reqLog := LoggerFromCtx(c, logger)

		failures, err := queries.GetUnresolvedFailures(ctx, workspace)
		if err != nil {
			reqLog.Error().Err(err).Msg("failed to get unresolved failures")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get unresolved failures")
		}

		if len(failures) == 0 {
			return c.JSON(http.StatusOK, RetryResponse{
				Retried:   0,
				Succeeded: 0,
				Failed:    0,
				Message:   "no failures to retry",
			})
		}

		hashes := make([]string, 0, len(failures))
		for _, f := range failures {
			hashes = append(hashes, f.ContentHash)
		}
		_, _, _, err = summarizer.RetryContentHashes(ctx, workspace, hashes)
		if err != nil {
			if errors.Is(err, codesummarize.ErrBudgetExhausted) {
				return c.JSON(http.StatusOK, RetryResponse{
					Retried:   len(failures),
					Succeeded: 0,
					Failed:    len(failures),
					Message:   "daily code summarization budget exhausted",
				})
			}

			reqLog.Error().Err(err).Msg("code summarization retry-all failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "code summarization retry-all failed")
		}

		remaining, err := queries.GetUnresolvedFailures(ctx, workspace)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to verify retry results")
		}
		stillFailed := make(map[uuid.UUID]bool, len(remaining))
		for _, f := range remaining {
			stillFailed[f.ID] = true
		}
		succeeded := 0
		for _, f := range failures {
			if !stillFailed[f.ID] {
				succeeded++
			}
		}
		failed := len(failures) - succeeded

		reqLog.Info().
			Int("retried", len(failures)).
			Int("succeeded", succeeded).
			Int("failed", failed).
			Msg("code summarization retry-all completed")

		return c.JSON(http.StatusOK, RetryResponse{
			Retried:   len(failures),
			Succeeded: succeeded,
			Failed:    failed,
		})
	}
}
