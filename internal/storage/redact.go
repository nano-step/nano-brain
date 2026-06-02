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
// Two-layer redaction:
//  1. url.Parse path — handles well-formed URIs (e.g. percent-encoded passwords).
//  2. Regex-fallback — handles malformed URIs that url.Parse rejects (e.g. raw
//     '{', '}', '\\' in password). Without this fallback the secret would leak
//     unchanged. See #307 / Gemini review on PR #315.
func RedactString(s string) string {
	return dsnRegex.ReplaceAllStringFunc(s, func(match string) string {
		if u, err := url.Parse(match); err == nil {
			if u.User == nil {
				return match
			}
			if _, hasPwd := u.User.Password(); !hasPwd {
				return match
			}
			u.User = url.UserPassword(u.User.Username(), "REDACTED")
			return u.String()
		}
		return pwdInDSN.ReplaceAllString(match, "$1:REDACTED@")
	})
}

// dsnRegex matches postgres connection strings up to a quote/whitespace
// boundary. The character class [^...] explicitly excludes characters that
// cannot legally appear in a postgres DSN per RFC 3986 / libpq syntax.
var dsnRegex = regexp.MustCompile(`postgres(?:ql)?://[^\s"'` + "`" + `]+`)

// pwdInDSN extracts username and password from a postgres URI's userinfo.
// Used only as fallback when url.Parse fails. Captures: $1 = scheme + username
// up to the colon; $2 = password (discarded). Replaces with "$1:REDACTED@".
var pwdInDSN = regexp.MustCompile(`(postgres(?:ql)?://[^:/@\s]+):[^@\s]+@`)


