package codesummarize

import (
	"context"
	"strings"
	"time"
)

type ErrorClass int

const (
	ErrorTransient ErrorClass = iota
	ErrorPermanent
)

// ClassifyError categorizes an error as transient (retryable) or permanent.
// Transient: 429, 408, 5xx, timeout, context errors
// Permanent: 400, 401, 403
// Default: transient (safer to retry than to skip)
func ClassifyError(err error) ErrorClass {
	errStr := err.Error()

	transientPatterns := []string{
		"429", "408", "500", "502", "503", "504",
		"context deadline", "context canceled", "timeout",
		"connection refused", "connection reset",
	}
	for _, p := range transientPatterns {
		if strings.Contains(errStr, p) {
			return ErrorTransient
		}
	}

	permanentPatterns := []string{"400", "401", "403"}
	for _, p := range permanentPatterns {
		if strings.Contains(errStr, p) {
			return ErrorPermanent
		}
	}

	// Default to transient (safer — will retry)
	return ErrorTransient
}

// sendWithRetry attempts to send a batch to the LLM provider with exponential backoff.
// Returns summaries on success, or the last error after exhausting retries.
func (s *Service) sendWithRetry(ctx context.Context, batch []SymbolForSummary) ([]SymbolSummary, error) {
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
		summaries, err := s.provider.SummarizeBatch(ctx, batch)
		if err == nil {
			return summaries, nil
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
