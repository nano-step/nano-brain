package storage

import (
	"errors"
	"testing"
)

func TestRedactString_ScrubsPasswordInPostgresURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			"basic password leak",
			"failed to connect: postgres://user:secret@localhost:5432/db",
			"failed to connect: postgres://user:REDACTED@localhost:5432/db",
		},
		{
			"postgresql scheme also scrubbed",
			"err: postgresql://admin:p%40ssw0rd@host:5432/prod",
			"err: postgresql://admin:REDACTED@host:5432/prod",
		},
		{
			"no password — leave as-is",
			"postgres://user@localhost/db",
			"postgres://user@localhost/db",
		},
		{
			"empty input",
			"",
			"",
		},
		{
			"no URL — leave as-is",
			"random error message about something",
			"random error message about something",
		},
		{
			"URL inside quotes",
			`error: "postgres://u:p@h/db"`,
			`error: "postgres://u:REDACTED@h/db"`,
		},
		{
			"malformed URL fallback path — raw brace in password",
			`failed: postgres://user:p{ass}word@host/db oops`,
			`failed: postgres://user:REDACTED@host/db oops`,
		},
		{
			"malformed URL fallback path — backslash in password",
			`postgres://nb:back\\slash@host:5432/db`,
			`postgres://nb:REDACTED@host:5432/db`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RedactString(c.in)
			if got != c.want {
				t.Errorf("\n  in:   %q\n  got:  %q\n  want: %q", c.in, got, c.want)
			}
		})
	}
}

func TestRedactError_NilSafe(t *testing.T) {
	if got := RedactError(nil); got != "" {
		t.Errorf("RedactError(nil) = %q, want empty string", got)
	}
}

func TestRedactError_WrapsErrorString(t *testing.T) {
	err := errors.New("dial tcp postgres://nb:topsecret@host:5432/db: timeout")
	got := RedactError(err)
	want := "dial tcp postgres://nb:REDACTED@host:5432/db: timeout"
	if got != want {
		t.Errorf("\n  got:  %q\n  want: %q", got, want)
	}
}
