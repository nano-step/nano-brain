package mcp

import (
	"context"
	"encoding/json"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/sqlc-dev/pqtype"
)

// telemetryArgMaxLen caps each string argument stored in a telemetry row, so
// a memory_write does not duplicate its whole document into telemetry_logs.
const telemetryArgMaxLen = 500

// toolTelemetryPayload is the telemetry_logs.payload of an 'mcp_tool' row.
type toolTelemetryPayload struct {
	Tool    string         `json:"tool"`
	IsError bool           `json:"is_error"`
	Args    map[string]any `json:"args,omitempty"`
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
		go a.recordToolCall(params, defaultWorkspace, toolResult, err != nil, latency)
		return result, err
	}
}

func (a *Adapter) recordToolCall(params *mcpsdk.CallToolParamsRaw, defaultWorkspace string, result *mcpsdk.CallToolResult, failed bool, latency time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	args, _ := parseArgs(params.Arguments)
	workspace := argString(args, "workspace")
	if workspace == "" {
		workspace = defaultWorkspace
	}
	if workspace != "" && workspace != "all" {
		// An unresolvable workspace is kept as given: the call that used it
		// failed, and the raw value is what explains why.
		if hash, err := storage.ResolveWorkspaceParam(ctx, a.queries, workspace); err == nil {
			workspace = hash
		}
	}

	payload := toolTelemetryPayload{Tool: params.Name, IsError: failed || (result != nil && result.IsError)}
	if !payload.IsError {
		payload.ResultCount = toolResultCount(result)
	}
	payload.Args = truncateArgs(args)
	raw, err := json.Marshal(payload)
	if err != nil {
		a.logger.Warn().Err(err).Str("tool", params.Name).Msg("failed to encode tool telemetry")
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
		a.logger.Warn().Err(err).Str("tool", params.Name).Msg("failed to record tool telemetry")
	}
}

// truncateArgs caps every top-level string argument at telemetryArgMaxLen
// bytes, cutting on a rune boundary.
func truncateArgs(args map[string]any) map[string]any {
	for k, v := range args {
		s, ok := v.(string)
		if !ok || len(s) <= telemetryArgMaxLen {
			continue
		}
		cut := telemetryArgMaxLen
		for cut > 0 && s[cut]&0xC0 == 0x80 {
			cut--
		}
		args[k] = s[:cut] + "…"
	}
	return args
}

// toolResultCount derives how many results a tool returned from its JSON
// text response: the length of "results", else the "count" field, else 0
// when "found" is false. It returns nil when the response has none of these.
func toolResultCount(result *mcpsdk.CallToolResult) *int {
	if result == nil || len(result.Content) == 0 {
		return nil
	}
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	if !ok {
		return nil
	}
	var body struct {
		Results *[]json.RawMessage `json:"results"`
		Count   *int               `json:"count"`
		Found   *bool              `json:"found"`
	}
	if json.Unmarshal([]byte(text.Text), &body) != nil {
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
