package evalbench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// EvalClient executes read-only capability and replay calls over MCP Streamable
// HTTP (/mcp) or REST (/api/v1/*) backed by ReadOnlyTransport.
type EvalClient struct {
	serverURL string
	http      *http.Client

	initMu    sync.Mutex
	initDone  bool
	sessionID string
	reqSeq    atomic.Int64
}

func NewEvalClient(serverURL string, httpClient *http.Client) *EvalClient {
	return &EvalClient{
		serverURL: strings.TrimRight(serverURL, "/"),
		http:      EnsureReadOnlyClient(httpClient),
	}
}

func (c *EvalClient) HTTPClient() *http.Client {
	return c.http
}

func EstimateTokens(byteCount int) int {
	if byteCount <= 0 {
		return 0
	}
	return (byteCount + 3) / 4
}

func normalizeTaskToolName(tool string) string {
	switch tool {
	case "query", "memory_query":
		return "memory_query"
	case "search", "memory_search":
		return "memory_search"
	case "vsearch", "memory_vsearch":
		return "memory_vsearch"
	case "symbols", "memory_symbols":
		return "memory_symbols"
	case "get", "memory_get":
		return "memory_get"
	case "multi_get", "multi-get", "memory_multi_get":
		return "memory_multi_get"
	case "graph", "memory_graph":
		return "memory_graph"
	case "impact", "memory_impact":
		return "memory_impact"
	case "trace", "memory_trace":
		return "memory_trace"
	case "flow", "memory_flow":
		return "memory_flow"
	case "flowchart", "memory_flowchart":
		return "memory_flowchart"
	case "wake_up", "wake-up", "memory_wake_up":
		return "memory_wake_up"
	case "tags", "memory_tags":
		return "memory_tags"
	case "status", "memory_status":
		return "memory_status"
	default:
		return tool
	}
}

func (c *EvalClient) ensureMCPInitialized(ctx context.Context) error {
	c.initMu.Lock()
	defer c.initMu.Unlock()
	if c.initDone {
		return nil
	}

	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      c.reqSeq.Add(1),
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "nano-brain-evalbench",
				"version": "1.0.0",
			},
		},
	}
	_, hdrs, status, err := c.postMCPRaw(ctx, initReq, "")
	if err != nil {
		return fmt.Errorf("mcp initialize: %w", err)
	}
	if status >= 400 {
		return fmt.Errorf("mcp initialize returned status %d", status)
	}
	c.sessionID = hdrs.Get("Mcp-Session-Id")
	if c.sessionID != "" {
		notif := map[string]any{
			"jsonrpc": "2.0",
			"method":  "notifications/initialized",
		}
		_, _, _, _ = c.postMCPRaw(ctx, notif, c.sessionID)
	}
	c.initDone = true
	return nil
}

