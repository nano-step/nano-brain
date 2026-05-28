package summarize

import (
	"context"
	"fmt"

	"github.com/nano-brain/nano-brain/internal/harvest"
	"github.com/rs/zerolog"
)

// HarvestSummarizer adapts Pipeline + Persister to the harvest.SessionSummarizer interface.
type HarvestSummarizer struct {
	pipeline  *Pipeline
	persister *Persister
	logger    zerolog.Logger
}

func NewHarvestSummarizer(pipeline *Pipeline, persister *Persister, logger zerolog.Logger) *HarvestSummarizer {
	return &HarvestSummarizer{pipeline: pipeline, persister: persister, logger: logger}
}

func (s *HarvestSummarizer) SummarizeAndPersist(ctx context.Context, content string, meta harvest.SummaryMeta) error {
	s.logger.Info().
		Str("session_id", meta.SessionID).
		Str("source", meta.Source).
		Str("title", meta.Title).
		Int("content_len", len(content)).
		Msg("summarize: starting summarization")

	sessionMeta := SessionMetadata{
		Source:      Source(meta.Source),
		SessionID:   meta.SessionID,
		Title:       meta.Title,
		Agent:       meta.Agent,
		ProjectPath: meta.ProjectPath,
		CreatedAt:   meta.CreatedAt,
		Duration:    meta.Duration,
		ParentID:    meta.ParentID,
	}
	summary, err := s.pipeline.Summarize(ctx, content, sessionMeta)
	if err != nil {
		s.logger.Error().Err(err).Str("session_id", meta.SessionID).Msg("summarize: pipeline failed")
		return fmt.Errorf("summarize: %w", err)
	}

	s.logger.Info().Str("session_id", meta.SessionID).Int("summary_len", len(summary)).Msg("summarize: pipeline done, persisting")
	return s.persister.Save(ctx, summary, sessionMeta)
}
