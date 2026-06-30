package summarize

import (
	"context"
	"fmt"

	"github.com/nano-brain/nano-brain/internal/harvest"
	"github.com/nano-brain/nano-brain/internal/links"
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

// SetLinkExtractor forwards link dependencies to the underlying Persister.
func (s *HarvestSummarizer) SetLinkExtractor(resolver *links.Resolver, extractor *links.Extractor) {
	s.persister.SetLinkExtractor(resolver, extractor)
}

// EnsureSummaryOnDisk writes an already-stored summary markdown to disk if the
// file is absent. It maps harvest.SummaryMeta → summarize.SessionMetadata using
// the same mapping as SummarizeAndPersist, then delegates to the Persister.
// This is called on the harvester content-unchanged skip path to backfill
// summaries created before write_to_disk was enabled.
func (s *HarvestSummarizer) EnsureSummaryOnDisk(ctx context.Context, summaryMarkdown string, meta harvest.SummaryMeta) error {
	sessionMeta := SessionMetadata{
		Source:        Source(meta.Source),
		SessionID:     meta.SessionID,
		Title:         meta.Title,
		Agent:         meta.Agent,
		ProjectPath:   meta.ProjectPath,
		CreatedAt:     meta.CreatedAt,
		Duration:      meta.Duration,
		ParentID:      meta.ParentID,
		Branch:        meta.Branch,
		Cwd:           meta.Cwd,
		Tags:          meta.Tags,
		WorkspaceHash: meta.WorkspaceHash,
	}
	s.persister.EnsureSummaryOnDisk(ctx, summaryMarkdown, sessionMeta)
	return nil
}

func (s *HarvestSummarizer) SummarizeAndPersist(ctx context.Context, content string, meta harvest.SummaryMeta) error {
	s.logger.Info().
		Str("session_id", meta.SessionID).
		Str("source", meta.Source).
		Str("title", meta.Title).
		Int("content_len", len(content)).
		Msg("summarize: starting summarization")

	sessionMeta := SessionMetadata{
		Source:        Source(meta.Source),
		SessionID:     meta.SessionID,
		Title:         meta.Title,
		Agent:         meta.Agent,
		ProjectPath:   meta.ProjectPath,
		CreatedAt:     meta.CreatedAt,
		Duration:      meta.Duration,
		ParentID:      meta.ParentID,
		Branch:        meta.Branch,
		Cwd:           meta.Cwd,
		Tags:          meta.Tags,
		WorkspaceHash: meta.WorkspaceHash,
	}
	summary, err := s.pipeline.Summarize(ctx, content, sessionMeta)
	if err != nil {
		s.logger.Error().Err(err).Str("session_id", meta.SessionID).Msg("summarize: pipeline failed")
		return fmt.Errorf("summarize: %w", err)
	}

	s.logger.Info().Str("session_id", meta.SessionID).Int("summary_len", len(summary)).Msg("summarize: pipeline done, persisting")
	return s.persister.Save(ctx, summary, sessionMeta)
}
