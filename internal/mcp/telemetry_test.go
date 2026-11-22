package mcp

import (
	"strings"
	"testing"
	"unicode/utf8"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestToolResultCount(t *testing.T) {
	n := func(v int) *int { return &v }
	text := func(s string) *mcpsdk.CallToolResult {
		return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: s}}}
	}
	tests := []struct {
		name   string
		result *mcpsdk.CallToolResult
		want   *int
	}{
		{"results", text(`{"results":[{},{}],"total":9}`), n(2)},
		{"empty results", text(`{"results":[],"total":0}`), n(0)},
		{"count", text(`{"count":3,"symbols":[{},{},{}]}`), n(3)},
		{"zero count", text(`{"count":0,"edges":[]}`), n(0)},
		{"not found", text(`{"found":false,"message":"not indexed"}`), n(0)},
		{"found", text(`{"found":true,"mermaid":"graph TD"}`), nil},
		{"document", text(`{"title":"t","content":"c","tags":[]}`), nil},
		{"plain text", text("Saved"), nil},
		{"no content", &mcpsdk.CallToolResult{}, nil},
		{"nil result", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toolResultCount(tt.result)
			if (got == nil) != (tt.want == nil) || got != nil && *got != *tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTruncateArgs(t *testing.T) {
	long := strings.Repeat("é", telemetryArgMaxLen) // 2 bytes per rune
	args := truncateArgs(map[string]any{
		"query":   "short",
		"content": long,
		"limit":   float64(5),
		"tags":    []any{"a"},
	})
	if args["query"] != "short" || args["limit"] != float64(5) || len(args["tags"].([]any)) != 1 {
		t.Errorf("untouched args changed: %v", args)
	}
	got := args["content"].(string)
	body := strings.TrimSuffix(got, "…")
	if got == body || len(body) > telemetryArgMaxLen || !utf8.ValidString(got) || !strings.HasPrefix(long, body) {
		t.Errorf("content = %d bytes, valid utf8 = %v; want a valid prefix of at most %d bytes plus an ellipsis", len(got), utf8.ValidString(got), telemetryArgMaxLen)
	}
	if truncateArgs(nil) != nil {
		t.Error("nil args must stay nil")
	}
}
