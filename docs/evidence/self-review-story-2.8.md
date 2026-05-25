# Story 2.8 Self-Review Evidence

## Oracle Review
- **0 Critical**
- **2 Major**: Stub 404 exit code 0→2 (fixed), missing command-level tests (fixed: 3 httptest tests added)
- **4 Minor**: t.Setenv (fixed), dedup env resolution (fixed), --json no-op on stubs (acceptable), collection.go not using doRequest (deferred)

## Gemini Review (PR #57)
- **4 Medium**: HTTP timeout (fixed: 30s), dedup env (already fixed), collection.go doRequest (deferred), fragile 404 string match (fixed: status code return)

## Verification
- `go build ./...` ✅
- `go test -race -short ./...` ✅ (8 packages pass)
