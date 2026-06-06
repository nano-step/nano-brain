package codesummarize

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
	"github.com/sqlc-dev/pqtype"
)

type ServiceQuerier interface {
	GetUnsummarizedSymbols(ctx context.Context, arg sqlc.GetUnsummarizedSymbolsParams) ([]sqlc.GetUnsummarizedSymbolsRow, error)
	UpsertDocument(ctx context.Context, arg sqlc.UpsertDocumentParams) (sqlc.UpsertDocumentRow, error)
	ListChunksByDocumentID(ctx context.Context, arg sqlc.ListChunksByDocumentIDParams) ([]sqlc.ListChunksByDocumentIDRow, error)
}

type EmbedQueue interface {
	Enqueue(id uuid.UUID) bool
}

type Service struct {
	cfg      config.CodeSummarizationConfig
	provider *LLMProvider
	budget   *BudgetTracker
	queries  ServiceQuerier
	embedQ   EmbedQueue
	logger   zerolog.Logger
}

func NewService(
	cfg config.CodeSummarizationConfig,
	provider *LLMProvider,
	budget *BudgetTracker,
	queries ServiceQuerier,
	embedQ EmbedQueue,
	logger zerolog.Logger,
) *Service {
	return &Service{
		cfg:      cfg,
		provider: provider,
		budget:   budget,
		queries:  queries,
		embedQ:   embedQ,
		logger:   logger.With().Str("component", "code-summarize-service").Logger(),
	}
}

func (s *Service) RunOnce(ctx context.Context, workspaceHash string) (processed, skipped, errors int, err error) {
	exhausted, err := s.budget.IsExhausted(ctx, workspaceHash, s.cfg.MaxRequestsPerDay)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("check budget: %w", err)
	}
	if exhausted {
		s.logger.Warn().Str("workspace", workspaceHash).Msg("budget exhausted for today")
		return 0, 0, 0, ErrBudgetExhausted
	}

	maxCycle := s.cfg.MaxSummariesPerCycle
	if maxCycle <= 0 {
		maxCycle = 100
	}

	rows, err := s.queries.GetUnsummarizedSymbols(ctx, sqlc.GetUnsummarizedSymbolsParams{
		WorkspaceHash: workspaceHash,
		Limit:         int32(maxCycle),
	})
	if err != nil {
		return 0, 0, 0, fmt.Errorf("get unsummarized symbols: %w", err)
	}

	if len(rows) == 0 {
		s.logger.Info().Str("workspace", workspaceHash).Msg("no unsummarized symbols found")
		return 0, 0, 0, nil
	}

	symbols := make([]SymbolForSummary, 0, len(rows))
	for _, row := range rows {
		symbolName := ""
		if row.SymbolName.Valid {
			symbolName = row.SymbolName.String
		}
		symbolKind := ""
		if row.SymbolKind.Valid {
			symbolKind = row.SymbolKind.String
		}
		language := ""
		if row.Language.Valid {
			language = row.Language.String
		}

		lineCount := strings.Count(row.Content, "\n") + 1
		if s.cfg.MaxSymbolLines > 0 && lineCount > s.cfg.MaxSymbolLines {
			s.logger.Debug().
				Str("symbol", symbolName).
				Int("lines", lineCount).
				Int("max", s.cfg.MaxSymbolLines).
				Msg("symbol too large, skipping")
			skipped++
			continue
		}

		symbols = append(symbols, SymbolForSummary{
			Name:        symbolName,
			Kind:        symbolKind,
			File:        row.SourcePath,
			Language:    language,
			Code:        row.Content,
			ContentHash: row.ContentHash,
		})
	}

	if len(symbols) == 0 {
		s.logger.Info().Str("workspace", workspaceHash).Msg("all symbols filtered out (too large)")
		return 0, skipped, 0, nil
	}

	batchSize := s.cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 5
	}

	batches := make([][]SymbolForSummary, 0)
	for i := 0; i < len(symbols); i += batchSize {
		end := i + batchSize
		if end > len(symbols) {
			end = len(symbols)
		}
		batches = append(batches, symbols[i:end])
	}

	s.logger.Info().
		Str("workspace", workspaceHash).
		Int("total_symbols", len(symbols)).
		Int("batches", len(batches)).
		Int("batch_size", batchSize).
		Msg("starting code summarization")

	for _, batch := range batches {
		exhausted, err := s.budget.IsExhausted(ctx, workspaceHash, s.cfg.MaxRequestsPerDay)
		if err != nil {
			s.logger.Warn().Err(err).Msg("failed to check budget inside loop")
		} else if exhausted {
			s.logger.Warn().Str("workspace", workspaceHash).Msg("budget exhausted mid-cycle")
			break
		}

		summaries, err := s.provider.SummarizeBatch(ctx, batch)
		if err != nil {
			s.logger.Error().Err(err).Int("batch_size", len(batch)).Msg("batch summarization failed")
			errors += len(batch)
			continue
		}

		for _, summary := range summaries {
			var matchedSymbol *SymbolForSummary
			for i := range batch {
				if batch[i].Name == summary.Name && batch[i].File == summary.File {
					matchedSymbol = &batch[i]
					break
				}
			}
			if matchedSymbol == nil {
				s.logger.Warn().
					Str("name", summary.Name).
					Str("file", summary.File).
					Msg("summary returned for unknown symbol")
				errors++
				continue
			}

			if err := s.upsertSummaryDocument(ctx, workspaceHash, matchedSymbol, summary.Summary); err != nil {
				s.logger.Error().
					Err(err).
					Str("symbol", matchedSymbol.Name).
					Str("file", matchedSymbol.File).
					Msg("upsert summary document failed")
				errors++
				continue
			}

			processed++
		}

		if err := s.budget.Increment(ctx, workspaceHash); err != nil {
			s.logger.Warn().Err(err).Msg("failed to increment budget counter")
		}
	}

	s.logger.Info().
		Str("workspace", workspaceHash).
		Int("processed", processed).
		Int("skipped", skipped).
		Int("errors", errors).
		Msg("code summarization completed")

	return processed, skipped, errors, nil
}

