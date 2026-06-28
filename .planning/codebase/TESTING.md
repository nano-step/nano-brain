# Testing Patterns

> Observed patterns from the nano-brain codebase. Last reviewed: 2026-06-28.

## Framework & Tooling

- **Standard `testing` package** — no testify assertions (testify only used for `require` in a few places)
- **`t.Run` subtests** for table-driven and scenario tests
- **`t.Parallel()`** used in pure unit tests
- **`t.TempDir()`** for filesystem isolation
- **`t.Setenv()`** for env var manipulation (auto-restored on cleanup)
- **`t.Cleanup()`** for resource teardown
- **`t.Helper()`** on all test helper functions
- **`httptest`** for HTTP server and request simulation

## Test Categories

| Category | Build tag | Command | Database | Purpose |
|---|---|---|---|---|
| **Unit** | (none) | `go test -race -short ./...` | None | Pure logic, no external deps |
| **Integration** | `integration` | `go test -race -tags=integration ./...` | `nanobrain_test` | DB, migrations, full pipeline |
| **E2E** | `e2e` | `go test -race -tags=e2e ./...` | `nanobrain_test` | Server + curl endpoints |
| **Capability bench** | `capbench` | `go test -v -tags=capbench -run TestCapabilityBenchmark ./...` | `nanobrain_test` | Agent-oriented search quality |

## Makefile Targets

```bash
make test              # unit tests (race, short)
make test-integration  # integration tests (race, build tag)
make test-e2e          # e2e tests (race, build tag)
make lint              # golangci-lint run
```

## Unit Test Patterns

### Table-Driven Tests

```go
func TestRedactString_ScrubsPasswordInPostgresURL(t *testing.T) {
    cases := []struct {
        name string
        in   string
        want string
    }{
        {"basic password leak", "postgres://user:secret@host/db", "postgres://user:REDACTED@host/db"},
        {"no password — leave as-is", "postgres://user@host/db", "postgres://user@host/db"},
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
```

### Inline Struct Mocks (No gomock/gomock)

```go
type mockPool struct{ pingErr error }
func (m *mockPool) Ping(_ context.Context) error { return m.pingErr }

type mockQueue struct{}
func (m *mockQueue) Depth() int      { return 0 }
func (m *mockQueue) Capacity() int   { return 0 }
func (m *mockQueue) Status() string  { return "idle" }
```

- Mocks are defined **in the test file**, not in a separate mocks package
- Each handler defines its own small interface (e.g., `HybridSearcher`, `PoolChecker`)
- Mocks implement only the methods the handler needs
- No code generation for mocks — hand-rolled inline structs

### HTTP Handler Tests

```go
e := echo.New()
req := httptest.NewRequest(http.MethodPost, "/api/v1/query", strings.NewReader(body))
req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
rec := httptest.NewRecorder()
c := e.NewContext(req, rec)
c.Set("workspace", "test-hash")  // inject workspace middleware value

err := handlers.Query(mock, zerolog.Nop())(c)
// assert err, rec.Code, parse rec.Body
```

- Handler tests use `package handlers_test` (external test package)
- Request/response JSON constructed as string literals
- Workspace injected via `c.Set("workspace", "test-hash")`
- Response decoded with `json.NewDecoder(rec.Body).Decode(&result)`

### CLI Command Tests

```go
ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}))
defer ts.Close()

t.Setenv("NANO_BRAIN_HOST", host)
t.Setenv("NANO_BRAIN_PORT", port)
```

- CLI tests mock the HTTP server with `httptest.NewServer`
- Env vars set via `t.Setenv` for host/port overrides
- Tests in `package main` (internal test package) for direct function access

## Integration Test Patterns

### Build Tag Guard

```go
//go:build integration

package storage
```

All integration test files start with `//go:build integration`.

### Database Setup

```go
pool := testutil.SetupTestDB(t)
```

- Connects to `nanobrain_test` (NOT `nanobrain_dev`)
- Default DSN: `postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_test?sslmode=disable`
- Override: `NANO_BRAIN_TEST_DATABASE_URL=<dsn>`
- Creates an **isolated schema** per test: `test_<sha256(name)[:6]>`
- Runs goose migrations automatically
- Drops schema on cleanup

### Isolation Model

```
testutil.SetupTestDB(t)
  → sha256(t.Name()) → schema "test_<hash>"
  → CREATE SCHEMA IF NOT EXISTS <schema>
  → SET search_path TO <schema>, public
  → goose.UpContext (migrations)
  → t.Cleanup: DROP SCHEMA IF EXISTS <schema> CASCADE + pool.Close()
```

- Tests never share state — each gets its own PG schema
- Parallel-safe because schemas are isolated

### Integration Test Structure

```go
func TestUpsertChunkPreservesEmbedStatus(t *testing.T) {
    pool := testutil.SetupTestDB(t)
    db := stdlib.OpenDBFromPool(pool)
    defer db.Close()

    q := sqlc.New(db)
    ctx := context.Background()

    // seed data...
    workspaceHash := uuid.New().String()[:8]
    _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{...})

    t.Run("new chunk gets pending status", func(t *testing.T) {
        chunkID, err := q.UpsertChunk(ctx, sqlc.UpsertChunkParams{...})
        // assert...
    })
}
```

## Test Helpers (`internal/testutil/`)

| Function | Purpose |
|---|---|
| `SetupTestDB(t)` | Connect, create isolated schema, migrate, register cleanup |
| `TestDSN()` | Returns integration test DSN (env override or default) |
| `NopLogger()` | Returns `zerolog.Nop()` for silent logging in tests |
| `SeedDocumentWithTimestamps(...)` | Insert test documents with custom timestamps and tags |

## Assertion Style

- **No testify assertions** — standard `t.Errorf` / `t.Fatalf` throughout
- Format: `t.Errorf("field = %v, want %v", got, want)`
- Multi-line diffs in error messages when helpful:
  ```go
  t.Errorf("\n  in:   %q\n  got:  %q\n  want: %q", in, got, want)
  ```
- `t.Fatalf` for setup failures (DB connect, migration)
- `t.Errorf` for assertion failures (continues test)

## Coverage & Quality

- **Race detector** always on: `-race` flag in all test commands
- **Short mode** for unit tests: `-short` skips long-running operations
- **No coverage thresholds enforced** — coverage is informational
- **golangci-lint** with minimal linters: `errcheck`, `govet`, `staticcheck`, `unused`
- **Lint timeout:** 5 minutes (`timeout: 5m` in `.golangci.yml`)

## CI Pipeline

```yaml
# .github/workflows/ci.yml
go build ./...
go test -race -short ./...
# Runs against ephemeral PG service container
```

## Common Patterns to Follow

1. **Always use `t.Helper()`** in test helper functions
2. **Always use `t.Parallel()`** in pure unit tests (no DB)
3. **Always use `t.TempDir()`** for file system tests (auto-cleanup)
4. **Always use `t.Setenv()`** for env var tests (auto-restore)
5. **Never hardcode paths** — use `t.TempDir()` or relative paths
6. **Never share state between tests** — isolated schemas per test
7. **Test both success and failure paths** — error messages validated with string matching
8. **Table-driven tests** for functions with multiple input variations
9. **Mock at interface boundaries** — handler tests mock services, not DB
