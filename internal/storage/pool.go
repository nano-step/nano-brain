package storage

import (
	"context"
	"fmt"
	"regexp"
	"strings"
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

// keywordPassword matches the password of a keyword/value DSN
// (host=... password=secret, value optionally single-quoted with backslash
// escapes) and a password= query parameter of a URL DSN. The key must end in
// "password" so sslpassword= is covered too, and an unquoted value runs to
// the next whitespace so `&` and `"` inside it cannot leak the tail.
var keywordPassword = regexp.MustCompile(`(?i)(^|[\s?&])(\w*password\s*=\s*)('(?:[^'\\]|\\.)*'|[^\s]+)`)

// dsnParamStart finds a `&key=` parameter boundary inside a matched
// keyword-DSN value so the tail is preserved rather than eaten as password
// text.
var dsnParamStart = regexp.MustCompile(`&[A-Za-z0-9_]+=`)

// maskDSNValue builds the replacement for a keyword-DSN password match: the
// key stays, the value becomes ***, and a `&key=` tail (a URL query parameter
// or the next keyword pair) is kept.
func maskDSNValue(m string) string {
	g := keywordPassword.FindStringSubmatchIndex(m)
	if g == nil || g[6] < 0 {
		return m
	}
	v := m[g[6]:g[7]]
	tail := ""
	if !strings.HasPrefix(v, "'") {
		if loc := dsnParamStart.FindStringIndex(v); loc != nil {
			tail = v[loc[0]:]
		}
	}
	return m[:g[6]] + "***" + tail
}

func maskPassword(dsn string) string {
	dsn = keywordPassword.ReplaceAllStringFunc(dsn, maskDSNValue)
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
