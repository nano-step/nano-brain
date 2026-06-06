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

		// Run the summarizer to reprocess
		_, _, errs, err := summarizer.RunOnce(ctx, workspace)
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

		succeeded := 0
		failed := errs
		for id := range failuresToRetry {
			if err := queries.ResolveFailure(ctx, id); err != nil {
				reqLog.Warn().Err(err).Str("failure_id", id.String()).Msg("failed to resolve failure")
				failed++
			} else {
				succeeded++
			}
		}

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

		// Run the summarizer to reprocess
		_, _, errs, err := summarizer.RunOnce(ctx, workspace)
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

		succeeded := 0
		failed := errs
		for _, f := range failures {
			if err := queries.ResolveFailure(ctx, f.ID); err != nil {
				reqLog.Warn().Err(err).Str("failure_id", f.ID.String()).Msg("failed to resolve failure")
				failed++
			} else {
				succeeded++
			}
		}

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
