package codesummarize

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
	BulkGetCallerContext(ctx context.Context, arg sqlc.BulkGetCallerContextParams) ([]sqlc.BulkGetCallerContextRow, error)
	BulkGetCalleeNodes(ctx context.Context, arg sqlc.BulkGetCalleeNodesParams) ([]sqlc.BulkGetCalleeNodesRow, error)
	UpdateChunkGraphContextHash(ctx context.Context, arg sqlc.UpdateChunkGraphContextHashParams) error
	GetSymbolChunksByGraphContextStale(ctx context.Context, arg sqlc.GetSymbolChunksByGraphContextStaleParams) ([]sqlc.GetSymbolChunksByGraphContextStaleRow, error)
	NullifyGraphContextHashBySymbols(ctx context.Context, arg sqlc.NullifyGraphContextHashBySymbolsParams) error
}

type WorkspaceLister interface {
	ListWorkspaces(ctx context.Context) ([]sqlc.Workspace, error)
}

type EmbedQueue interface {
	Enqueue(id uuid.UUID) bool
}

type Service struct {
	cfg             config.CodeSummarizationConfig
	provider        *LLMProvider
	budget          *BudgetTracker
	queries         ServiceQuerier
	workspaceLister WorkspaceLister
	embedQ          EmbedQueue
	logger          zerolog.Logger
	notifyCh        chan struct{}
	processing      atomic.Bool
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
		notifyCh: make(chan struct{}, 1),
	}
}

func (s *Service) WithWorkspaceLister(wl WorkspaceLister) *Service {
	s.workspaceLister = wl
	return s
}

func (s *Service) tryAcquireProcessing() bool {
	return s.processing.CompareAndSwap(false, true)
}

func (s *Service) releaseProcessing() {
	s.processing.Store(false)
}

func (s *Service) Notify() {
	select {
	case s.notifyCh <- struct{}{}:
	default:
	}
}