func (s *Service) upsertSummaryDocument(ctx context.Context, workspaceHash string, symbol *SymbolForSummary, summary string) error {
	shortHash := symbol.ContentHash
	if len(shortHash) > 8 {
		shortHash = shortHash[:8]
	}

	sourcePath := fmt.Sprintf("%s?symbol=%s&kind=%s&hash=%s&summary=true",
		symbol.File,
		symbol.Name,
		symbol.Kind,
		shortHash,
	)

	title := fmt.Sprintf("Summary: %s (%s)", symbol.Name, symbol.Kind)

	metadata := map[string]interface{}{
		"symbol_name":         symbol.Name,
		"symbol_kind":         symbol.Kind,
		"source_file":         symbol.File,
		"source_content_hash": symbol.ContentHash,
		"model_version":       s.cfg.Model,
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	result, err := s.queries.UpsertDocument(ctx, sqlc.UpsertDocumentParams{
		WorkspaceHash: workspaceHash,
		ContentHash:   computeContentHash(summary),
		Title:         title,
		Content:       summary,
		SourcePath:    sourcePath,
		Collection:    "code",
		Tags:          []string{"symbol-summary"},
		Metadata:      pqtype.NullRawMessage{RawMessage: metadataJSON, Valid: true},
		SupersedesID:  uuid.NullUUID{},
	})
	if err != nil {
		return fmt.Errorf("upsert document: %w", err)
	}

	chunks, err := s.queries.ListChunksByDocumentID(ctx, sqlc.ListChunksByDocumentIDParams{
		DocumentID:    result.ID,
		WorkspaceHash: workspaceHash,
	})
	if err != nil {
		return fmt.Errorf("list chunks: %w", err)
	}

	if s.embedQ != nil {
		for _, chunk := range chunks {
			s.embedQ.Enqueue(chunk.ID)
		}
	}

	return nil
}

func computeContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}