func (c *EvalClient) postMCPRaw(ctx context.Context, rpcBody any, sessionID string) ([]byte, http.Header, int, error) {
	payload, err := json.Marshal(rpcBody)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("marshal mcp request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.serverURL+"/mcp", bytes.NewReader(payload))
	if err != nil {
		return nil, nil, 0, fmt.Errorf("create mcp request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("X-Requested-With", "nano-brain-ui")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.Header, resp.StatusCode, fmt.Errorf("read mcp response: %w", err)
	}
	return raw, resp.Header, resp.StatusCode, nil
}

// CallMCPTool invokes an MCP tool via Streamable HTTP POST /mcp (guarded by ReadOnlyTransport)
// and parses either application/json or text/event-stream (SSE) responses.
func (c *EvalClient) CallMCPTool(ctx context.Context, toolName string, args map[string]any) (CallResult, error) {
	if AllowedMCPTools[toolName] {
		if err := c.ensureMCPInitialized(ctx); err != nil {
			return CallResult{Tool: toolName, IsError: true, ErrorMsg: err.Error()}, err
		}
	}
	if args == nil {
		args = map[string]any{}
	}
	c.initMu.Lock()
	sessID := c.sessionID
	c.initMu.Unlock()

	rpcReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      c.reqSeq.Add(1),
		"method":  "tools/call",
		"params": map[string]any{
			"name":      toolName,
			"arguments": args,
		},
	}

	start := time.Now()
	raw, hdrs, status, err := c.postMCPRaw(ctx, rpcReq, sessID)
	latencyMs := float64(time.Since(start).Microseconds()) / 1000.0
	if err != nil {
		return CallResult{
			Tool:      toolName,
			LatencyMs: latencyMs,
			IsError:   true,
			ErrorMsg:  err.Error(),
		}, err
	}
	if status >= 400 {
		errMsg := fmt.Sprintf("HTTP %d: %s", status, strings.TrimSpace(string(raw)))
		return CallResult{
			Tool:      toolName,
			LatencyMs: latencyMs,
			IsError:   true,
			ErrorMsg:  errMsg,
		}, nil
	}

	contentType := ""
	if hdrs != nil {
		contentType = hdrs.Get("Content-Type")
	}
	toolText, isErr, parseErr := unwrapMCPResponse(raw, contentType)
	if parseErr != nil {
		return CallResult{
			Tool:      toolName,
			LatencyMs: latencyMs,
			IsError:   true,
			ErrorMsg:  parseErr.Error(),
		}, nil
	}

	res := ParseToolPayload(toolName, []byte(toolText))
	res.Tool = toolName
	res.LatencyMs = latencyMs
	res.ResponseBytes = len(toolText)
	res.EstimatedTokens = EstimateTokens(len(toolText))
	if isErr {
		res.IsError = true
		res.ErrorMsg = toolText
	}
	return res, nil
}

// CallTaskTool executes a capability task tool over either MCP ("mcp") or REST ("rest").
func (c *EvalClient) CallTaskTool(ctx context.Context, transport, workspace, taskTool string, input map[string]any) (CallResult, error) {
	mcpTool := normalizeTaskToolName(taskTool)
	args := buildTaskToolArgs(mcpTool, workspace, input)
	if transport == TransportREST {
		return c.callRESTTool(ctx, mcpTool, workspace, args)
	}
	return c.CallMCPTool(ctx, mcpTool, args)
}

func buildTaskToolArgs(mcpTool, workspace string, input map[string]any) map[string]any {
	args := make(map[string]any, len(input)+2)
	for k, v := range input {
		args[k] = v
	}
	if workspace != "" {
		args["workspace"] = workspace
	}

	switch mcpTool {
	case "memory_flow":
		if _, ok := args["max_depth"]; !ok {
			args["max_depth"] = 8
		}
		if _, ok := args["format"]; !ok {
			args["format"] = "json"
		}
	case "memory_impact":
		if _, ok := args["max_depth"]; !ok {
			args["max_depth"] = 3
		}
	case "memory_trace":
		if _, ok := args["max_depth"]; !ok {
			args["max_depth"] = 4
		}
	case "memory_symbols":
		if _, ok := args["limit"]; !ok {
			if mr, hasMR := args["max_results"]; hasMR {
				args["limit"] = mr
				delete(args, "max_results")
			} else {
				args["limit"] = 20
			}
		} else {
			delete(args, "max_results")
		}
	case "memory_query", "memory_search", "memory_vsearch":
		if _, ok := args["max_results"]; !ok {
			if lim, hasLim := args["limit"]; hasLim {
				args["max_results"] = lim
				delete(args, "limit")
			} else {
				args["max_results"] = 20
			}
		}
	}
	return args
}

func (c *EvalClient) callRESTTool(ctx context.Context, mcpTool, workspace string, args map[string]any) (CallResult, error) {
	method := http.MethodPost
	var endpoint string
	var body any = args

	switch mcpTool {
	case "memory_flow":
		endpoint = c.serverURL + "/api/v1/graph/flow"
	case "memory_impact":
		endpoint = c.serverURL + "/api/v1/graph/impact"
	case "memory_trace":
		endpoint = c.serverURL + "/api/v1/graph/trace"
	case "memory_graph":
		endpoint = c.serverURL + "/api/v1/graph/query"
	case "memory_flowchart":
		endpoint = c.serverURL + "/api/v1/graph/flowchart"
	case "memory_query":
		endpoint = c.serverURL + "/api/v1/query"
	case "memory_search":
		endpoint = c.serverURL + "/api/v1/search"
	case "memory_vsearch":
		endpoint = c.serverURL + "/api/v1/vsearch"
	case "memory_get":
		endpoint = c.serverURL + "/api/v1/get"
	case "memory_multi_get":
		endpoint = c.serverURL + "/api/v1/multi-get"
	case "memory_symbols":
		method = http.MethodGet
		q := url.Values{}
		q.Set("workspace", workspace)
		if query, _ := args["query"].(string); query != "" {
			q.Set("query", query)
		}
		if kind, _ := args["kind"].(string); kind != "" {
			q.Set("kind", kind)
		}
		limit := intFromAny(args["limit"], 20)
		q.Set("limit", fmt.Sprintf("%d", limit))
		endpoint = c.serverURL + "/api/v1/symbols?" + q.Encode()
		body = nil
	case "memory_wake_up":
		method = http.MethodGet
		q := url.Values{}
		q.Set("workspace", workspace)
		endpoint = c.serverURL + "/api/v1/wake-up?" + q.Encode()
		body = nil
	case "memory_tags":
		method = http.MethodGet
		q := url.Values{}
		q.Set("workspace", workspace)
		endpoint = c.serverURL + "/api/v1/tags?" + q.Encode()
		body = nil
	case "memory_status":
		method = http.MethodGet
		endpoint = c.serverURL + "/api/status"
		body = nil
	default:
		return CallResult{
			Tool:     mcpTool,
			IsError:  true,
			ErrorMsg: fmt.Sprintf("unsupported REST tool %q", mcpTool),
		}, fmt.Errorf("unsupported REST tool %q", mcpTool)
	}

	var reqBody io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return CallResult{Tool: mcpTool, IsError: true, ErrorMsg: err.Error()}, fmt.Errorf("marshal REST request: %w", err)
		}
		reqBody = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reqBody)
	if err != nil {
		return CallResult{Tool: mcpTool, IsError: true, ErrorMsg: err.Error()}, fmt.Errorf("create REST request: %w", err)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Requested-With", "nano-brain-ui")
	}

	start := time.Now()
	resp, err := c.http.Do(req)
	latencyMs := float64(time.Since(start).Microseconds()) / 1000.0
	if err != nil {
		return CallResult{Tool: mcpTool, LatencyMs: latencyMs, IsError: true, ErrorMsg: err.Error()}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return CallResult{Tool: mcpTool, LatencyMs: latencyMs, IsError: true, ErrorMsg: err.Error()}, fmt.Errorf("read REST response: %w", err)
	}
	if resp.StatusCode >= 300 {
		msg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
		return CallResult{
			Tool:            mcpTool,
			LatencyMs:       latencyMs,
			ResponseBytes:   len(raw),
			EstimatedTokens: EstimateTokens(len(raw)),
			IsError:         true,
			ErrorMsg:        msg,
		}, nil
	}

	res := ParseToolPayload(mcpTool, raw)
	res.Tool = mcpTool
	res.LatencyMs = latencyMs
	res.ResponseBytes = len(raw)
	res.EstimatedTokens = EstimateTokens(len(raw))
	return res, nil
}

