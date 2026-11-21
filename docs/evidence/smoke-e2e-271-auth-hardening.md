# smoke:e2e — #271 auth hardening

Change type: `bug-fix` (security) · Lane: high-risk (hard gate: auth; the issue label `lane:tiny` predates that rule) · Branch: `fix/271-auth-hardening` · Date: 2026-11-21

Server built from this branch, started on **:3199** against **`nanobrain_test`**, bound to `127.0.0.1` only (`lsof`), stopped by exact PID. Never `:3100`, never `nanobrain_dev`.

Config: auth enabled, one Basic user `admin` (bcrypt cost 10), `NANO_BRAIN_AUTH_TOKENS='tok1, tok2'`.

## 1. Env tokens are comma-separated

```
no header                          -> 401
Bearer tok1                        -> 200
Bearer tok2                        -> 200
Bearer "tok1, tok2" (the old single-token reading) -> 401
Bearer nope                        -> 401
```

## 2. Unknown username costs the same as a wrong password

`time_total` from curl, three runs each, `GET /api/status`:

```
admin:secret   -> 200  0.124s
admin:wrong    -> 401  0.084s  0.086s  0.111s
nobody:wrong   -> 401  0.084s  0.125s  0.109s
```

Before the fix the unknown-user path skipped bcrypt: with the production change reverted, `validateBasic` took 916ns for an unknown user against 16.3ms for a wrong password. `TestValidateBasic_AlwaysOneBcryptComparison` pins this by counting comparisons rather than timing them; an earlier wall-clock version of the test was flaky under `-race` and was replaced after review.

## 3. Realm is validated at startup

```
NANO_BRAIN_AUTH_REALM='evil" x="y' nano-brain serve
failed to load config: configuration validation failed:
  - server.auth.realm "evil\" x=\"y" is not valid: use only letters, digits, spaces, '.', '_' and '-'
```

Default realm still renders `Www-Authenticate: Basic realm="nano-brain"`.

## 4. Tests fail without the fix

With `internal/server/middleware/auth.go` and `internal/config/config.go` restored to `origin/master` and the tests kept:

```
--- FAIL: TestValidateBasic_AlwaysOneBcryptComparison (unknown_user)
--- FAIL: TestAuthConfig_TokensFromEnvAreCommaSeparated (two_tokens, padding_and_empty_entries_dropped)
--- FAIL: TestAuthConfig_BlankTokensEnvIsNotACredential ("", " ", ",", " , ")
--- FAIL: TestAuthConfig_EmptyTokenRejected
--- FAIL: TestAuthConfig_RealmValidation (double_quote, newline, carriage_return, backslash, empty)
```

## 5. Validation

```
go build ./... && go vet ./...     clean
go test -race -short ./...         32 packages ok, 0 fail
```
