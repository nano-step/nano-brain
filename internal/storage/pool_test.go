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
	}

	for _, tc := range cases {
		got := maskPassword(tc.in)
		if got != tc.want {
			t.Errorf("maskPassword(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
