package mcp

import (
	"net/http"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewStreamableHTTPHandler creates an http.Handler for MCP Streamable HTTP transport.
func NewStreamableHTTPHandler(server *mcpsdk.Server) http.Handler {
	return mcpsdk.NewStreamableHTTPHandler(
		func(_ *http.Request) *mcpsdk.Server {
			return server
		},
		nil,
	)
}
