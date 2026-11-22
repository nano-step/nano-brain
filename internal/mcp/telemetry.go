package mcp

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/sqlc-dev/pqtype"
)

const (
	// telemetryArgMaxLen caps every string stored in a telemetry row —
	// argument values, object keys, and nested strings alike — so a
	// memory_write does not duplicate its document into telemetry_logs.
	telemetryArgMaxLen = 500
	// telemetryMaxDepth bounds nesting when sanitizing arguments.
	telemetryMaxDepth = 6
	// telemetryMaxElems caps array length and object key count per level.
	telemetryMaxElems = 50
	// telemetryArgsMaxBytes drops the whole args object when its sanitized
	// JSON still exceeds this size.
	telemetryArgsMaxBytes = 8 * 1024
	// telemetryToolMaxLen caps the stored tool name (the SDK limits names to
	// 128 chars, but params.Name is client-controlled).
	telemetryToolMaxLen = 128
	// telemetryMaxPending bounds concurrent telemetry writes so a burst of
	// tool calls cannot pile up goroutines behind the shared DB pool.
	telemetryMaxPending = 64
)

// toolTelemetrySlots is the semaphore bounding in-flight telemetry writes.
var toolTelemetrySlots = make(chan struct{}, telemetryMaxPending)

// workspaceIndependentTools are tools whose result does not depend on a
// workspace (global listings, cross-workspace ticket lookup). A connection
// default workspace must not be attributed to them.
var workspaceIndependentTools = map[string]bool{
	"memory_status":             true,
	"memory_workspaces_list":    true,
	"memory_workspaces_resolve": true,
	"memory_ticket":             true,
}

// toolTelemetryPayload is the telemetry_logs.payload of an 'mcp_tool' row.
type toolTelemetryPayload struct {
	Tool    string         `json:"tool"`
	IsError bool           `json:"is_error"`
	Args    map[string]any `json:"args,omitempty"`
	// ArgsOmitted is set when args exceeded telemetryArgsMaxBytes even after
	// sanitization and were dropped entirely.
	ArgsOmitted bool `json:"args_omitted,omitempty"`
	// ResultCount is absent for tools whose response carries no result list,
	// which distinguishes "not countable" from a counted 0.
	ResultCount *int `json:"result_count,omitempty"`
}

// telemetryMiddleware records one telemetry_logs row per tools/call. The row
// is written from a goroutine after the handler returns, so it adds no
// latency to the call and a failed write never fails the call.
func (a *Adapter) telemetryMiddleware(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
	return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		params, ok := req.GetParams().(*mcpsdk.CallToolParamsRaw)
		if !ok {
			return next(ctx, method, req)
		}
		start := time.Now()
		result, err := next(ctx, method, req)
		latency := time.Since(start)
		defaultWorkspace, _ := ctx.Value(ctxKeyDefaultWorkspace{}).(string)
		toolResult, _ := result.(*mcpsdk.CallToolResult)
		select {
		case toolTelemetrySlots <- struct{}{}:
			go func() {
				defer func() { <-toolTelemetrySlots }()
				a.recordToolCall(params, defaultWorkspace, toolResult, err != nil, latency)
			}()
		default:
			a.logger.Debug().Str("tool", capString(params.Name, telemetryToolMaxLen)).Msg("tool telemetry dropped: recorder busy")
		}
		return result, err
	}
}