// ResolveWorkspaceReadOnly resolves a workspace hash, name, or root path to a
// registered WorkspaceInfo without ever calling POST /api/v1/init.
func (c *EvalClient) ResolveWorkspaceReadOnly(ctx context.Context, transport, spec string, snapshot WorkspaceSnapshot) (WorkspaceInfo, bool, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return WorkspaceInfo{}, false, nil
	}
	if ws, ok := snapshot.Workspaces[spec]; ok {
		return ws, true, nil
	}
	for _, ws := range snapshot.Workspaces {
		if ws.Name == spec || ws.RootPath == spec {
			return ws, true, nil
		}
	}
	for _, ws := range snapshot.Workspaces {
		if strings.EqualFold(ws.Name, spec) {
			return ws, true, nil
		}
	}

	if strings.ContainsAny(spec, `/\`) || strings.HasPrefix(spec, ".") {
		if transport == TransportREST {
			return c.resolvePathREST(ctx, spec, snapshot)
		}
		return c.resolvePathMCP(ctx, spec, snapshot)
	}
	return WorkspaceInfo{}, false, nil
}

func (c *EvalClient) resolvePathMCP(ctx context.Context, pathSpec string, snapshot WorkspaceSnapshot) (WorkspaceInfo, bool, error) {
	callRes, err := c.CallMCPTool(ctx, "memory_workspaces_resolve", map[string]any{"path": pathSpec})
	if err != nil {
		return WorkspaceInfo{}, false, err
	}
	if callRes.IsError {
		return WorkspaceInfo{}, false, nil
	}
	var resolved struct {
		WorkspaceHash string `json:"workspace_hash"`
		WorkspaceName string `json:"workspace_name"`
		RootPath      string `json:"root_path"`
		Registered    bool   `json:"registered"`
		CoveredBy     *struct {
			WorkspaceHash string `json:"workspace_hash"`
			WorkspaceName string `json:"workspace_name"`
			RootPath      string `json:"root_path"`
		} `json:"covered_by"`
	}
	// Parse from raw surfaced/ordered or re-invoke helper? Keep raw text on resolve via postMCPRaw?
	// ParseToolPayload stores raw JSON in OrderedItems[0].Snippet for memory_workspaces_resolve!
	if len(callRes.OrderedItems) == 0 || json.Unmarshal([]byte(callRes.OrderedItems[0].Snippet), &resolved) != nil {
		return WorkspaceInfo{}, false, nil
	}
	if resolved.Registered {
		if ws, ok := snapshot.Workspaces[resolved.WorkspaceHash]; ok {
			return ws, true, nil
		}
		return WorkspaceInfo{
			Hash:     resolved.WorkspaceHash,
			Name:     resolved.WorkspaceName,
			RootPath: resolved.RootPath,
		}, true, nil
	}
	if resolved.CoveredBy != nil && resolved.CoveredBy.WorkspaceHash != "" {
		if ws, ok := snapshot.Workspaces[resolved.CoveredBy.WorkspaceHash]; ok {
			return ws, true, nil
		}
		return WorkspaceInfo{
			Hash:     resolved.CoveredBy.WorkspaceHash,
			Name:     resolved.CoveredBy.WorkspaceName,
			RootPath: resolved.CoveredBy.RootPath,
		}, true, nil
	}
	return WorkspaceInfo{}, false, nil
}

func (c *EvalClient) resolvePathREST(ctx context.Context, pathSpec string, snapshot WorkspaceSnapshot) (WorkspaceInfo, bool, error) {
	payload, _ := json.Marshal(map[string]string{"path": pathSpec})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.serverURL+"/api/v1/workspaces/resolve", bytes.NewReader(payload))
	if err != nil {
		return WorkspaceInfo{}, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "nano-brain-ui")
	resp, err := c.http.Do(req)
	if err != nil {
		return WorkspaceInfo{}, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return WorkspaceInfo{}, false, nil
	}
	var resolved struct {
		WorkspaceHash string `json:"workspace_hash"`
		Name          string `json:"name"`
		RootPath      string `json:"root_path"`
		Registered    bool   `json:"registered"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&resolved); err != nil || !resolved.Registered {
		return WorkspaceInfo{}, false, nil
	}
	if ws, ok := snapshot.Workspaces[resolved.WorkspaceHash]; ok {
		return ws, true, nil
	}
	return WorkspaceInfo{
		Hash:     resolved.WorkspaceHash,
		Name:     resolved.Name,
		RootPath: resolved.RootPath,
	}, true, nil
}

type mcpContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mcpCallToolResult struct {
	Content []mcpContentBlock `json:"content"`
	IsError bool              `json:"isError"`
}

type jsonRPCResponseEnvelope struct {
	Result *mcpCallToolResult `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Method string `json:"method"`
}

func unwrapMCPResponse(raw []byte, contentType string) (string, bool, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return "", false, fmt.Errorf("empty MCP response")
	}

	isSSE := strings.Contains(strings.ToLower(contentType), "text/event-stream") ||
		strings.HasPrefix(trimmed, "event:") ||
		strings.HasPrefix(trimmed, "data:") ||
		strings.HasPrefix(trimmed, ":")

	if isSSE {
		events := extractSSEDataPayloads(trimmed)
		var lastErr error
		for i := len(events) - 1; i >= 0; i-- {
			txt, isErr, ok, err := parseJSONRPCToolResult([]byte(events[i]))
			if err != nil {
				lastErr = err
				continue
			}
			if ok {
				return txt, isErr, nil
			}
		}
		if lastErr != nil {
			return "", true, lastErr
		}
		return "", true, fmt.Errorf("no JSON-RPC result found in SSE stream")
	}

	txt, isErr, ok, err := parseJSONRPCToolResult([]byte(trimmed))
	if err != nil {
		return "", true, err
	}
	if !ok {
		return "", true, fmt.Errorf("missing JSON-RPC result in MCP response")
	}
	return txt, isErr, nil
}

// UnwrapRecordedResponse extracts the inner tool text payload from a recorded
// nano-brain-observe response string (SSE, JSON-RPC envelope, CallToolResult, or raw tool JSON).
func UnwrapRecordedResponse(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	if txt, isErr, err := unwrapMCPResponse([]byte(trimmed), ""); err == nil {
		return txt, isErr
	}
	var bare mcpCallToolResult
	if json.Unmarshal([]byte(trimmed), &bare) == nil && len(bare.Content) > 0 {
		return joinContentBlocks(bare.Content), bare.IsError
	}
	return trimmed, false
}

func extractSSEDataPayloads(stream string) []string {
	var events []string
	var cur []string
	for _, rawLine := range strings.Split(stream, "\n") {
		line := strings.TrimRight(rawLine, "\r")
		if line == "" {
			if len(cur) > 0 {
				events = append(events, strings.Join(cur, "\n"))
				cur = nil
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			val := strings.TrimPrefix(line, "data:")
			if len(val) > 0 && val[0] == ' ' {
				val = val[1:]
			}
			cur = append(cur, val)
		}
	}
	if len(cur) > 0 {
		events = append(events, strings.Join(cur, "\n"))
	}
	return events
}

func parseJSONRPCToolResult(data []byte) (string, bool, bool, error) {
	var env jsonRPCResponseEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return "", false, false, fmt.Errorf("decode JSON-RPC response: %w", err)
	}
	if env.Error != nil {
		return env.Error.Message, true, true, nil
	}
	if env.Result != nil {
		return joinContentBlocks(env.Result.Content), env.Result.IsError, true, nil
	}
	if env.Method != "" {
		return "", false, false, nil
	}
	var bare mcpCallToolResult
	if json.Unmarshal(data, &bare) == nil && len(bare.Content) > 0 {
		return joinContentBlocks(bare.Content), bare.IsError, true, nil
	}
	return "", false, false, nil
}

func joinContentBlocks(blocks []mcpContentBlock) string {
	if len(blocks) == 0 {
		return ""
	}
	if len(blocks) == 1 {
		return blocks[0].Text
	}
	var sb strings.Builder
	for i, b := range blocks {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(b.Text)
	}
	return sb.String()
}

// ParseToolPayload parses raw JSON/text returned by an MCP or REST tool into
// Surfaced names/paths, ranked OrderedItems, Collection counts, and IsEmpty.
func ParseToolPayload(toolName string, raw []byte) CallResult {
	canon := normalizeTaskToolName(toolName)
	res := CallResult{
		Tool:        canon,
		Collections: make(map[string]int),
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		res.IsEmpty = true
		return res
	}

	switch canon {
	case "memory_query", "memory_search", "memory_vsearch":
		var payload struct {
			Results []struct {
				ID         string `json:"id"`
				DocumentID string `json:"document_id"`
				Title      string `json:"title"`
				SourcePath string `json:"source_path"`
				Snippet    string `json:"snippet"`
				Content    string `json:"content"`
				Collection string `json:"collection"`
			} `json:"results"`
		}
		if json.Unmarshal([]byte(trimmed), &payload) != nil {
			res.IsEmpty = true
			return res
		}
		res.IsEmpty = len(payload.Results) == 0
		for i, r := range payload.Results {
			res.Surfaced.addPath(r.SourcePath)
			res.Surfaced.addName(r.Title)
			res.Surfaced.addName(r.Snippet)
			if r.Content != "" {
				res.Surfaced.addName(r.Content)
			}
			col := normalizeCollection(r.Collection)
			res.Collections[col]++
			res.OrderedItems = append(res.OrderedItems, OrderedItem{
				Rank:       i + 1,
				ID:         firstNonEmpty(r.SourcePath, r.DocumentID, r.ID),
				Name:       r.Title,
				Path:       r.SourcePath,
				Collection: col,
				Snippet:    r.Snippet,
			})
		}

	case "memory_symbols":
		var payload struct {
			Symbols []struct {
				Name       string `json:"name"`
				Kind       string `json:"kind"`
				SourcePath string `json:"source_path"`
			} `json:"symbols"`
		}
		if json.Unmarshal([]byte(trimmed), &payload) != nil {
			res.IsEmpty = true
			return res
		}
		res.IsEmpty = len(payload.Symbols) == 0
		for i, sym := range payload.Symbols {
			res.Surfaced.addName(sym.Name)
			res.Surfaced.addPath(sym.SourcePath)
			id := sym.Name
			if sym.SourcePath != "" {
				id = sym.SourcePath + "::" + sym.Name
			}
			res.OrderedItems = append(res.OrderedItems, OrderedItem{
				Rank: i + 1,
				ID:   id,
				Name: sym.Name,
				Path: sym.SourcePath,
			})
		}

	case "memory_flow":
		var payload struct {
			Found bool `json:"found"`
			Chain []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"chain"`
			Externals []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"externals"`
		}
		if json.Unmarshal([]byte(trimmed), &payload) != nil {
			res.IsEmpty = true
			return res
		}
		rank := 0
		for _, c := range payload.Chain {
			res.Surfaced.addName(c.Name)
			file, _ := splitNodeID(c.ID)
			if file != "" {
				res.Surfaced.addPath(file)
			}
			rank++
			res.OrderedItems = append(res.OrderedItems, OrderedItem{
				Rank: rank,
				ID:   firstNonEmpty(c.ID, c.Name),
				Name: c.Name,
				Path: file,
			})
		}
		for _, e := range payload.Externals {
			res.Surfaced.addName(e.Name)
			file, _ := splitNodeID(e.ID)
			if file != "" {
				res.Surfaced.addPath(file)
			}
			rank++
			res.OrderedItems = append(res.OrderedItems, OrderedItem{
				Rank: rank,
				ID:   firstNonEmpty(e.ID, e.Name),
				Name: e.Name,
				Path: file,
			})
		}
		res.IsEmpty = !payload.Found || rank == 0

	case "memory_impact":
		var payload struct {
			Impacted []struct {
				Node string `json:"node"`
			} `json:"impacted"`
		}
		if json.Unmarshal([]byte(trimmed), &payload) != nil {
			res.IsEmpty = true
			return res
		}
		res.IsEmpty = len(payload.Impacted) == 0
		for i, n := range payload.Impacted {
			res.Surfaced.addPath(n.Node)
			file, bare := splitNodeID(n.Node)
			res.Surfaced.addName(bare)
			if file != "" {
				res.Surfaced.addPath(file)
			}
			res.OrderedItems = append(res.OrderedItems, OrderedItem{
				Rank: i + 1,
				ID:   n.Node,
				Name: bare,
				Path: firstNonEmpty(file, n.Node),
			})
		}

	case "memory_trace":
		var payload struct {
			Chain []struct {
				Node string `json:"node"`
				Name string `json:"name"`
			} `json:"chain"`
		}
		if json.Unmarshal([]byte(trimmed), &payload) != nil {
			res.IsEmpty = true
			return res
		}
		res.IsEmpty = len(payload.Chain) == 0
		for i, n := range payload.Chain {
			res.Surfaced.addPath(n.Node)
			file, bare := splitNodeID(n.Node)
			res.Surfaced.addName(bare)
			if n.Name != "" && n.Name != bare {
				res.Surfaced.addName(n.Name)
			}
			if file != "" {
				res.Surfaced.addPath(file)
			}
			res.OrderedItems = append(res.OrderedItems, OrderedItem{
				Rank: i + 1,
				ID:   n.Node,
				Name: firstNonEmpty(n.Name, bare),
				Path: firstNonEmpty(file, n.Node),
			})
		}

	case "memory_graph":
		var payload struct {
			Direction string `json:"direction"`
			Edges     []struct {
				Source string `json:"source"`
				Target string `json:"target"`
			} `json:"edges"`
		}
		if json.Unmarshal([]byte(trimmed), &payload) != nil {
			res.IsEmpty = true
			return res
		}
		res.IsEmpty = len(payload.Edges) == 0
		for i, e := range payload.Edges {
			primary := e.Target
			if payload.Direction == "in" {
				primary = e.Source
			}
			for _, candidate := range []string{e.Source, e.Target} {
				res.Surfaced.addPath(candidate)
				f, b := splitNodeID(candidate)
				res.Surfaced.addName(b)
				if f != "" {
					res.Surfaced.addPath(f)
				}
			}
			f, b := splitNodeID(primary)
			res.OrderedItems = append(res.OrderedItems, OrderedItem{
				Rank: i + 1,
				ID:   primary,
				Name: b,
				Path: firstNonEmpty(f, primary),
			})
		}

	case "memory_get":
		var payload struct {
			ID         string `json:"id"`
			Title      string `json:"title"`
			Content    string `json:"content"`
			SourcePath string `json:"source_path"`
			Collection string `json:"collection"`
		}
		if json.Unmarshal([]byte(trimmed), &payload) != nil {
			res.IsEmpty = true
			return res
		}
		res.IsEmpty = payload.ID == "" && payload.SourcePath == "" && payload.Content == ""
		if !res.IsEmpty {
			res.Surfaced.addPath(payload.SourcePath)
			res.Surfaced.addName(payload.Title)
			res.Surfaced.addName(payload.Content)
			if payload.Collection != "" {
				col := normalizeCollection(payload.Collection)
				res.Collections[col]++
			}
			res.OrderedItems = append(res.OrderedItems, OrderedItem{
				Rank:       1,
				ID:         firstNonEmpty(payload.SourcePath, payload.ID),
				Name:       payload.Title,
				Path:       payload.SourcePath,
				Collection: normalizeCollection(payload.Collection),
				Snippet:    payload.Content,
			})
		}

	case "memory_multi_get":
		var payload struct {
			Documents []struct {
				ID         string `json:"id"`
				Title      string `json:"title"`
				Content    string `json:"content"`
				SourcePath string `json:"source_path"`
				Collection string `json:"collection"`
			} `json:"documents"`
		}
		if json.Unmarshal([]byte(trimmed), &payload) != nil {
			res.IsEmpty = true
			return res
		}
		res.IsEmpty = len(payload.Documents) == 0
		for i, d := range payload.Documents {
			res.Surfaced.addPath(d.SourcePath)
			res.Surfaced.addName(d.Title)
			res.Surfaced.addName(d.Content)
			col := normalizeCollection(d.Collection)
			res.Collections[col]++
			res.OrderedItems = append(res.OrderedItems, OrderedItem{
				Rank:       i + 1,
				ID:         firstNonEmpty(d.SourcePath, d.ID),
				Name:       d.Title,
				Path:       d.SourcePath,
				Collection: col,
			})
		}

	case "memory_flowchart":
		var payload struct {
			Found bool   `json:"found"`
			Entry string `json:"entry"`
		}
		if json.Unmarshal([]byte(trimmed), &payload) != nil || !payload.Found {
			res.IsEmpty = true
			return res
		}
		f, b := splitNodeID(payload.Entry)
		res.Surfaced.addName(b)
		res.Surfaced.addPath(firstNonEmpty(f, payload.Entry))
		res.OrderedItems = append(res.OrderedItems, OrderedItem{
			Rank: 1,
			ID:   payload.Entry,
			Name: b,
			Path: firstNonEmpty(f, payload.Entry),
		})

	case "memory_wake_up":
		var payload struct {
			Summary        string `json:"summary"`
			RecentMemories []struct {
				ID      string `json:"id"`
				Title   string `json:"title"`
				Snippet string `json:"snippet"`
			} `json:"recent_memories"`
		}
		if json.Unmarshal([]byte(trimmed), &payload) != nil {
			res.IsEmpty = true
			return res
		}
		res.IsEmpty = len(payload.RecentMemories) == 0 && payload.Summary == ""
		for i, m := range payload.RecentMemories {
			res.Surfaced.addName(m.Title)
			res.Surfaced.addName(m.Snippet)
			res.OrderedItems = append(res.OrderedItems, OrderedItem{
				Rank:    i + 1,
				ID:      m.ID,
				Name:    m.Title,
				Snippet: m.Snippet,
			})
		}

	case "memory_ticket":
		res.IsEmpty = strings.HasPrefix(trimmed, "No sessions found")
		if !res.IsEmpty {
			res.OrderedItems = append(res.OrderedItems, OrderedItem{
				Rank:    1,
				ID:      "ticket_sessions",
				Snippet: trimmed,
			})
		}

	default:
		var generic struct {
			Count *int  `json:"count"`
			Found *bool `json:"found"`
		}
		if json.Unmarshal([]byte(trimmed), &generic) == nil {
			switch {
			case generic.Count != nil && *generic.Count == 0:
				res.IsEmpty = true
			case generic.Found != nil && !*generic.Found:
				res.IsEmpty = true
			}
		}
		res.OrderedItems = append(res.OrderedItems, OrderedItem{
			Rank:    1,
			ID:      canon,
			Snippet: trimmed,
		})
	}

	if len(res.Collections) == 0 {
		res.Collections = nil
	}
	return res
}

func splitNodeID(v string) (string, string) {
	idx := strings.LastIndex(v, "::")
	if idx < 0 {
		return "", v
	}
	return v[:idx], v[idx+2:]
}

func normalizeCollection(col string) string {
	switch strings.ToLower(strings.TrimSpace(col)) {
	case "code":
		return "code"
	case "memory":
		return "memory"
	case "sessions":
		return "sessions"
	default:
		return "other"
	}
}

func intFromAny(v any, fallback int) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case json.Number:
		if parsed, err := n.Int64(); err == nil {
			return int(parsed)
		}
	}
	return fallback
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
