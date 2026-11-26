# CLI auth smoke test

`GOFLAGS=-buildvcs=false go test -race -tags=integration ./cmd/nano-brain -run '^TestCLIAuthEndToEnd716$' -count=1`

Result: `ok github.com/nano-brain/nano-brain/cmd/nano-brain`.

Unauthenticated probe against the isolated auth middleware returned:
HTTP/1.1 401 Unauthorized

The test builds the CLI, starts an isolated HTTP test server using the real
authentication middleware, and exercises status, workspaces, init, reindex,
write, all four collection operations, and the online doctor probes. It checks
environment/config precedence, wrong-token rejection, absence of token values
in command output, and no token forwarding to a separate MCP origin. It does
not connect to a database or the running nano-brain server.
