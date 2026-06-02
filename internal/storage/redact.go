package storage

import (
	"net/url"
	"regexp"
)

// RedactError returns err.Error() with any embedded postgres connection string
// (postgres://user:password@host/db) replaced by postgres://user:REDACTED@host/db.
// Used to scrub passwords from CLI error messages — see issue #307.
func RedactError(err error) string {
	if err == nil {
		return ""
	}
	return RedactString(err.Error())
}

// RedactString scrubs all postgres:// and postgresql:// connection strings
// embedded anywhere in s, replacing the password portion with "REDACTED".
// Safe to call on arbitrary strings; non-URL content is returned unchanged.
//
// Regex matches the URI from scheme up to the first whitespace or quote
// boundary (postgres connection strings cannot contain raw whitespace/quotes).
func RedactString(s string) string {
	return dsnRegex.ReplaceAllStringFunc(s, func(match string) string {
		u, err := url.Parse(match)
		if err != nil || u.User == nil {
			return match
		}
		if _, hasPwd := u.User.Password(); !hasPwd {
			return match
		}
		u.User = url.UserPassword(u.User.Username(), "REDACTED")
		return u.String()
	})
}

// dsnRegex matches postgres connection strings up to a quote/whitespace
// boundary. The character class [^...] explicitly excludes characters that
// cannot legally appear in a postgres DSN per RFC 3986 / libpq syntax.
var dsnRegex = regexp.MustCompile(`postgres(?:ql)?://[^\s"'` + "`" + `]+`)


