package mcp

import (
	"encoding/json"
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
		{"top-level array", text(`[{"name":"a"},{"name":"b"},{"name":"c"}]`), n(3)},
		{"empty top-level array", text(`[]`), n(0)},
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

func TestSanitizeArgs(t *testing.T) {
	long := strings.Repeat("é", telemetryArgMaxLen) // 2 bytes per rune
	args := sanitizeArgs(map[string]any{
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
	if sanitizeArgs(nil) != nil {
		t.Error("nil args must stay nil")
	}
}

func TestSanitizeArgsNestedAndKeys(t *testing.T) {
	long := strings.Repeat("k", telemetryArgMaxLen+10)
	nested := map[string]any{}
	for i := 0; i < telemetryMaxDepth+3; i++ {
		nested = map[string]any{"child": nested, "s": long}
	}
	bigArr := make([]any, telemetryMaxElems+10)
	for i := range bigArr {
		bigArr[i] = i
	}
	args := sanitizeArgs(map[string]any{
		"metadata": map[string]any{long: long},
		"deep":     nested,
		"arr":      bigArr,
	})

	meta := args["metadata"].(map[string]any)
	if len(meta) != 1 {
		t.Fatalf("metadata keys = %d, want 1", len(meta))
	}
	for k, v := range meta {
		if len(k) > telemetryArgMaxLen+len("…") || len(v.(string)) > telemetryArgMaxLen+len("…") {
			t.Errorf("metadata key/value not capped: key %d bytes, value %d bytes", len(k), len(v.(string)))
		}
	}
	// Depth cap: walking "child" must terminate in the "…" marker.
	cur := args["deep"]
	for i := 0; i < telemetryMaxDepth+2; i++ {
		m, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("depth %d: expected map or marker, got %T", i, cur)
		}
		cur = m["child"]
		if s, ok := cur.(string); ok {
			if s != "…" {
				t.Fatalf("depth cap produced %q, want marker", s)
			}
			break
		}
	}
	if arr := args["arr"].([]any); len(arr) != telemetryMaxElems {
		t.Errorf("array len = %d, want %d", len(arr), telemetryMaxElems)
	}
}

func TestToolTelemetryPayloadSizeCap(t *testing.T) {
	// Many medium strings stay under the per-string cap but exceed the total
	// args budget; the payload must then drop args entirely.
	args := map[string]any{}
	for i := 0; i < 40; i++ {
		args[string(rune('a'+i%26))+string(rune('a'+i/26))+"-"+strings.Repeat("x", 400)] = strings.Repeat("y", 400)
	}
	payload := toolTelemetryPayload{Tool: "memory_write", Args: sanitizeArgs(args)}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) <= telemetryArgsMaxBytes {
		t.Fatalf("test setup: sanitized payload is %d bytes, want > %d", len(raw), telemetryArgsMaxBytes)
	}
	payload.Args = nil
	payload.ArgsOmitted = true
	raw, err = json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > telemetryArgsMaxBytes {
		t.Errorf("payload after dropping args is %d bytes, want <= %d", len(raw), telemetryArgsMaxBytes)
	}
}