func (a *Adapter) recordToolCall(params *mcpsdk.CallToolParamsRaw, defaultWorkspace string, result *mcpsdk.CallToolResult, failed bool, latency time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rawArgs, _ := parseArgs(params.Arguments)
	args := sanitizeArgs(rawArgs)

	// workspace_hash only ever holds a resolved hash, "all", or "" — the raw
	// workspace argument stays visible (truncated) in payload.args.
	toolName := capString(params.Name, telemetryToolMaxLen)
	workspace := ""
	if !workspaceIndependentTools[params.Name] {
		workspace = argString(args, "workspace")
		if workspace == "" {
			workspace = defaultWorkspace
		}
	}
	if workspace != "" && workspace != "all" {
		if hash, err := storage.ResolveWorkspaceParam(ctx, a.queries, workspace); err == nil {
			workspace = hash
		} else {
			workspace = ""
		}
	}

	payload := toolTelemetryPayload{Tool: toolName, IsError: failed || (result != nil && result.IsError)}
	if !payload.IsError {
		payload.ResultCount = toolResultCount(result)
	}
	payload.Args = args
	raw, err := json.Marshal(payload)
	if err == nil && len(raw) > telemetryArgsMaxBytes {
		payload.Args = nil
		payload.ArgsOmitted = true
		raw, err = json.Marshal(payload)
	}
	if err != nil {
		a.logger.Warn().Err(err).Str("tool", toolName).Msg("failed to encode tool telemetry")
		return
	}
	row := sqlc.InsertToolTelemetryParams{
		WorkspaceHash: workspace,
		QueryText:     argString(payload.Args, "query"),
		LatencyMs:     int32(latency.Milliseconds()),
		Payload:       pqtype.NullRawMessage{RawMessage: raw, Valid: true},
	}
	if payload.ResultCount != nil {
		row.ResultCount = int32(*payload.ResultCount)
	}
	if err := a.queries.InsertToolTelemetry(ctx, row); err != nil {
		a.logger.Warn().Err(err).Str("tool", toolName).Msg("failed to record tool telemetry")
	}
}

// sanitizeArgs returns a bounded deep copy of args safe to store in
// telemetry: every string (values and object keys, at any depth) is capped
// at telemetryArgMaxLen bytes, nesting stops at telemetryMaxDepth, and each
// level keeps at most telemetryMaxElems entries. Keys are visited in sorted
// order so truncation is deterministic.
func sanitizeArgs(args map[string]any) map[string]any {
	if args == nil {
		return nil
	}
	out := make(map[string]any, len(args))
	for _, k := range sortedKeys(args) {
		out[capString(k, telemetryArgMaxLen)] = sanitizeValue(args[k], 1)
	}
	return out
}

func sanitizeValue(v any, depth int) any {
	switch t := v.(type) {
	case string:
		return capString(t, telemetryArgMaxLen)
	case map[string]any:
		if depth >= telemetryMaxDepth {
			return "…"
		}
		out := make(map[string]any, len(t))
		for _, k := range sortedKeys(t) {
			out[capString(k, telemetryArgMaxLen)] = sanitizeValue(t[k], depth+1)
		}
		return out
	case []any:
		if depth >= telemetryMaxDepth {
			return "…"
		}
		if len(t) > telemetryMaxElems {
			t = t[:telemetryMaxElems]
		}
		out := make([]any, 0, len(t))
		for _, e := range t {
			out = append(out, sanitizeValue(e, depth+1))
		}
		return out
	default:
		return v
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// capString truncates s to max bytes on a rune boundary, marking the cut
// with an ellipsis.
func capString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && s[cut]&0xC0 == 0x80 {
		cut--
	}
	return s[:cut] + "…"
}

// toolResultCount derives how many results a tool returned from its JSON
// text response: the length of a top-level array, else the length of
// "results", else the "count" field, else 0 when "found" is false. It returns
// nil when the response has none of these.
func toolResultCount(result *mcpsdk.CallToolResult) *int {
	if result == nil || len(result.Content) == 0 {
		return nil
	}
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	if !ok {
		return nil
	}
	trimmed := strings.TrimSpace(text.Text)
	if strings.HasPrefix(trimmed, "[") {
		var arr []json.RawMessage
		if json.Unmarshal([]byte(trimmed), &arr) == nil {
			n := len(arr)
			return &n
		}
		return nil
	}
	var body struct {
		Results *[]json.RawMessage `json:"results"`
		Count   *int               `json:"count"`
		Found   *bool              `json:"found"`
	}
	if json.Unmarshal([]byte(trimmed), &body) != nil {
		return nil
	}
	switch {
	case body.Results != nil:
		n := len(*body.Results)
		return &n
	case body.Count != nil:
		return body.Count
	case body.Found != nil && !*body.Found:
		zero := 0
		return &zero
	}
	return nil
}
