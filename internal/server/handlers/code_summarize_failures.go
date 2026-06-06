package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type CodeSummarizationFailuresQuerier interface {
	GetUnresolvedFailures(ctx context.Context, workspaceHash string) ([]sqlc.GetUnresolvedFailuresRow, error)
}

type FailureResponse struct {
	ID            string    `json:"id"`
	SymbolName    string    `json:"symbol_name"`
	SymbolKind    string    `json:"symbol_kind"`
	SourceFile    string    `json:"source_file"`
	ErrorReason   string    `json:"error_reason"`
	ErrorType     string    `json:"error_type"`
	Attempts      int       `json:"attempts"`
	LastAttemptAt time.Time `json:"last_attempt_at"`
}

func GetCodeSummarizeFailures(
	queries CodeSummarizationFailuresQuerier,
	logger zerolog.Logger,
) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspace, ok := c.Get("workspace").(string)
		if !ok || workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace not found in context")
		}

		ctx := c.Request().Context()
		failures, err := queries.GetUnresolvedFailures(ctx, workspace)
		if err != nil {
			reqLog := LoggerFromCtx(c, logger)
			reqLog.Error().Err(err).Str("workspace", workspace).Msg("failed to get unresolved failures")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get unresolved failures")
		}

		response := make([]FailureResponse, len(failures))
		for i, f := range failures {
			symbolKind := ""
			if f.SymbolKind.Valid {
				symbolKind = f.SymbolKind.String
			}
			response[i] = FailureResponse{
				ID:            f.ID.String(),
				SymbolName:    f.SymbolName,
				SymbolKind:    symbolKind,
				SourceFile:    f.SourceFile,
				ErrorReason:   f.ErrorReason,
				ErrorType:     f.ErrorType,
				Attempts:      int(f.Attempts),
				LastAttemptAt: f.LastAttemptAt,
			}
		}

		return c.JSON(http.StatusOK, response)
	}
}
