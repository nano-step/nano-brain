package codesummarize

import (
	"context"
	"regexp"
	"strings"
	"time"
)

type ErrorClass int

const (
	ErrorTransient ErrorClass = iota
	ErrorPermanent
)

var (
	transientRegex = regexp.MustCompile(`\b(429|408|500|502|503|504)\b`)
	permanentRegex = regexp.MustCompile(`\b(400|401|403)\b`)
)

func ClassifyError(err error) ErrorClass {
	if err == nil {
		return ErrorTransient
	}
	errStr := err.Error()

	if transientRegex.MatchString(errStr) ||
		strings.Contains(errStr, "context deadline") ||
		strings.Contains(errStr, "context canceled") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") {
		return ErrorTransient
	}

	if permanentRegex.MatchString(errStr) {
		return ErrorPermanent
	}

	return ErrorTransient
}

// sendWithRetry attempts to send a batch to the LLM provider with exponential backoff.
// Returns summaries on success, or the last error after exhausting retries.
func (s *Service) sendWithRetry(ctx context.Context, batch []SymbolForSummary, graphContexts map[string]*SymbolGraphContext) ([]SymbolSummary, error) {
	maxRetries := s.cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	backoffBase := time.Duration(s.cfg.RetryBackoffSeconds) * time.Second
	if backoffBase <= 0 {
		backoffBase = time.Second
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		summaries, err := s.provider.SummarizeBatch(ctx, batch, graphContexts)
		if err == nil {
			return summaries, nil
		}

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		lastErr = err
		errClass := ClassifyError(err)

		if errClass == ErrorPermanent {
			s.logger.Error().
				Err(err).
				Int("batch_size", len(batch)).
				Str("error_type", "permanent").
				Msg("batch failed permanently, not retrying")
			return nil, err
		}

		if attempt < maxRetries {
			backoff := backoffBase * time.Duration(attempt*attempt)
			s.logger.Warn().
				Err(err).
				Int("attempt", attempt).
				Int("max", maxRetries).
				Dur("backoff", backoff).
				Int("batch_size", len(batch)).
				Msg("retrying batch after transient error")

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
	}

	s.logger.Error().
		Err(lastErr).
		Int("attempts", maxRetries).
		Int("batch_size", len(batch)).
		Msg("batch failed after max retries")
	return nil, lastErr
}
