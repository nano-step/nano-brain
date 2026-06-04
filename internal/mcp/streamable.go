package mcp

import (
	"net/http"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewStreamableHTTPHandler creates an http.Handler for MCP Streamable HTTP transport.
// Stateless mode is enabled so that clients do not need to track session IDs.
// Each tool call is self-contained (workspace hash passed per-request), so
// server-side session state provides no benefit for nano-brain.
func NewStreamableHTTPHandler(server *mcpsdk.Server) http.Handler {
	return mcpsdk.NewStreamableHTTPHandler(
		func(_ *http.Request) *mcpsdk.Server {
			return server
		},
		&mcpsdk.StreamableHTTPOptions{Stateless: true},
	)
}
