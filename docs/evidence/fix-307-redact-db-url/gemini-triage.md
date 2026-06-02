# Gemini Review Triage — PR #315

## Cycle 1 — security-HIGH finding accepted

### Finding 1 security-HIGH — url.Parse fallback leaks password
**Verdict: ACCEPTED**

When a DSN contains raw special chars in the password (e.g. `{`, `}`, `\`),
url.Parse returns an error and the previous code returned the unredacted
match. The dsnRegex matched the entire URI, so this silently leaked the
password rather than redacting it.

**Fix:** Add a regex-based fallback `pwdInDSN` that runs only when url.Parse
fails. It uses simple character classes (no URL semantics required) to
extract scheme+user, then replaces ':password@' with ':REDACTED@'.

Two-layer design:
1. url.Parse for well-formed URIs (handles percent-encoded chars correctly)
2. Regex fallback for malformed URIs (catches what url.Parse rejects)

### Finding 2 MEDIUM — missing test for fallback path
**Verdict: ACCEPTED**

Added 2 test cases:
- 'malformed URL fallback path — raw brace in password' → `p{ass}word`
- 'malformed URL fallback path — backslash in password' → `back\\slash`

Both now PASS with the new fallback.

## Cycle 1 verification

- go build PASS, go vet clean
- 10/10 test cases PASS (was 8, added 2)
- No production code changes outside redact.go

Both findings real, accepted, ~10 LOC added to redact.go + 10 LOC of tests.
