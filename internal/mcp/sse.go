package mcp

import (
	"net/http"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewSSEHandler creates an http.Handler for MCP SSE transport.
func NewSSEHandler(server *mcpsdk.Server) http.Handler {
	return mcpsdk.NewSSEHandler(
		func(_ *http.Request) *mcpsdk.Server {
			return server
		},
		nil,
	)
}
