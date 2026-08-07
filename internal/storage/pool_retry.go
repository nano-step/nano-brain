package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/rs/zerolog"
)

// NewPoolWithRetry creates and validates a PostgreSQL pool for the long-lived
// foreground server, retrying transient connection failures (connection
// refused, timeout, DNS) with a 1s/2s/4s exponential backoff capped at 30s so
// a managed service can stay alive while PostgreSQL becomes available.
//
// It deliberately does NOT change the fail-fast behavior of NewPool: a
// malformed configuration still fails immediately, and one-shot callers keep
// the single-attempt contract. Retry is cancellation-aware and leaves no pool
// behind when the context is cancelled.
func NewPoolWithRetry(ctx context.Context, cfg config.DatabaseConfig, logger zerolog.Logger) (*pgxpool.Pool, error) {
	// Configuration errors are permanent — fail immediately, never retry.
	if _, err := parsePoolConfig(cfg.URL); err != nil {
		return nil, err
	}

	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for attempt := 1; ; attempt++ {
		pool, err := NewPool(ctx, cfg, logger)
		if err == nil {
			return pool, nil
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("database connection retry cancelled: %w", ctx.Err())
		case <-time.After(backoff):
		}

		logger.Warn().
			Err(err).
			Int("attempt", attempt).
			Dur("next_retry", backoff).
			Msg("database connection failed; PostgreSQL may still be starting")
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}
