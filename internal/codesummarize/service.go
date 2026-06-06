package codesummarize

import (
	"context"
	"crypto/sha256"
	"database/sql"
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
	UpsertCodeSummarizationFailure(ctx context.Context, arg sqlc.UpsertCodeSummarizationFailureParams) error
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

// splitAndSend recursively splits a batch if it exceeds token limits, then sends with retry.
// Returns (processed count, error count).
func (s *Service) splitAndSend(ctx context.Context, workspaceHash string, batch []SymbolForSummary) (processed, errors int) {
	estimated := EstimateTokens(batch)
	maxTokens := s.cfg.MaxBatchTokens
	if maxTokens <= 0 {
		maxTokens = 100000
	}

	if estimated > maxTokens && len(batch) > 1 {
		mid := len(batch) / 2
		p1, e1 := s.splitAndSend(ctx, workspaceHash, batch[:mid])
		p2, e2 := s.splitAndSend(ctx, workspaceHash, batch[mid:])
		return p1 + p2, e1 + e2
	}

	summaries, err := s.sendWithRetry(ctx, batch)
	if err != nil {
		errType := "transient_exhausted"
		if ClassifyError(err) == ErrorPermanent {
			errType = "permanent"
		}
		for _, sym := range batch {
			if dbErr := s.queries.UpsertCodeSummarizationFailure(ctx, sqlc.UpsertCodeSummarizationFailureParams{
				WorkspaceHash: workspaceHash,
				SymbolName:    sym.Name,
				SymbolKind:    sql.NullString{String: sym.Kind, Valid: sym.Kind != ""},
				SourceFile:    sym.File,
				ContentHash:   sym.ContentHash,
				ErrorReason:   err.Error(),
				ErrorType:     errType,
			}); dbErr != nil {
				s.logger.Warn().Err(dbErr).Str("symbol", sym.Name).Msg("failed to persist failure record")
			}
		}
		return 0, len(batch)
	}

	matched := make(map[int]bool)
	for _, summary := range summaries {
		var matchedSymbol *SymbolForSummary
		for i := range batch {
			if matched[i] {
				continue
			}
			if batch[i].Name == summary.Name && batch[i].File == summary.File {
				matchedSymbol = &batch[i]
				matched[i] = true
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

	return processed, errors
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
	var oversized []SymbolForSummary

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

		sym := SymbolForSummary{
			Name:        symbolName,
			Kind:        symbolKind,
			File:        row.SourcePath,
			Language:    language,
			Code:        row.Content,
			ContentHash: row.ContentHash,
		}

		lineCount := strings.Count(row.Content, "\n") + 1
		if s.cfg.MaxSymbolLines > 0 && lineCount > s.cfg.MaxSymbolLines {
			s.logger.Info().
				Str("symbol", symbolName).
				Int("lines", lineCount).
				Int("max", s.cfg.MaxSymbolLines).
				Msg("oversized symbol, will send individually")
			oversized = append(oversized, sym)
			continue
		}

		symbols = append(symbols, sym)
	}

	if len(symbols) == 0 && len(oversized) == 0 {
		s.logger.Info().Str("workspace", workspaceHash).Msg("no symbols to summarize")
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

		p, e := s.splitAndSend(ctx, workspaceHash, batch)
		processed += p
		errors += e
	}

	for _, sym := range oversized {
		exhausted, err := s.budget.IsExhausted(ctx, workspaceHash, s.cfg.MaxRequestsPerDay)
		if err != nil {
			s.logger.Warn().Err(err).Msg("failed to check budget for oversized symbol")
		} else if exhausted {
			s.logger.Warn().Str("workspace", workspaceHash).Msg("budget exhausted during oversized symbols")
			break
		}

		s.logger.Info().
			Str("symbol", sym.Name).
			Str("file", sym.File).
			Int("lines", strings.Count(sym.Code, "\n")+1).
			Msg("summarizing oversized symbol individually")

		p, e := s.splitAndSend(ctx, workspaceHash, []SymbolForSummary{sym})
		processed += p
		errors += e
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
