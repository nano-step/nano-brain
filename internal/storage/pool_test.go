package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestNewPoolConfigParsing(t *testing.T) {
	t.Parallel()

	poolCfg, err := parsePoolConfig("postgres://user:pass@localhost:5432/mydb")
	if err != nil {
		t.Fatalf("parsePoolConfig returned error: %v", err)
	}

	if poolCfg.MaxConns != 10 {
		t.Errorf("MaxConns = %d, want 10", poolCfg.MaxConns)
	}
	if poolCfg.HealthCheckPeriod != 30*time.Second {
		t.Errorf("HealthCheckPeriod = %v, want 30s", poolCfg.HealthCheckPeriod)
	}
}

func TestNewPoolConfigParsingInvalidDSN(t *testing.T) {
	t.Parallel()

	_, err := parsePoolConfig("not-a-valid-dsn://??&&")
	if err == nil {
		t.Error("expected error for invalid DSN, got nil")
	}
}

func TestMaskPassword(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{
			in:   "postgres://user:secret@localhost:5432/db",
			want: "postgres://***:***@localhost:5432/db",
		},
		{
			in:   "postgres://localhost:5432/db",
			want: "postgres://localhost:5432/db",
		},
		{
			in:   "not-a-url",
			want: "not-a-url",
		},
		{
			in:   "host=localhost user=nb password=hunter2 dbname=db",
			want: "host=localhost user=nb password=*** dbname=db",
		},
		{
			in:   "host=localhost password='hun ter\\'2' dbname=db",
			want: "host=localhost password=*** dbname=db",
		},
		{
			in:   "host=localhost PASSWORD = hunter2",
			want: "host=localhost PASSWORD = ***",
		},
		{
			in:   "password=hunter2",
			want: "password=***",
		},
		{
			in:   "postgres://localhost:5432/db?user=nb&password=hunter2&sslmode=disable",
			want: "postgres://localhost:5432/db?user=nb&password=***&sslmode=disable",
		},
		{
			in:   "postgres://u@localhost:5432/d?password=FAKEPW1&sslpassword=SSLTAIL",
			want: "postgres://***:***@localhost:5432/d?password=***&sslpassword=***",
		},
		{
			in:   "host=localhost user=nb sslpassword=hunter2 dbname=db",
			want: "host=localhost user=nb sslpassword=*** dbname=db",
		},
		{
			in:   "host=localhost password=ab&cd dbname=db",
			want: "host=localhost password=*** dbname=db",
		},
		{
			in:   "host=h password=abc&key=TAILSECRET dbname=d",
			want: "host=h password=*** dbname=d",
		},
		{
			in:   "password=abc&tail=KWTAIL",
			want: "password=***",
		},
		{
			in:   "host=h password='a b&c=d' dbname=d",
			want: "host=h password=*** dbname=d",
		},
		{
			in:   `host=localhost password=ab"cd dbname=db`,
			want: "host=localhost password=*** dbname=db",
		},
		{
			in:   "host=localhost password='a b&c' dbname=db",
			want: "host=localhost password=*** dbname=db",
		},
		{
			in:   "host=localhost user=nb dbname=db",
			want: "host=localhost user=nb dbname=db",
		},
	}

	for _, tc := range cases {
		got := maskPassword(tc.in)
		if got != tc.want {
			t.Errorf("maskPassword(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestMaskPassword_PgxParity checks the masked output against what
// pgconn.ParseConfig actually loads: the password pgx reads must not survive
// in the printed form, in whole or in part.
func TestMaskPassword_PgxParity(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		dsn     string
		secrets []string
		kept    []string
	}{
		{"escaped space in unquoted value", `host=h user=u password=ESC\ TAILPW dbname=d`, []string{"TAILPW"}, []string{"dbname=d"}},
		{"no space after closing quote", `host=h user='u'password=NOSPACEPW dbname=d`, []string{"NOSPACEPW"}, nil},
		{"vertical tab separator", "host=h\vpassword=VTABPW dbname=d", []string{"VTABPW"}, []string{"dbname=d"}},
		{"encoded query key", `postgres://u@h/d?pass%77ord=ENCKEYPW&sslmode=disable`, []string{"ENCKEYPW"}, []string{"sslmode=disable"}},
		{"encoded sslpassword key", `postgres://u@h/d?sslpass%77ord=ENCSSLPW`, []string{"ENCSSLPW"}, nil},
		{"quoted value holding password=", `host=h password='my SECRET&password=x' sslmode=disable`, []string{"SECRET"}, nil},
		{"URL value holding password=", `postgres://u@h/d?password='my SECRET&password=x'`, []string{"SECRET"}, nil},
		{"URL value with raw space", `postgres://u@h/d?password=ab TAILSP`, []string{"TAILSP"}, nil},
		{"two password params", `postgres://u@localhost:5432/d?password=FAKEPW1&sslpassword=SSLTAIL`, []string{"FAKEPW1", "SSLTAIL"}, nil},
		{"keyword ampersand tail", `host=h password=abc&key=TAILSECRET dbname=d`, []string{"TAILSECRET"}, []string{"dbname=d"}},
		{"trailing ampersand pair", `password=abc&tail=KWTAIL`, []string{"KWTAIL"}, nil},
		{"quoted escaped quote", `host=h password='a\' b' dbname=d`, []string{"a' b"}, []string{"dbname=d"}},
		{"spaces around equals", `host=h password = SPACEDPW dbname=d`, []string{"SPACEDPW"}, nil},
		{"uppercase key", `host=h PASSWORD=UPPERPW dbname=d`, []string{"UPPERPW"}, nil},
		{"malformed unterminated quote", `host=h password='unterminated SECRET`, []string{"SECRET"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := maskPassword(tc.dsn)
			if pcfg, err := pgconn.ParseConfig(tc.dsn); err == nil && pcfg.Password != "" && strings.Contains(got, pcfg.Password) {
				t.Errorf("masked output still contains the password pgx loads (%q): %q", pcfg.Password, got)
			}
			for _, s := range tc.secrets {
				if strings.Contains(got, s) {
					t.Errorf("masked output still contains %q: %q", s, got)
				}
			}
			for _, k := range tc.kept {
				if !strings.Contains(got, k) {
					t.Errorf("masked output lost %q: %q", k, got)
				}
			}
		})
	}
}
