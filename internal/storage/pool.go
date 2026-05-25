package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/rs/zerolog"
)

func parsePoolConfig(url string) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}
	cfg.MaxConns = 10
	cfg.HealthCheckPeriod = 30 * time.Second
	return cfg, nil
}

// NewPool creates and validates a PostgreSQL connection pool.
func NewPool(ctx context.Context, cfg config.DatabaseConfig, logger zerolog.Logger) (*pgxpool.Pool, error) {
	poolCfg, err := parsePoolConfig(cfg.URL)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info().Str("url", maskPassword(cfg.URL)).Msg("database pool connected")
	return pool, nil
}

// ClosePool closes the connection pool.
func ClosePool(pool *pgxpool.Pool) {
	pool.Close()
}

func maskPassword(dsn string) string {
	for i, c := range dsn {
		if c == '@' {
			for j := i - 1; j >= 0; j-- {
				if dsn[j] == '/' && j > 0 && dsn[j-1] == '/' {
					return dsn[:j+1] + "***:***" + dsn[i:]
				}
			}
		}
	}
	return dsn
}
