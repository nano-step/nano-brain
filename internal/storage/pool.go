package storage

import (
	"context"
	"fmt"
	"regexp"
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

// dsnURLPassword matches a *password= query parameter inside a URL DSN
// (postgres://u@h/db?password=x&sslpassword=y). Each parameter is masked
// independently, so every credential in the query is covered; a literal &
// inside a URL password must be written %26, so the value ends at & or at
// the fragment.
var dsnURLPassword = regexp.MustCompile(`(?i)([?&]\w*password=)[^&#\s]*`)

// dsnKeywordPassword matches *password=value in a whitespace-separated
// keyword DSN (host=... password=secret, value optionally single-quoted with
// backslash escapes). The key must end in "password" so sslpassword= is
// covered. An unquoted value runs to the next whitespace and is masked whole:
// pgx keeps `abc&tail=X` as the password — keyword pairs are separated by
// whitespace only, so a `&key=` inside the value is password bytes, not a
// parameter boundary.
var dsnKeywordPassword = regexp.MustCompile(`(?i)(^|\s)(\w*password\s*=\s*)('(?:[^'\\]|\\.)*'|\S+)`)

// MaskDSNPasswords masks every *password= value in a database DSN, in both
// URL form (postgres://u@h/db?password=x) and keyword form
// (host=... password=secret). The key is kept, the value becomes ***. The
// URL-query pass runs first; the keyword pass requires the key at the start
// or after whitespace, so it never re-matches a URL query parameter.
func MaskDSNPasswords(dsn string) string {
	dsn = dsnURLPassword.ReplaceAllString(dsn, "${1}***")
	return dsnKeywordPassword.ReplaceAllString(dsn, "${1}${2}***")
}

func maskPassword(dsn string) string {
	dsn = MaskDSNPasswords(dsn)
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
