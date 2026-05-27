package handlers

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/harvest"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

const (
	summarizeDefaultLimit = 10
	summarizeMaxLimit     = 20
)

type SummarizeRequest struct {
	Workspace string `json:"workspace"`
	Source    string `json:"source"`
	Limit     int    `json:"limit"`
	Force     bool   `json:"force"`
}

type SummarizeResponse struct {
	Summarized int `json:"summarized"`
	Skipped    int `json:"skipped"`
	Errors     int `json:"errors"`
}

type SummarizeSummarizer interface {
	SummarizeAndPersist(ctx context.Context, content string, meta harvest.SummaryMeta) error
}

type SummarizeQuerier interface {
	ListSessionDocumentsByWorkspace(ctx context.Context, arg sqlc.ListSessionDocumentsByWorkspaceParams) ([]sqlc.ListSessionDocumentsByWorkspaceRow, error)
	GetDocumentBySourcePath(ctx context.Context, arg sqlc.GetDocumentBySourcePathParams) (sqlc.Document, error)
}

func TriggerSummarize(
	getSummarizer func() SummarizeSummarizer,
	queries SummarizeQuerier,
	logger zerolog.Logger,
) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req SummarizeRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		workspace := c.Get("workspace").(string)

		s := getSummarizer()
		if s == nil {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "summarization not configured")
		}

		lim := req.Limit
		if lim <= 0 {
			lim = summarizeDefaultLimit
		}
		if lim > summarizeMaxLimit {
			lim = summarizeMaxLimit
		}

		tagFilter := req.Source
		if tagFilter == "claude" {
			tagFilter = "claude_code"
		}

		ctx := c.Request().Context()
		docs, err := queries.ListSessionDocumentsByWorkspace(ctx, sqlc.ListSessionDocumentsByWorkspaceParams{
			WorkspaceHash: workspace,
			TagFilter:     tagFilter,
			Lim:           int32(lim),
		})
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("list sessions: %v", err))
		}

		reqLog := LoggerFromCtx(c, logger)
		var summarized, skipped, errors int

		for _, doc := range docs {
			sessionID := path.Base(doc.SourcePath)
			summaryPath := "session-summary://" + sourceFromTags(doc.Tags) + "/" + sessionID

			if !req.Force {
				existing, lookupErr := queries.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
					WorkspaceHash: workspace,
					SourcePath:    summaryPath,
				})
				if lookupErr == nil && existing.ID != uuid.Nil {
					skipped++
					continue
				}
			}

			source := sourceFromTags(doc.Tags)
			meta := harvest.SummaryMeta{
				Source:    source,
				SessionID: sessionID,
				Title:     strings.TrimPrefix(doc.Title, "Session: "),
				CreatedAt: doc.CreatedAt,
			}

			if err := s.SummarizeAndPersist(ctx, doc.Content, meta); err != nil {
				reqLog.Warn().Err(err).Str("session_id", sessionID).Msg("summarize_failed")
				errors++
				continue
			}
			summarized++
		}

		reqLog.Info().
			Str("workspace", workspace).
			Int("summarized", summarized).
			Int("skipped", skipped).
			Int("errors", errors).
			Msg("summarize triggered")

		return c.JSON(http.StatusOK, SummarizeResponse{
			Summarized: summarized,
			Skipped:    skipped,
			Errors:     errors,
		})
	}
}

func sourceFromTags(tags []string) string {
	for _, t := range tags {
		if t == "claude_code" {
			return "claude"
		}
		if t == "opencode" {
			return "opencode"
		}
	}
	return "opencode"
}
