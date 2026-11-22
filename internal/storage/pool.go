package storage

import (
	"context"
	"fmt"
	"net/url"
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

// isASCIISpace mirrors pgx's asciiSpace table (pgconn/config.go): the bytes
// that separate keyword/value pairs in a DSN. It includes '\v', which Go's
// \s class does not.
func isASCIISpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	}
	return false
}

const dsnSpaceTrim = " \t\n\r\v\f"

// MaskDSNPasswords masks every *password= value in a database DSN. The
// URL-vs-keyword choice is pgx's own: a case-sensitive prefix test on the
// untrimmed string (pgconn/config.go), so an upper-case or space-prefixed
// scheme is tokenized as keyword/value exactly as pgx tokenizes it. The
// masked byte span is then exactly the value pgx would load. Keys are
// matched case-insensitively and must end in "password" so sslpassword= is
// covered.
func MaskDSNPasswords(dsn string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		return maskURLDSN(dsn)
	}
	return maskKeywordDSN(dsn)
}

// maskURLDSN masks the userinfo password and every *password= query
// parameter of a URL-form DSN. pgx decodes query parameter names, so each
// key is URL-unescaped before the suffix check and an undecodable key is
// treated as sensitive. The query is masked before the userinfo because an
// '@' can legitimately sit inside a parameter (a user name like
// alice@example.com); url.Parse splits the query off before it looks at the
// authority, so the '@' there is not userinfo. Everything else — other
// parameters and the fragment (which pgx does not load) — is kept
// byte-for-byte.
func maskURLDSN(dsn string) string {
	q := strings.IndexByte(dsn, '?')
	head := dsn
	if q >= 0 {
		head = dsn[:q]
	}
	out := maskDSNUserinfo(head)
	if q < 0 {
		return out
	}
	end := len(dsn)
	if h := strings.IndexByte(dsn[q+1:], '#'); h >= 0 {
		end = q + 1 + h
	}
	segments := strings.Split(dsn[q+1:end], "&")
	for i, seg := range segments {
		k, _, hasValue := strings.Cut(seg, "=")
		if !hasValue {
			continue
		}
		name, err := url.QueryUnescape(k)
		if err != nil || strings.HasSuffix(strings.ToLower(name), "password") {
			segments[i] = k + "=***"
		}
	}
	return out + "?" + strings.Join(segments, "&") + dsn[end:]
}

// maskDSNUserinfo blanks the user:password part of a URL the way the pool
// log always has. As url.Parse does, the authority runs from after "://" to
// the first '/', '?' or '#', and the userinfo ends at the last '@' inside
// it, so an '@' in the path, query or fragment is left alone.
func maskDSNUserinfo(dsn string) string {
	scheme := strings.Index(dsn, "://")
	if scheme < 0 {
		return dsn
	}
	start := scheme + 3
	end := len(dsn)
	if i := strings.IndexAny(dsn[start:], "/?#"); i >= 0 {
		end = start + i
	}
	if at := strings.LastIndexByte(dsn[start:end], '@'); at >= 0 {
		return dsn[:start] + "***:***" + dsn[start+at:]
	}
	return dsn
}

// maskKeywordDSN ports pgx's parseKeywordValueSettings (pgconn/config.go),
// recording each value's byte span rather than its decoded text so a
// *password= value can be replaced in place. The quotes of a single-quoted
// value are part of its span. Malformed input fails closed: everything from
// the first password= onward is masked.
func maskKeywordDSN(dsn string) string {
	var spans [][2]int
	rest := dsn
	for len(rest) > 0 {
		eq := strings.IndexByte(rest, '=')
		if eq < 0 {
			return maskDSNFailClosed(dsn)
		}
		key := strings.Trim(rest[:eq], dsnSpaceTrim)
		rest = strings.TrimLeft(rest[eq+1:], dsnSpaceTrim)
		valStart := len(dsn) - len(rest)
		var span [2]int
		switch {
		case len(rest) == 0:
			span = [2]int{valStart, valStart}
		case rest[0] != '\'':
			end := 0
			for ; end < len(rest); end++ {
				if isASCIISpace(rest[end]) {
					break
				}
				if rest[end] == '\\' {
					end++
					if end == len(rest) {
						return maskDSNFailClosed(dsn)
					}
				}
			}
			span = [2]int{valStart, valStart + end}
			if end < len(rest) {
				rest = rest[end+1:]
			} else {
				rest = ""
			}
		default:
			inner := rest[1:]
			end := 0
			for ; end < len(inner); end++ {
				if inner[end] == '\'' {
					break
				}
				if inner[end] == '\\' {
					end++
				}
			}
			if end >= len(inner) {
				return maskDSNFailClosed(dsn)
			}
			span = [2]int{valStart, valStart + end + 2}
			rest = inner[end+1:]
		}
		if key == "" {
			return maskDSNFailClosed(dsn)
		}
		if strings.HasSuffix(strings.ToLower(key), "password") {
			spans = append(spans, span)
		}
	}
	var b strings.Builder
	prev := 0
	for _, sp := range spans {
		b.WriteString(dsn[prev:sp[0]])
		b.WriteString("***")
		prev = sp[1]
	}
	b.WriteString(dsn[prev:])
	return b.String()
}

// maskDSNFailClosed handles input the keyword tokenizer cannot parse: if it
// still mentions password (any case) followed by '=', everything from that
// occurrence's value on is masked. Without a '=' there is no value to hide,
// so the input is returned unchanged — a bare word like "password_hash" or a
// URL path mentioning "password" must not be eaten.
func maskDSNFailClosed(dsn string) string {
	i := indexOfPassword(dsn)
	if i < 0 {
		return dsn
	}
	eq := strings.IndexByte(dsn[i:], '=')
	if eq < 0 {
		return dsn
	}
	return dsn[:i+eq+1] + "***"
}

// indexOfPassword locates the first case-insensitive "password" in s. It
// compares fixed byte windows instead of lowering the whole string, because
// strings.ToLower rewrites invalid UTF-8 as U+FFFD and can return a longer
// string, which would leave an index that no longer applies to s.
func indexOfPassword(s string) int {
	const needle = "password"
	for i := 0; i+len(needle) <= len(s); i++ {
		if strings.EqualFold(s[i:i+len(needle)], needle) {
			return i
		}
	}
	return -1
}

func maskPassword(dsn string) string {
	return maskDSNUserinfo(MaskDSNPasswords(dsn))
}
