package storage

import (
	"testing"
	"time"
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