// splitAndSend recursively splits a batch if it exceeds token limits, then sends with retry.
// Returns (processed count, error count).
func (s *Service) splitAndSend(ctx context.Context, workspaceHash string, batch []SymbolForSummary, graphContexts map[string]*SymbolGraphContext) (processed, errors int) {
	estimated := EstimateTokens(batch)
	maxTokens := s.cfg.MaxBatchTokens
	if maxTokens <= 0 {
		maxTokens = 100000
	}

	if estimated > maxTokens && len(batch) > 1 {
		mid := len(batch) / 2
		p1, e1 := s.splitAndSend(ctx, workspaceHash, batch[:mid], graphContexts)
		p2, e2 := s.splitAndSend(ctx, workspaceHash, batch[mid:], graphContexts)
		return p1 + p2, e1 + e2
	}

	summaries, err := s.sendWithRetry(ctx, batch, graphContexts)
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
		var matchedIdx int
		for i := range batch {
			if matched[i] {
				continue
			}
			if batch[i].Name == summary.Name && batch[i].File == summary.File {
				matchedSymbol = &batch[i]
				matchedIdx = i
				matched[matchedIdx] = true
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

		if graphContexts != nil {
			nodeKey := matchedSymbol.File + "::" + matchedSymbol.Name
			if gc, ok := graphContexts[nodeKey]; ok {
				gcHash := ComputeGraphContextHash(gc)
				if gcHash != "" {
					if dbErr := s.queries.UpdateChunkGraphContextHash(ctx, sqlc.UpdateChunkGraphContextHashParams{
						ID:               matchedSymbol.ChunkID,
						GraphContextHash: sql.NullString{String: gcHash, Valid: true},
					}); dbErr != nil {
						s.logger.Warn().Err(dbErr).Str("symbol", matchedSymbol.Name).Msg("failed to update graph context hash")
					}
				}
			}
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
			ChunkID:     row.ID,
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

	symbolNodes := make([]string, 0, len(symbols)+len(oversized))
	for _, sym := range symbols {
		symbolNodes = append(symbolNodes, sym.File+"::"+sym.Name)
	}
	for _, sym := range oversized {
		symbolNodes = append(symbolNodes, sym.File+"::"+sym.Name)
	}

	graphContexts, err := FetchGraphContext(ctx, s.queries, workspaceHash, symbolNodes)
	if err != nil {
		s.logger.Warn().Err(err).Msg("failed to fetch graph context, proceeding without it")
		graphContexts = nil
	}

	for _, batch := range batches {
		exhausted, err := s.budget.IsExhausted(ctx, workspaceHash, s.cfg.MaxRequestsPerDay)
		if err != nil {
			s.logger.Warn().Err(err).Msg("failed to check budget inside loop")
		} else if exhausted {
			s.logger.Warn().Str("workspace", workspaceHash).Msg("budget exhausted mid-cycle")
			break
		}

		p, e := s.splitAndSend(ctx, workspaceHash, batch, graphContexts)
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

		p, e := s.splitAndSend(ctx, workspaceHash, []SymbolForSummary{sym}, graphContexts)
		processed += p
		errors += e
	}

	s.processGraphContextStale(ctx, workspaceHash, &processed, &errors)

	s.logger.Info().
		Str("workspace", workspaceHash).
		Int("processed", processed).
		Int("skipped", skipped).
		Int("errors", errors).
		Msg("code summarization completed")

	return processed, skipped, errors, nil
}

func (s *Service) processGraphContextStale(ctx context.Context, workspaceHash string, processed, errors *int) {
	staleRows, err := s.queries.GetSymbolChunksByGraphContextStale(ctx, sqlc.GetSymbolChunksByGraphContextStaleParams{
		WorkspaceHash: workspaceHash,
		Limit:         20,
	})
	if err != nil {
		s.logger.Warn().Err(err).Msg("failed to query graph-context-stale symbols")
		return
	}
	if len(staleRows) == 0 {
		return
	}

	staleSymbols := make([]SymbolForSummary, 0, len(staleRows))
	staleNodes := make([]string, 0, len(staleRows))
	for _, row := range staleRows {
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
			ChunkID:     row.ID,
			Name:        symbolName,
			Kind:        symbolKind,
			File:        row.SourcePath,
			Language:    language,
			Code:        row.Content,
			ContentHash: row.ContentHash,
		}
		staleSymbols = append(staleSymbols, sym)
		staleNodes = append(staleNodes, sym.File+"::"+sym.Name)
	}

	graphContexts, err := FetchGraphContext(ctx, s.queries, workspaceHash, staleNodes)
	if err != nil {
		s.logger.Warn().Err(err).Msg("failed to fetch graph context for stale symbols")
		return
	}

	needsResummarize := make([]SymbolForSummary, 0)
	for i, sym := range staleSymbols {
		nodeKey := sym.File + "::" + sym.Name
		gc := graphContexts[nodeKey]
		newHash := ComputeGraphContextHash(gc)

		oldHash := ""
		if staleRows[i].GraphContextHash.Valid {
			oldHash = staleRows[i].GraphContextHash.String
		}

		if newHash != oldHash {
			needsResummarize = append(needsResummarize, sym)
		} else if newHash != "" {
			if err := s.queries.UpdateChunkGraphContextHash(ctx, sqlc.UpdateChunkGraphContextHashParams{
				ID:               sym.ChunkID,
				GraphContextHash: sql.NullString{String: newHash, Valid: true},
			}); err != nil {
				s.logger.Warn().Err(err).Str("symbol", sym.Name).Msg("failed to mark graph context as current")
			}
		}
	}

	if len(needsResummarize) == 0 {
		return
	}

	s.logger.Info().
		Str("workspace", workspaceHash).
		Int("graph_stale_count", len(needsResummarize)).
		Msg("re-summarizing symbols with changed graph context")

	p, e := s.splitAndSend(ctx, workspaceHash, needsResummarize, graphContexts)
	*processed += p
	*errors += e
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

func (s *Service) StartWorkerPool(ctx context.Context) {
	workers := s.cfg.Workers
	if workers <= 0 {
		workers = 2
	}

	pollInterval := time.Duration(s.cfg.PollIntervalSeconds) * time.Second
	if pollInterval <= 0 {
		pollInterval = 10 * time.Second
	}

	s.logger.Info().Int("workers", workers).Dur("poll_interval", pollInterval).Msg("starting code summarization worker pool")

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			s.runWorker(ctx, workerID, pollInterval)
		}(i)
	}

	wg.Wait()
	s.logger.Info().Msg("code summarization worker pool stopped")
}

func (s *Service) runWorker(ctx context.Context, id int, pollInterval time.Duration) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	backoff := time.Duration(0)

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.notifyCh:
			if !s.tryAcquireProcessing() {
				continue
			}
			backoff = s.processAllWorkspaces(ctx, id, backoff)
			s.releaseProcessing()
		case <-ticker.C:
			if !s.tryAcquireProcessing() {
				continue
			}
			backoff = s.processAllWorkspaces(ctx, id, backoff)
			s.releaseProcessing()
		}
	}
}

func (s *Service) processAllWorkspaces(ctx context.Context, workerID int, currentBackoff time.Duration) time.Duration {
	if currentBackoff > 0 {
		select {
		case <-ctx.Done():
			return 0
		case <-time.After(currentBackoff):
		}
	}

	if s.workspaceLister == nil {
		return 0
	}

	workspaces, err := s.workspaceLister.ListWorkspaces(ctx)
	if err != nil {
		s.logger.Error().Err(err).Int("worker", workerID).Msg("failed to list workspaces")
		return 0
	}

	for _, ws := range workspaces {
		if ctx.Err() != nil {
			return 0
		}

		_, _, _, err := s.RunOnce(ctx, ws.Hash)
		if err != nil {
			if errors.Is(err, ErrBudgetExhausted) {
				s.logger.Info().
					Int("worker", workerID).
					Str("workspace", ws.Hash).
					Msg("budget exhausted for workspace, skipping to next")
				continue
			}
			if isRateLimited(err) {
				newBackoff := nextBackoff(currentBackoff)
				s.logger.Warn().
					Int("worker", workerID).
					Dur("backoff", newBackoff).
					Msg("rate limited (429), backing off")
				return newBackoff
			}
			s.logger.Error().Err(err).Int("worker", workerID).Str("workspace", ws.Hash).Msg("RunOnce failed")
		}
	}

	return 0
}

func isRateLimited(err error) bool {
	return err != nil && strings.Contains(err.Error(), "429")
}

func nextBackoff(current time.Duration) time.Duration {
	if current <= 0 {
		return 60 * time.Second
	}
	next := current * 2
	if next > 900*time.Second {
		next = 900 * time.Second
	}
	return next
}
