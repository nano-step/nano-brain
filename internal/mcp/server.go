// Package mcp implements the Model Context Protocol server.
package mcp

import (
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// KeepAliveInterval is the period between MCP-level JSON-RPC pings sent to
// connected clients. Prevents proxy/load-balancer idle-connection timeouts.
const KeepAliveInterval = 30 * time.Second

func NewMCPServer(version string) *mcpsdk.Server {
	return mcpsdk.NewServer(
		&mcpsdk.Implementation{
			Name:    "nano-brain",
			Version: version,
		},
		&mcpsdk.ServerOptions{
			KeepAlive: KeepAliveInterval,
		},
	)
}
