// Package mcp implements the Model Context Protocol server.
package mcp

import (
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewMCPServer creates the core MCP server instance.
// Tools are NOT registered here — that's Story 5.2.
func NewMCPServer(version string) *mcpsdk.Server {
	return mcpsdk.NewServer(
		&mcpsdk.Implementation{
			Name:    "nano-brain",
			Version: version,
		},
		nil,
	)
}
