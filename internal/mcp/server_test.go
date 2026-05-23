package mcp_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/mcp"
)

func TestNewMCPServer(t *testing.T) {
	s := mcp.NewMCPServer("1.0.0")
	if s == nil {
		t.Fatal("NewMCPServer returned nil")
	}
}

func TestNewSSEHandler(t *testing.T) {
	s := mcp.NewMCPServer("1.0.0")
	h := mcp.NewSSEHandler(s)
	if h == nil {
		t.Fatal("NewSSEHandler returned nil")
	}
}

func TestNewStreamableHTTPHandler(t *testing.T) {
	s := mcp.NewMCPServer("1.0.0")
	h := mcp.NewStreamableHTTPHandler(s)
	if h == nil {
		t.Fatal("NewStreamableHTTPHandler returned nil")
	}
}
