package mcp

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nano-brain/nano-brain/internal/chunk"
	"github.com/nano-brain/nano-brain/internal/flow"
	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/search"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/symbol"
	pgvector_go "github.com/pgvector/pgvector-go"
	"github.com/sqlc-dev/pqtype"
)

func RegisterTools(server *mcpsdk.Server, a *Adapter) {
	registerMemoryQuery(server, a)
	registerMemorySearch(server, a)
	registerMemoryVSearch(server, a)
	registerMemoryGet(server, a)
	registerMemoryWrite(server, a)
	registerMemoryTags(server, a)
	registerMemoryStatus(server, a)
	registerMemoryUpdate(server, a)
	registerMemoryWakeUp(server, a)
	registerMemorySymbols(server, a)
	registerMemoryGraph(server, a)
	registerMemoryImpact(server, a)
	registerMemoryTrace(server, a)
	registerMemoryFlow(server, a)
	registerMemoryFlowchart(server, a)
	registerMemoryWorkspacesResolve(server, a)
	registerMemoryWorkspacesList(server, a)
	registerMemoryTicket(server, a)
}

func toolSchema(props map[string]map[string]any, required []string) json.RawMessage {
	schema := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	b, _ := json.Marshal(schema)
	return b
}

func textResult(v any) (*mcpsdk.CallToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return errResult(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(b)}},
	}, nil
}

func errResult(msg string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: msg}},
		IsError: true,
	}
}

func argString(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func argInt(args map[string]any, key string, def, max int) int {
	v, ok := args[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case float64:
		i := int(n)
		if i <= 0 {
			return def
		}
		if i > max {
			return max
		}
		return i
	case json.Number:
		i64, _ := n.Int64()
		i := int(i64)
		if i <= 0 {
			return def
		}
		if i > max {
			return max
		}
		return i
	default:
		return def
	}
}

func argStringSlice(args map[string]any, key string) []string {
	raw, ok := args[key]
	if !ok {
		return nil
	}
	slice, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(slice))
	for _, v := range slice {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func argBool(args map[string]any, key string) bool {
	if v, ok := args[key].(bool); ok {
		return v
	}
	return false
}

func parseArgs(raw json.RawMessage) (map[string]any, error) {
	var args map[string]any
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	return args, nil
}

// requireWorkspace resolves the workspace to use for a tool call. It reads
// the explicit "workspace" argument first (always wins when present); when
// omitted, it falls back to the per-connection default injected into ctx by
// WrapStreamableHandler (see streamable.go). When neither is present, it
// returns the same "workspace is required" error as before this fallback was
// added.
func (a *Adapter) requireWorkspace(ctx context.Context, args map[string]any) (string, *mcpsdk.CallToolResult) {
	input := argString(args, "workspace")
	if input == "" {
		if v, ok := ctx.Value(ctxKeyDefaultWorkspace{}).(string); ok && v != "" {
			input = v
		}
	}
	if input == "" {
		return "", errResult("workspace is required")
	}
	if input == "all" {
		return "all", nil
	}
	hash, err := storage.ResolveWorkspaceParam(ctx, a.queries, input)
	if err != nil {
		return "", errResult(err.Error())
	}
	return hash, nil
}

// requireRegisteredWorkspace extends requireWorkspace with a registration check
// against the workspaces table. Use in write tool handlers (memory_write,
// memory_update) — MCP transport bypasses HTTP middleware so registration
// enforcement must happen inside each write tool (issue #238). Rejects the
// literal "all" since cross-workspace writes are not supported.
//
// The "all" check runs against the raw argument (not the resolved value)
// before delegating to requireWorkspace, since a connection-level default is
// never "all" (D-02) — this keeps that rejection unaffected by the fallback.
// The empty-arg check itself is NOT duplicated here: it is owned solely by
// requireWorkspace, so the context-fallback (D-05) is not shadowed for write
// tools (see RESEARCH.md Pitfall 2).
func requireRegisteredWorkspace(ctx context.Context, a *Adapter, args map[string]any) (string, *mcpsdk.CallToolResult) {
	if argString(args, "workspace") == "all" {
		return "", errResult("workspace_all_not_supported: this tool does not accept the 'all' workspace scope; provide a specific registered workspace name or hash")
	}
	ws, errRes := a.requireWorkspace(ctx, args)
	if errRes != nil {
		return "", errRes
	}
	// For full-hash inputs, requireWorkspace returns the hash without a DB check.
	// Verify registration here to enforce the write-path constraint (issue #238).
	if _, err := a.queries.GetWorkspaceByHash(ctx, ws); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errResult(fmt.Sprintf("workspace_not_registered: workspace %q is not registered; use POST /api/v1/init to register it first", ws))
		}
		return "", errResult(fmt.Sprintf("workspace_lookup_failed: %v", err))
	}
	return ws, nil
}

const mcpSnippetLen = 200

// Over-fetch factor/cap applied to vector-search fetchLimit when
// group_by="document" collapses chunks to documents (#545). Vector search
// ranks and fetches CHUNKS with no similarity threshold; when the top chunks
// cluster into few documents, deduplicateByDocument can yield far fewer than
// max_results distinct documents unless more chunks are fetched up front so
// dedup has enough candidates to draw from.
const (
	vsearchDedupOverFetchFactor = 5
	vsearchDedupOverFetchCap    = 200
)

// mcpSearchResultItem is the per-result payload returned by memory_query,
// memory_search, and memory_vsearch over MCP. By default content is omitted;
// agents set include_content=true (or call memory_get) to obtain full text.
type mcpSearchResultItem struct {
	ID            string      `json:"id"`
	DocumentID    string      `json:"document_id"`
	WorkspaceHash string      `json:"workspace_hash,omitempty"`
	Title         string      `json:"title"`
	Snippet       string      `json:"snippet"`
	Content       string      `json:"content,omitempty"`
	Score         float64     `json:"score"`
	Tags          []string    `json:"tags,omitempty"`
	Collection    string      `json:"collection"`
	SourcePath    string      `json:"source_path"`
	CreatedAt     interface{} `json:"created_at,omitempty"`
	UpdatedAt     interface{} `json:"updated_at,omitempty"`
	HasMore       bool        `json:"has_more,omitempty"`
}

// mcpSearchResponse is the response envelope for all three search tools.
type mcpSearchResponse struct {
	Results    []mcpSearchResultItem `json:"results"`
	Total      *int                  `json:"total,omitempty"`
	QueryMs    *int64                `json:"query_ms,omitempty"`
	NextCursor string                `json:"next_cursor,omitempty"`
}

type mcpFilteredResponse struct {
	Results    []map[string]interface{} `json:"results"`
	Total      *int                     `json:"total,omitempty"`
	QueryMs    *int64                   `json:"query_ms,omitempty"`
	NextCursor string                   `json:"next_cursor,omitempty"`
}

func parseFieldSet(fields string) map[string]bool {
	set := make(map[string]bool)
	for _, f := range strings.Split(fields, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			set[f] = true
		}
	}
	return set
}

func filterFields(item mcpSearchResultItem, fieldSet map[string]bool) map[string]interface{} {
	result := map[string]interface{}{"id": item.ID}
	if fieldSet["title"] {
		result["title"] = item.Title
	}
	if fieldSet["snippet"] {
		result["snippet"] = item.Snippet
	}
	if fieldSet["score"] {
		result["score"] = item.Score
	}
	if fieldSet["source_path"] {
		result["source_path"] = item.SourcePath
	}
	if fieldSet["tags"] && len(item.Tags) > 0 {
		result["tags"] = item.Tags
	}
	if fieldSet["created_at"] && item.CreatedAt != nil {
		result["created_at"] = item.CreatedAt
	}
	if fieldSet["updated_at"] && item.UpdatedAt != nil {
		result["updated_at"] = item.UpdatedAt
	}
	if fieldSet["collection"] {
		result["collection"] = item.Collection
	}
	if fieldSet["document_id"] {
		result["document_id"] = item.DocumentID
	}
	if fieldSet["workspace_hash"] && item.WorkspaceHash != "" {
		result["workspace_hash"] = item.WorkspaceHash
	}
	if fieldSet["content"] && item.Content != "" {
		result["content"] = item.Content
	}
	return result
}

func deduplicateByDocument(results []search.Result) []search.Result {
	seen := make(map[string]bool)
	deduped := make([]search.Result, 0, len(results))
	for _, r := range results {
		if !seen[r.DocumentID] {
			seen[r.DocumentID] = true
			deduped = append(deduped, r)
		}
	}
	return deduped
}

func registerMemoryQuery(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_query",
			Description: "DEFAULT FIRST TOOL for broad agent questions: hybrid search (BM25 + vector + RRF + recency). Use for domain/codebase understanding, past decisions, and natural-language questions. For exact errors/identifiers use memory_search; for fuzzy concepts where wording may differ use memory_vsearch. Returns 500-char snippets by default; set include_content=true or call memory_get for full text. Use group_by='document' to deduplicate and paginate via cursor.",
			InputSchema: toolSchema(map[string]map[string]any{
				"query":           {"type": "string", "description": "Natural-language task or domain question. Start here for broad codebase understanding before falling back to exact search or symbols."},
				"workspace":       {"type": "string", "description": "Workspace identifier — name (e.g. 'nano-brain') or full hash. Optional if the MCP connection was configured with a default workspace via the ?workspace= URL query param; otherwise required."},
				"max_results":     {"type": "number", "description": "Max results (default 10, max 100)"},
				"cursor":          {"type": "string", "description": "Opaque pagination cursor from a previous response's next_cursor field. Pass the same query when paginating."},
				"include_content": {"type": "boolean", "description": "Set to true to include full chunk content alongside the snippet. Defaults to false. Increases response size; prefer memory_get for fetching one full document."},
				"chunk_type":      {"type": "string", "description": "Filter by chunk type: 'raw' or 'symbol'. Omit for all."},
				"created_after":   {"type": "string", "description": "Filter to documents whose created_at is >= this value. Accepts RFC3339 timestamp or relative duration ('30d', '1w', '720h'). Negative or zero durations rejected."},
				"created_before":  {"type": "string", "description": "Filter to documents whose created_at is <= this value. Accepts RFC3339 timestamp or relative duration ('30d', '1w', '720h'). Negative or zero durations rejected."},
				"updated_after":   {"type": "string", "description": "Filter to documents whose updated_at is >= this value. Accepts RFC3339 timestamp or relative duration ('30d', '1w', '720h'). Negative or zero durations rejected."},
				"updated_before":  {"type": "string", "description": "Filter to documents whose updated_at is <= this value. Accepts RFC3339 timestamp or relative duration ('30d', '1w', '720h'). Negative or zero durations rejected."},
				"time_format":     {"type": "string", "description": "Timestamp format: 'rfc3339' (default) or 'epoch' (unix seconds, saves tokens)"},
				"fields":          {"type": "string", "description": "Comma-separated field list to return (e.g. 'id,title,snippet,source_path'). Default: all fields. 'id' is always included."},
				"group_by":        {"type": "string", "description": "Group results: 'document' returns only best chunk per document. Default: no grouping."},
				"mode":            {"type": "string", "description": "Search mode: 'debugging' runs parallel code/session/config searches with source labels. Omit for standard hybrid search."},
			}, []string{"query"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			start := time.Now()
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := a.requireWorkspace(ctx, args)
			if errRes != nil {
				return errRes, nil
			}
			query := argString(args, "query")
			if query == "" {
				return errResult("query is required"), nil
			}
			if a.searchService == nil {
				return errResult("hybrid search not available (no embedding provider)"), nil
			}
			maxResults := argInt(args, "max_results", 10, 100)
			includeContent := argBool(args, "include_content")
			timeFormat := argString(args, "time_format")
			if timeFormat == "" {
				timeFormat = "rfc3339"
			}
			fields := argString(args, "fields")

			chunkType := argString(args, "chunk_type")
			if chunkType != "" && chunkType != "raw" && chunkType != "symbol" {
				return errResult("invalid chunk_type: must be 'raw' or 'symbol'"), nil
			}

			createdAfter := argString(args, "created_after")
			createdBefore := argString(args, "created_before")
			updatedAfter := argString(args, "updated_after")
			updatedBefore := argString(args, "updated_before")

			timeRange, paramName, rawValue, timeParseErr := search.ParseTimeRangeFilter(
				time.Now().UTC(),
				createdAfter,
				createdBefore,
				updatedAfter,
				updatedBefore,
			)
			if timeParseErr != nil {
				return errResult(fmt.Sprintf("invalid %s: %v (value: %q)", paramName, timeParseErr, rawValue)), nil
			}

			if timeRange == nil {
				if hint := search.DetectTemporalIntent(query); hint != nil {
					timeRange = &search.TimeRangeFilter{
						CreatedAfter:  hint.CreatedAfter,
						CreatedBefore: hint.CreatedBefore,
					}
				}
			} else if timeRange.CreatedAfter == nil && timeRange.CreatedBefore == nil &&
				timeRange.UpdatedAfter == nil && timeRange.UpdatedBefore == nil {
				if hint := search.DetectTemporalIntent(query); hint != nil {
					timeRange.CreatedAfter = hint.CreatedAfter
					timeRange.CreatedBefore = hint.CreatedBefore
				}
			}

			cursorToken := argString(args, "cursor")
			hashInput := search.QueryHashInput{
				Query:       query,
				Tags:        nil,
				Scope:       ws,
				Collections: nil,
				TimeRange:   timeRange,
			}
			offset, cursorErr := search.VerifyCursor(cursorToken, hashInput)
			if cursorErr != nil {
				if errors.Is(cursorErr, search.ErrCursorQueryMismatch) {
					return errResult("cursor query mismatch: pass the same query when paginating"), nil
				}
				return errResult(fmt.Sprintf("invalid cursor: %v", cursorErr)), nil
			}

			fetchLimit := offset + maxResults + 1

			mode := argString(args, "mode")
			var results []search.Result
			if mode == search.DebugSearchMode {
				results, err = a.searchService.DebugSearch(ctx, query, ws, fetchLimit, timeRange, chunkType)
			} else {
				results, err = a.searchService.HybridSearch(ctx, query, ws, fetchLimit, nil, timeRange, chunkType)
			}
			if err != nil {
				return errResult(fmt.Sprintf("hybrid search failed: %v", err)), nil
			}

			groupBy := argString(args, "group_by")
			if groupBy == "" {
				groupBy = "document"
			}
			if groupBy == "document" {
				results = deduplicateByDocument(results)
			}

			total := len(results)
			hasMore := total > offset+maxResults
			pageEnd := offset + maxResults
			if pageEnd > total {
				pageEnd = total
			}
			pageStart := offset
			if pageStart > total {
				pageStart = total
			}
			page := results[pageStart:pageEnd]

			items := make([]mcpSearchResultItem, len(page))
			for i, r := range page {
				var createdAt, updatedAt interface{}
				if timeFormat == "epoch" {
					createdAt = r.CreatedAt.Unix()
					updatedAt = r.UpdatedAt.Unix()
				} else {
					createdAt = r.CreatedAt
					updatedAt = r.UpdatedAt
				}
				item := mcpSearchResultItem{
					ID: r.ID, DocumentID: r.DocumentID,
					WorkspaceHash: "", Title: r.Title,
					Snippet:    search.ExtractRelevantSnippet(r.Content, query, mcpSnippetLen),
					Score:      r.Score,
					Tags:       r.Tags,
					Collection: r.Collection, SourcePath: r.SourcePath,
					CreatedAt: createdAt, UpdatedAt: updatedAt,
					HasMore: len(r.Content) > mcpSnippetLen,
				}
				if includeContent {
					item.Content = r.Content
				}
				items[i] = item
			}

			if fields != "" {
				fieldSet := parseFieldSet(fields)
				filteredItems := make([]map[string]interface{}, len(items))
				for i, item := range items {
					filteredItems[i] = filterFields(item, fieldSet)
				}
				fresp := mcpFilteredResponse{Results: filteredItems}
				if cursorToken == "" {
					fresp.Total = &total
					qms := time.Since(start).Milliseconds()
					fresp.QueryMs = &qms
				}
				if hasMore {
					fresp.NextCursor = search.EncodeCursor(pageEnd, search.QueryHash(hashInput))
				}
				return textResult(fresp)
			}

			resp := mcpSearchResponse{Results: items}
			if cursorToken == "" {
				resp.Total = &total
				qms := time.Since(start).Milliseconds()
				resp.QueryMs = &qms
			}
			if hasMore {
				resp.NextCursor = search.EncodeCursor(pageEnd, search.QueryHash(hashInput))
			}
			return textResult(resp)
		},
	)
}

func registerMemorySearch(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_search",
			Description: "Exact keyword/BM25 search for literal text: error messages, log lines, function/class names, file names, config keys, and known identifiers. Use memory_query first for broad questions; use memory_symbols for known code symbols. Returns 500-char snippets by default; set include_content=true or call memory_get for full text. Use group_by='document' to deduplicate and paginate via cursor.",
			InputSchema: toolSchema(map[string]map[string]any{
				"query":           {"type": "string", "description": "Exact words to match, such as an error string, identifier, symbol name, file name, route, or config key."},
				"workspace":       {"type": "string", "description": "Workspace identifier — name (e.g. 'nano-brain') or full hash. Optional if the MCP connection was configured with a default workspace via the ?workspace= URL query param; otherwise required."},
				"max_results":     {"type": "number", "description": "Max results (default 10, max 100)"},
				"tags":            {"type": "array", "description": "Filter by tags", "items": map[string]any{"type": "string"}},
				"cursor":          {"type": "string", "description": "Opaque pagination cursor from a previous response's next_cursor field. Pass the same query when paginating."},
				"include_content": {"type": "boolean", "description": "Set to true to include full chunk content alongside the snippet. Defaults to false. Increases response size; prefer memory_get for fetching one full document."},
				"chunk_type":      {"type": "string", "description": "Filter by chunk type: 'raw' or 'symbol'. Omit for all."},
				"created_after":   {"type": "string", "description": "Filter to documents whose created_at is >= this value. Accepts RFC3339 timestamp or relative duration ('30d', '1w', '720h'). Negative or zero durations rejected."},
				"created_before":  {"type": "string", "description": "Filter to documents whose created_at is <= this value. Accepts RFC3339 timestamp or relative duration ('30d', '1w', '720h'). Negative or zero durations rejected."},
				"updated_after":   {"type": "string", "description": "Filter to documents whose updated_at is >= this value. Accepts RFC3339 timestamp or relative duration ('30d', '1w', '720h'). Negative or zero durations rejected."},
				"updated_before":  {"type": "string", "description": "Filter to documents whose updated_at is <= this value. Accepts RFC3339 timestamp or relative duration ('30d', '1w', '720h'). Negative or zero durations rejected."},
				"time_format":     {"type": "string", "description": "Timestamp format: 'rfc3339' (default) or 'epoch' (unix seconds, saves tokens)"},
				"fields":          {"type": "string", "description": "Comma-separated field list to return (e.g. 'id,title,snippet,source_path'). Default: all fields. 'id' is always included."},
				"group_by":        {"type": "string", "description": "Group results: 'document' returns only best chunk per document. Default: no grouping."},
				"mode":            {"type": "string", "description": "Search mode: 'debugging' runs parallel code/session/config searches with source labels. Omit for standard BM25 search."},
			}, []string{"query"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			start := time.Now()
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := a.requireWorkspace(ctx, args)
			if errRes != nil {
				return errRes, nil
			}
			query := argString(args, "query")
			if query == "" {
				return errResult("query is required"), nil
			}
			maxResults := argInt(args, "max_results", 10, 100)
			tags := argStringSlice(args, "tags")
			includeContent := argBool(args, "include_content")
			timeFormat := argString(args, "time_format")
			if timeFormat == "" {
				timeFormat = "rfc3339"
			}
			fields := argString(args, "fields")

			chunkType := argString(args, "chunk_type")
			if chunkType != "" && chunkType != "raw" && chunkType != "symbol" {
				return errResult("invalid chunk_type: must be 'raw' or 'symbol'"), nil
			}
			chunkTypeNull := sql.NullString{}
			if chunkType != "" {
				chunkTypeNull = sql.NullString{String: chunkType, Valid: true}
			}

			createdAfter := argString(args, "created_after")
			createdBefore := argString(args, "created_before")
			updatedAfter := argString(args, "updated_after")
			updatedBefore := argString(args, "updated_before")

			timeRange, paramName, rawValue, timeParseErr := search.ParseTimeRangeFilter(
				time.Now().UTC(),
				createdAfter,
				createdBefore,
				updatedAfter,
				updatedBefore,
			)
			if timeParseErr != nil {
				return errResult(fmt.Sprintf("invalid %s: %v (value: %q)", paramName, timeParseErr, rawValue)), nil
			}

			if timeRange == nil {
				if hint := search.DetectTemporalIntent(query); hint != nil {
					timeRange = &search.TimeRangeFilter{
						CreatedAfter:  hint.CreatedAfter,
						CreatedBefore: hint.CreatedBefore,
					}
				}
			} else if timeRange.CreatedAfter == nil && timeRange.CreatedBefore == nil &&
				timeRange.UpdatedAfter == nil && timeRange.UpdatedBefore == nil {
				if hint := search.DetectTemporalIntent(query); hint != nil {
					timeRange.CreatedAfter = hint.CreatedAfter
					timeRange.CreatedBefore = hint.CreatedBefore
				}
			}

			cursorToken := argString(args, "cursor")
			hashInput := search.QueryHashInput{
				Query:       query,
				Tags:        tags,
				Scope:       ws,
				Collections: nil,
				TimeRange:   timeRange,
			}
			offset, cursorErr := search.VerifyCursor(cursorToken, hashInput)
			if cursorErr != nil {
				if errors.Is(cursorErr, search.ErrCursorQueryMismatch) {
					return errResult("cursor query mismatch: pass the same query when paginating"), nil
				}
				return errResult(fmt.Sprintf("invalid cursor: %v", cursorErr)), nil
			}

			mode := argString(args, "mode")
			fetchLimit := int32(offset + maxResults + 1)

			if mode == search.DebugSearchMode {
				debugResults, debugErr := a.searchService.DebugSearch(ctx, query, ws, int(fetchLimit), timeRange, chunkType)
				if debugErr != nil {
					return errResult(fmt.Sprintf("debug search failed: %v", debugErr)), nil
				}

				groupBy := argString(args, "group_by")
				if groupBy == "" {
					groupBy = "document"
				}
				if groupBy == "document" {
					debugResults = deduplicateByDocument(debugResults)
				}

				total := len(debugResults)
				hasMore := total > offset+maxResults
				pageEnd := offset + maxResults
				if pageEnd > total {
					pageEnd = total
				}
				pageStart := offset
				if pageStart > total {
					pageStart = total
				}
				page := debugResults[pageStart:pageEnd]

				items := make([]mcpSearchResultItem, len(page))
				for i, r := range page {
					var createdAt, updatedAt interface{}
					if timeFormat == "epoch" {
						createdAt = r.CreatedAt.Unix()
						updatedAt = r.UpdatedAt.Unix()
					} else {
						createdAt = r.CreatedAt
						updatedAt = r.UpdatedAt
					}
					item := mcpSearchResultItem{
						ID: r.ID, DocumentID: r.DocumentID,
						WorkspaceHash: "", Title: r.Title,
						Snippet:    search.ExtractRelevantSnippet(r.Content, query, mcpSnippetLen),
						Score:      r.Score,
						Tags:       r.Tags,
						Collection: r.Collection, SourcePath: r.SourcePath,
						CreatedAt: createdAt, UpdatedAt: updatedAt,
						HasMore: len(r.Content) > mcpSnippetLen,
					}
					if includeContent {
						item.Content = r.Content
					}
					items[i] = item
				}

				if fields != "" {
					fieldSet := parseFieldSet(fields)
					filteredItems := make([]map[string]interface{}, len(items))
					for i, item := range items {
						filteredItems[i] = filterFields(item, fieldSet)
					}
					fresp := mcpFilteredResponse{Results: filteredItems}
					if cursorToken == "" {
						fresp.Total = &total
						qms := time.Since(start).Milliseconds()
						fresp.QueryMs = &qms
					}
					if hasMore {
						fresp.NextCursor = search.EncodeCursor(pageEnd, search.QueryHash(hashInput))
					}
					return textResult(fresp)
				}

				resp := mcpSearchResponse{Results: items}
				if cursorToken == "" {
					resp.Total = &total
					qms := time.Since(start).Milliseconds()
					resp.QueryMs = &qms
				}
				if hasMore {
					resp.NextCursor = search.EncodeCursor(pageEnd, search.QueryHash(hashInput))
				}
				return textResult(resp)
			}

			var ca, cb, ua, ub sql.NullTime
			if timeRange != nil {
				ca, cb, ua, ub = timeRange.ToSqlNullTimes()
			}

			type bm25Row struct {
				ID, DocumentID       string
				WorkspaceHash, Title string
				Content, SourcePath  string
				Collection           string
				Tags                 []string
				Score                float64
				CreatedAt, UpdatedAt time.Time
			}
			var allRows []bm25Row

			if ws == "all" {
				if len(tags) > 0 {
					rows, err := a.queries.BM25SearchAllWithTags(ctx, sqlc.BM25SearchAllWithTagsParams{
						Query: query, Tags: tags, MaxResults: fetchLimit,
						ChunkType:    chunkTypeNull,
						CreatedAfter: ca, CreatedBefore: cb, UpdatedAfter: ua, UpdatedBefore: ub,
					})
					if err != nil {
						return errResult(fmt.Sprintf("bm25 search failed: %v", err)), nil
					}
					for _, r := range rows {
						allRows = append(allRows, bm25Row{
							ID: r.ID.String(), DocumentID: r.DocumentID.String(),
							WorkspaceHash: r.WorkspaceHash, Title: r.Title,
							Content: r.Content, SourcePath: r.SourcePath,
							Collection: r.Collection, Tags: r.Tags,
							Score: r.Score, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
						})
					}
				} else {
					rows, err := a.queries.BM25SearchAll(ctx, sqlc.BM25SearchAllParams{
						Query: query, MaxResults: fetchLimit,
						ChunkType:    chunkTypeNull,
						CreatedAfter: ca, CreatedBefore: cb, UpdatedAfter: ua, UpdatedBefore: ub,
					})
					if err != nil {
						return errResult(fmt.Sprintf("bm25 search failed: %v", err)), nil
					}
					for _, r := range rows {
						allRows = append(allRows, bm25Row{
							ID: r.ID.String(), DocumentID: r.DocumentID.String(),
							WorkspaceHash: r.WorkspaceHash, Title: r.Title,
							Content: r.Content, SourcePath: r.SourcePath,
							Collection: r.Collection, Tags: r.Tags,
							Score: r.Score, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
						})
					}
				}
			} else if len(tags) > 0 {
				rows, err := a.queries.BM25SearchWithTags(ctx, sqlc.BM25SearchWithTagsParams{
					Query: query, WorkspaceHash: ws, Tags: tags, MaxResults: fetchLimit,
					ChunkType:    chunkTypeNull,
					CreatedAfter: ca, CreatedBefore: cb, UpdatedAfter: ua, UpdatedBefore: ub,
				})
				if err != nil {
					return errResult(fmt.Sprintf("bm25 search failed: %v", err)), nil
				}
				for _, r := range rows {
					allRows = append(allRows, bm25Row{
						ID: r.ID.String(), DocumentID: r.DocumentID.String(),
						WorkspaceHash: r.WorkspaceHash, Title: r.Title,
						Content: r.Content, SourcePath: r.SourcePath,
						Collection: r.Collection, Tags: r.Tags,
						Score: r.Score, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
					})
				}
			} else {
				rows, err := a.queries.BM25Search(ctx, sqlc.BM25SearchParams{
					Query: query, WorkspaceHash: ws, MaxResults: fetchLimit,
					ChunkType:    chunkTypeNull,
					CreatedAfter: ca, CreatedBefore: cb, UpdatedAfter: ua, UpdatedBefore: ub,
				})
				if err != nil {
					return errResult(fmt.Sprintf("bm25 search failed: %v", err)), nil
				}
				for _, r := range rows {
					allRows = append(allRows, bm25Row{
						ID: r.ID.String(), DocumentID: r.DocumentID.String(),
						WorkspaceHash: r.WorkspaceHash, Title: r.Title,
						Content: r.Content, SourcePath: r.SourcePath,
						Collection: r.Collection, Tags: r.Tags,
						Score: r.Score, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
					})
				}
			}

			total := len(allRows)
			hasMore := total > offset+maxResults
			pageEnd := offset + maxResults
			if pageEnd > total {
				pageEnd = total
			}
			pageStart := offset
			if pageStart > total {
				pageStart = total
			}
			page := allRows[pageStart:pageEnd]

			items := make([]mcpSearchResultItem, len(page))
			for i, r := range page {
				var createdAt, updatedAt interface{}
				if timeFormat == "epoch" {
					createdAt = r.CreatedAt.Unix()
					updatedAt = r.UpdatedAt.Unix()
				} else {
					createdAt = r.CreatedAt
					updatedAt = r.UpdatedAt
				}
				item := mcpSearchResultItem{
					ID: r.ID, DocumentID: r.DocumentID,
					WorkspaceHash: "", Title: r.Title,
					Snippet:    search.ExtractRelevantSnippet(r.Content, query, mcpSnippetLen),
					Score:      r.Score,
					Tags:       r.Tags,
					Collection: r.Collection, SourcePath: r.SourcePath,
					CreatedAt: createdAt, UpdatedAt: updatedAt,
					HasMore: len(r.Content) > mcpSnippetLen,
				}
				if includeContent {
					item.Content = r.Content
				}
				items[i] = item
			}

			if fields != "" {
				fieldSet := parseFieldSet(fields)
				filteredItems := make([]map[string]interface{}, len(items))
				for i, item := range items {
					filteredItems[i] = filterFields(item, fieldSet)
				}
				fresp := mcpFilteredResponse{Results: filteredItems}
				if cursorToken == "" {
					fresp.Total = &total
					qms := time.Since(start).Milliseconds()
					fresp.QueryMs = &qms
				}
				if hasMore {
					fresp.NextCursor = search.EncodeCursor(pageEnd, search.QueryHash(hashInput))
				}
				return textResult(fresp)
			}

			resp := mcpSearchResponse{Results: items}
			if cursorToken == "" {
				resp.Total = &total
				qms := time.Since(start).Milliseconds()
				resp.QueryMs = &qms
			}
			if hasMore {
				resp.NextCursor = search.EncodeCursor(pageEnd, search.QueryHash(hashInput))
			}
			return textResult(resp)
		},
	)
}

func registerMemoryVSearch(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_vsearch",
			Description: "Fuzzy semantic/vector search for concepts where exact words may differ. Use when memory_query or memory_search miss relevant context, or when asking for similar ideas/patterns. Not ideal for exact identifiers; use memory_search or memory_symbols instead. Returns 500-char snippets by default; set include_content=true or call memory_get for full text. Use group_by='document' to deduplicate and paginate via cursor.",
			InputSchema: toolSchema(map[string]map[string]any{
				"query":           {"type": "string", "description": "Semantic concept or behavior to find when exact terms may not appear in the source text."},
				"workspace":       {"type": "string", "description": "Workspace identifier — name (e.g. 'nano-brain') or full hash. Optional if the MCP connection was configured with a default workspace via the ?workspace= URL query param; otherwise required."},
				"max_results":     {"type": "number", "description": "Max results (default 10, max 100)"},
				"cursor":          {"type": "string", "description": "Opaque pagination cursor from a previous response's next_cursor field. Pass the same query when paginating."},
				"include_content": {"type": "boolean", "description": "Set to true to include full chunk content alongside the snippet. Defaults to false. Increases response size; prefer memory_get for fetching one full document."},
				"chunk_type":      {"type": "string", "description": "Filter by chunk type: 'raw' or 'symbol'. Omit for all."},
				"created_after":   {"type": "string", "description": "Filter to documents whose created_at is >= this value. Accepts RFC3339 timestamp or relative duration ('30d', '1w', '720h'). Negative or zero durations rejected."},
				"created_before":  {"type": "string", "description": "Filter to documents whose created_at is <= this value. Accepts RFC3339 timestamp or relative duration ('30d', '1w', '720h'). Negative or zero durations rejected."},
				"updated_after":   {"type": "string", "description": "Filter to documents whose updated_at is >= this value. Accepts RFC3339 timestamp or relative duration ('30d', '1w', '720h'). Negative or zero durations rejected."},
				"updated_before":  {"type": "string", "description": "Filter to documents whose updated_at is <= this value. Accepts RFC3339 timestamp or relative duration ('30d', '1w', '720h'). Negative or zero durations rejected."},
				"time_format":     {"type": "string", "description": "Timestamp format: 'rfc3339' (default) or 'epoch' (unix seconds, saves tokens)"},
				"fields":          {"type": "string", "description": "Comma-separated field list to return (e.g. 'id,title,snippet,source_path'). Default: all fields. 'id' is always included."},
				"group_by":        {"type": "string", "description": "Group results: 'document' returns only best chunk per document. Default: no grouping."},
			}, []string{"query"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			start := time.Now()
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := a.requireWorkspace(ctx, args)
			if errRes != nil {
				return errRes, nil
			}
			query := argString(args, "query")
			if query == "" {
				return errResult("query is required"), nil
			}
			if a.embedder == nil {
				return errResult("vector search requires embedding provider"), nil
			}
			maxResults := argInt(args, "max_results", 10, 100)
			includeContent := argBool(args, "include_content")
			timeFormat := argString(args, "time_format")
			if timeFormat == "" {
				timeFormat = "rfc3339"
			}
			fields := argString(args, "fields")

			chunkType := argString(args, "chunk_type")
			if chunkType != "" && chunkType != "raw" && chunkType != "symbol" {
				return errResult("invalid chunk_type: must be 'raw' or 'symbol'"), nil
			}
			chunkTypeNull := sql.NullString{}
			if chunkType != "" {
				chunkTypeNull = sql.NullString{String: chunkType, Valid: true}
			}

			createdAfter := argString(args, "created_after")
			createdBefore := argString(args, "created_before")
			updatedAfter := argString(args, "updated_after")
			updatedBefore := argString(args, "updated_before")

			timeRange, paramName, rawValue, timeParseErr := search.ParseTimeRangeFilter(
				time.Now().UTC(),
				createdAfter,
				createdBefore,
				updatedAfter,
				updatedBefore,
			)
			if timeParseErr != nil {
				return errResult(fmt.Sprintf("invalid %s: %v (value: %q)", paramName, timeParseErr, rawValue)), nil
			}

			if timeRange == nil {
				if hint := search.DetectTemporalIntent(query); hint != nil {
					timeRange = &search.TimeRangeFilter{
						CreatedAfter:  hint.CreatedAfter,
						CreatedBefore: hint.CreatedBefore,
					}
				}
			} else if timeRange.CreatedAfter == nil && timeRange.CreatedBefore == nil &&
				timeRange.UpdatedAfter == nil && timeRange.UpdatedBefore == nil {
				if hint := search.DetectTemporalIntent(query); hint != nil {
					timeRange.CreatedAfter = hint.CreatedAfter
					timeRange.CreatedBefore = hint.CreatedBefore
				}
			}

			cursorToken := argString(args, "cursor")
			hashInput := search.QueryHashInput{
				Query:       query,
				Tags:        nil,
				Scope:       ws,
				Collections: nil,
				TimeRange:   timeRange,
			}
			offset, cursorErr := search.VerifyCursor(cursorToken, hashInput)
			if cursorErr != nil {
				if errors.Is(cursorErr, search.ErrCursorQueryMismatch) {
					return errResult("cursor query mismatch: pass the same query when paginating"), nil
				}
				return errResult(fmt.Sprintf("invalid cursor: %v", cursorErr)), nil
			}

			embedCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			vec, err := a.embedder.Embed(embedCtx, query)
			if err != nil {
				return errResult(fmt.Sprintf("embedding query failed: %v", err)), nil
			}

			groupBy := argString(args, "group_by")
			if groupBy == "" {
				groupBy = "document"
			}

			baseFetchLimit := offset + maxResults + 1
			fetchLimit := int32(baseFetchLimit)
			if groupBy == "document" {
				// Over-fetch chunks so deduplicateByDocument has enough
				// candidates to collapse into up to maxResults distinct
				// documents (#545) — vector search has no similarity
				// threshold, so a diluted compound-query embedding can rank
				// many chunks from the same few documents at the top.
				// The cap is a ceiling on the over-fetch boost, never a
				// reduction below what the page window itself needs — at
				// deep offsets baseFetchLimit can exceed the cap, and
				// min()-ing straight into the cap would starve the page
				// (R88 review #545 follow-up).
				fetchLimit = int32(max(baseFetchLimit, min(baseFetchLimit*vsearchDedupOverFetchFactor, vsearchDedupOverFetchCap)))
			}
			var ca, cb, ua, ub sql.NullTime
			if timeRange != nil {
				ca, cb, ua, ub = timeRange.ToSqlNullTimes()
			}

			type vsRow struct {
				ID, DocumentID       string
				WorkspaceHash, Title string
				Content, SourcePath  string
				Collection           string
				Tags                 []string
				Score                float64
				CreatedAt, UpdatedAt time.Time
			}
			var allRows []vsRow

			if ws == "all" {
				rows, err := a.queries.VectorSearchAll(ctx, sqlc.VectorSearchAllParams{
					QueryEmbedding: pgvector_go.NewVector(vec),
					MaxResults:     fetchLimit,
					ChunkType:      chunkTypeNull,
					CreatedAfter:   ca, CreatedBefore: cb, UpdatedAfter: ua, UpdatedBefore: ub,
				})
				if err != nil {
					return errResult(fmt.Sprintf("vector search failed: %v", err)), nil
				}
				allRows = make([]vsRow, 0, len(rows))
				for _, r := range rows {
					allRows = append(allRows, vsRow{
						ID: r.ChunkID.String(), DocumentID: r.DocumentID.String(),
						WorkspaceHash: r.WorkspaceHash, Title: r.Title,
						Content: r.Content, SourcePath: r.SourcePath,
						Collection: r.Collection, Tags: r.Tags,
						Score: r.Score, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
					})
				}
			} else {
				rows, err := a.queries.VectorSearch(ctx, sqlc.VectorSearchParams{
					QueryEmbedding: pgvector_go.NewVector(vec),
					WorkspaceHash:  ws,
					MaxResults:     fetchLimit,
					ChunkType:      chunkTypeNull,
					CreatedAfter:   ca, CreatedBefore: cb, UpdatedAfter: ua, UpdatedBefore: ub,
				})
				if err != nil {
					return errResult(fmt.Sprintf("vector search failed: %v", err)), nil
				}
				allRows = make([]vsRow, 0, len(rows))
				for _, r := range rows {
					allRows = append(allRows, vsRow{
						ID: r.ChunkID.String(), DocumentID: r.DocumentID.String(),
						WorkspaceHash: r.WorkspaceHash, Title: r.Title,
						Content: r.Content, SourcePath: r.SourcePath,
						Collection: r.Collection, Tags: r.Tags,
						Score: r.Score, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
					})
				}
			}

			// Stable secondary sort by id ASC on tied scores. Keeps cursor
			// pagination deterministic without forcing PostgreSQL to satisfy
			// a multi-column ORDER BY through the HNSW index (#358).
			sort.SliceStable(allRows, func(i, j int) bool {
				if allRows[i].Score != allRows[j].Score {
					return allRows[i].Score > allRows[j].Score
				}
				return allRows[i].ID < allRows[j].ID
			})

			if groupBy == "document" {
				vsearchResults := make([]search.Result, len(allRows))
				for i, r := range allRows {
					vsearchResults[i] = search.Result{
						ID: r.ID, DocumentID: r.DocumentID,
						WorkspaceHash: r.WorkspaceHash, Title: r.Title,
						Content: r.Content, SourcePath: r.SourcePath,
						Collection: r.Collection, Tags: r.Tags,
						Score: r.Score, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
					}
				}
				deduped := deduplicateByDocument(vsearchResults)
				allRows = make([]vsRow, len(deduped))
				for i, r := range deduped {
					allRows[i] = vsRow{
						ID: r.ID, DocumentID: r.DocumentID,
						WorkspaceHash: r.WorkspaceHash, Title: r.Title,
						Content: r.Content, SourcePath: r.SourcePath,
						Collection: r.Collection, Tags: r.Tags,
						Score: r.Score, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
					}
				}
			}

			total := len(allRows)
			hasMore := total > offset+maxResults
			pageEnd := offset + maxResults
			if pageEnd > total {
				pageEnd = total
			}
			pageStart := offset
			if pageStart > total {
				pageStart = total
			}
			page := allRows[pageStart:pageEnd]

			items := make([]mcpSearchResultItem, len(page))
			for i, r := range page {
				var createdAt, updatedAt interface{}
				if timeFormat == "epoch" {
					createdAt = r.CreatedAt.Unix()
					updatedAt = r.UpdatedAt.Unix()
				} else {
					createdAt = r.CreatedAt
					updatedAt = r.UpdatedAt
				}
				item := mcpSearchResultItem{
					ID: r.ID, DocumentID: r.DocumentID,
					WorkspaceHash: "", Title: r.Title,
					Snippet:    search.ExtractRelevantSnippet(r.Content, query, mcpSnippetLen),
					Score:      r.Score,
					Tags:       r.Tags,
					Collection: r.Collection, SourcePath: r.SourcePath,
					CreatedAt: createdAt, UpdatedAt: updatedAt,
					HasMore: len(r.Content) > mcpSnippetLen,
				}
				if includeContent {
					item.Content = r.Content
				}
				items[i] = item
			}

			if fields != "" {
				fieldSet := parseFieldSet(fields)
				filteredItems := make([]map[string]interface{}, len(items))
				for i, item := range items {
					filteredItems[i] = filterFields(item, fieldSet)
				}
				fresp := mcpFilteredResponse{Results: filteredItems}
				if cursorToken == "" {
					fresp.Total = &total
					qms := time.Since(start).Milliseconds()
					fresp.QueryMs = &qms
				}
				if hasMore {
					fresp.NextCursor = search.EncodeCursor(pageEnd, search.QueryHash(hashInput))
				}
				return textResult(fresp)
			}

			resp := mcpSearchResponse{Results: items}
			if cursorToken == "" {
				resp.Total = &total
				qms := time.Since(start).Milliseconds()
				resp.QueryMs = &qms
			}
			if hasMore {
				resp.NextCursor = search.EncodeCursor(pageEnd, search.QueryHash(hashInput))
			}
			return textResult(resp)
		},
	)
}

// resolveDocumentByAnyID looks up a document by ID, falling back to chunk ID
// when the UUID belongs to a chunk rather than a document. Search results
// expose chunk IDs as their primary "id" field, so a bare UUID or #<uuid>
// passed to memory_get frequently refers to a chunk, not a document.
func resolveDocumentByAnyID(ctx context.Context, q *sqlc.Queries, ws string, id uuid.UUID) (sqlc.Document, error) {
	doc, err := q.GetDocumentByID(ctx, sqlc.GetDocumentByIDParams{ID: id, WorkspaceHash: ws})
	if err == nil {
		return doc, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		// A real DB/connection error — surface it rather than masking it as a
		// misleading "not found" behind the chunk fallback.
		return sqlc.Document{}, err
	}
	chunk, chunkErr := q.GetChunkByID(ctx, id)
	if chunkErr != nil {
		if errors.Is(chunkErr, sql.ErrNoRows) {
			return sqlc.Document{}, fmt.Errorf("no document or chunk found for id %s in workspace %s", id, ws)
		}
		// A real DB/connection error on the chunk lookup — surface it rather
		// than masking it as a misleading "not found".
		return sqlc.Document{}, chunkErr
	}
	if chunk.WorkspaceHash != ws {
		return sqlc.Document{}, fmt.Errorf("no document or chunk found for id %s in workspace %s", id, ws)
	}
	return q.GetDocumentByID(ctx, sqlc.GetDocumentByIDParams{ID: chunk.DocumentID, WorkspaceHash: ws})
}

func registerMemoryGet(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_get",
			Description: "Fetch full content for one known document. Use after memory_query/memory_search/memory_vsearch when a result snippet is relevant and you need exact text or line slices. Do not use for broad discovery; search first, then get one selected hit.",
			InputSchema: toolSchema(map[string]map[string]any{
				"path":       {"type": "string", "description": "Document source_path, UUID (auto-detected), #<uuid> (document or chunk id from a search result), or a \"file::Symbol\" graph node from memory_trace/memory_graph (returns the symbol's full body)"},
				"workspace":  {"type": "string", "description": "Workspace identifier — name (e.g. 'nano-brain') or full hash. Optional if the MCP connection was configured with a default workspace via the ?workspace= URL query param; otherwise required."},
				"start_line": {"type": "number", "description": "Start line (1-indexed, inclusive)"},
				"end_line":   {"type": "number", "description": "End line (1-indexed, inclusive)"},
			}, []string{"path"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := a.requireWorkspace(ctx, args)
			if errRes != nil {
				return errRes, nil
			}
			if ws == "all" {
				return errResult("workspace 'all' is not valid for memory_get"), nil
			}
			path := argString(args, "path")
			if path == "" {
				return errResult("path is required"), nil
			}

			var doc sqlc.Document
			switch {
			case strings.Contains(path, "::"):
				// A graph node like "relpath::Symbol" as emitted by
				// memory_trace / memory_graph. Resolve it to the backing symbol
				// document so the node is directly re-feedable into memory_get.
				sepIdx := strings.Index(path, "::")
				filePart, symName := path[:sepIdx], path[sepIdx+2:]
				matches, rErr := a.queries.ResolveSymbolByName(ctx, sqlc.ResolveSymbolByNameParams{
					WorkspaceHash: ws,
					Column2:       symName,
				})
				if rErr != nil {
					return errResult(fmt.Sprintf("symbol lookup failed for %q", symName)), nil
				}
				for _, m := range matches {
					if qi := strings.Index(m.SourcePath, "?"); qi >= 0 && m.SourcePath[:qi] == filePart {
						doc, err = a.queries.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
							SourcePath:    m.SourcePath,
							WorkspaceHash: ws,
						})
						break
					}
				}
				if err == nil && doc.ID == uuid.Nil {
					return errResult(fmt.Sprintf("no symbol %q found in %q in workspace %s", symName, filePart, ws)), nil
				}
			case strings.HasPrefix(path, "#"):
				// Explicit ID lookup via #<uuid> prefix. Search results expose
				// chunk IDs first, so this may be a document or a chunk ID.
				docID, parseErr := uuid.Parse(strings.TrimPrefix(path, "#"))
				if parseErr != nil {
					return errResult(fmt.Sprintf("invalid document ID: %v", parseErr)), nil
				}
				doc, err = resolveDocumentByAnyID(ctx, a.queries, ws, docID)
			default:
				// Try UUID-first: many clients pass the document_id or chunk_id
				// (UUID) returned by memory_query / memory_search directly as
				// the path.
				if docID, parseErr := uuid.Parse(path); parseErr == nil {
					doc, err = resolveDocumentByAnyID(ctx, a.queries, ws, docID)
				} else {
					doc, err = a.queries.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
						SourcePath:    path,
						WorkspaceHash: ws,
					})
				}
			}
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return errResult(fmt.Sprintf("no document found for path %q in workspace %s", path, ws)), nil
				}
				return errResult(fmt.Sprintf("document not found: %v", err)), nil
			}

			content := doc.Content
			startLine := argInt(args, "start_line", 0, 1<<30)
			endLine := argInt(args, "end_line", 0, 1<<30)
			explicitLines := startLine > 0 || endLine > 0

			// A symbol doc's own content is just its signature. Swap in the
			// parent file's content so the caller gets the full body: default to
			// the symbol's own line span from metadata, but an explicit
			// start_line/end_line from the caller still wins. Never error out
			// solely because the body couldn't be resolved — fall back to the
			// signature.
			if qIdx := strings.Index(doc.SourcePath, "?symbol="); qIdx >= 0 {
				// Resolve the symbol's own line span from metadata unless the
				// caller supplied an explicit start_line/end_line (which wins).
				symSpanFromMeta := false
				if !explicitLines && doc.Metadata.Valid {
					var meta map[string]string
					if json.Unmarshal(doc.Metadata.RawMessage, &meta) == nil {
						if symStart, sErr := strconv.Atoi(meta["line"]); sErr == nil && symStart > 0 {
							if symEnd, eErr := strconv.Atoi(meta["end_line"]); eErr == nil && symEnd > 0 {
								startLine, endLine = symStart, symEnd
								symSpanFromMeta = true
							}
						}
					}
				}
				// Pull in the parent file's body so the caller gets the full
				// definition rather than just the signature — but only when we
				// have a span to slice.
				if startLine > 0 || endLine > 0 {
					parentPath := doc.SourcePath[:qIdx]
					parentDoc, perr := a.queries.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
						SourcePath:    parentPath,
						WorkspaceHash: ws,
					})
					if perr == nil {
						content = parentDoc.Content
					} else if symSpanFromMeta {
						// Parent unavailable (e.g. a workspace indexed before
						// symbols carried line metadata): keep the signature and
						// clear the metadata span so it isn't sliced into an
						// empty string when the symbol starts past line 1.
						startLine, endLine = 0, 0
					}
				}
			}
			if startLine > 0 || endLine > 0 {
				lines := strings.Split(content, "\n")
				total := len(lines)
				s := startLine
				if s < 1 {
					s = 1
				}
				e := endLine
				if e < 1 || e > total {
					e = total
				}
				if s > total || s > e {
					content = ""
				} else {
					content = strings.Join(lines[s-1:e], "\n")
				}
			}

			supersedes := ""
			if doc.SupersedesID.Valid {
				supersedes = doc.SupersedesID.UUID.String()
			}

			return textResult(map[string]any{
				"id":             doc.ID.String(),
				"content":        content,
				"title":          doc.Title,
				"tags":           doc.Tags,
				"collection":     doc.Collection,
				"workspace_hash": doc.WorkspaceHash,
				"source_path":    doc.SourcePath,
				"supersedes_id":  supersedes,
				"created_at":     doc.CreatedAt.Format(time.RFC3339),
				"updated_at":     doc.UpdatedAt.Format(time.RFC3339),
			})
		},
	)
}

func registerMemoryWrite(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_write",
			Description: "Persist a durable decision, lesson, summary, or handoff for future agents. Use at the end of work or after important discoveries; do not use for transient scratch notes. Writes are workspace-scoped and require a registered workspace.",
			InputSchema: toolSchema(map[string]map[string]any{
				"content":     {"type": "string", "description": "Document content"},
				"workspace":   {"type": "string", "description": "Workspace identifier — name (e.g. 'nano-brain') or full hash. Optional if the MCP connection was configured with a default workspace via the ?workspace= URL query param; otherwise required."},
				"title":       {"type": "string", "description": "Document title"},
				"tags":        {"type": "array", "description": "Document tags", "items": map[string]any{"type": "string"}},
				"collection":  {"type": "string", "description": "Collection name (default: memory)"},
				"source_path": {"type": "string", "description": "Source file path"},
				"metadata":    {"type": "object", "description": "Additional metadata"},
				"supersedes":  {"type": "string", "description": "Document to supersede (#<uuid> or source_path)"},
			}, []string{"content"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := requireRegisteredWorkspace(ctx, a, args)
			if errRes != nil {
				return errRes, nil
			}
			content := argString(args, "content")
			if content == "" {
				return errResult("content is required"), nil
			}
			const maxContentSize = 5 * 1024 * 1024 // 5 MB
			if int64(len(content)) > maxContentSize {
				return errResult("content exceeds maximum allowed size (5 MB)"), nil
			}

			collection := argString(args, "collection")
			if collection == "" {
				collection = "memory"
			}
			tags := argStringSlice(args, "tags")
			if tags == nil {
				tags = []string{}
			}
			title := argString(args, "title")
			sourcePath := argString(args, "source_path")

			meta := pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true}
			if raw, ok := args["metadata"]; ok && raw != nil {
				b, err := json.Marshal(raw)
				if err == nil && len(b) > 0 {
					meta = pqtype.NullRawMessage{RawMessage: b, Valid: true}
				}
			}

			var supersedesID uuid.NullUUID
			if sup := argString(args, "supersedes"); sup != "" {
				if strings.HasPrefix(sup, "#") {
					parsed, parseErr := uuid.Parse(strings.TrimPrefix(sup, "#"))
					if parseErr == nil {
						supersedesID = uuid.NullUUID{UUID: parsed, Valid: true}
					} else {
						a.logger.Warn().Str("supersedes", sup).Err(parseErr).Msg("invalid supersedes UUID, ignoring")
					}
				} else {
					target, lookupErr := a.queries.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
						SourcePath:    sup,
						WorkspaceHash: ws,
					})
					if lookupErr == nil {
						supersedesID = uuid.NullUUID{UUID: target.ID, Valid: true}
					} else {
						a.logger.Warn().Str("supersedes", sup).Err(lookupErr).Msg("supersedes target not found, ignoring")
					}
				}
			}

			sum := sha256.Sum256([]byte(content))
			contentHash := hex.EncodeToString(sum[:])

			params := sqlc.UpsertDocumentParams{
				WorkspaceHash: ws,
				ContentHash:   contentHash,
				Title:         title,
				Content:       content,
				SourcePath:    sourcePath,
				Collection:    collection,
				Tags:          tags,
				Metadata:      meta,
				SupersedesID:  supersedesID,
			}

			chunks := chunk.Split(content, chunk.DefaultConfig())
			chunkMeta := pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true}

			var row sqlc.UpsertDocumentRow
			var chunkCount int

			if a.db != nil {
				tx, err := a.db.BeginTx(ctx, nil)
				if err != nil {
					return errResult(fmt.Sprintf("begin transaction failed: %v", err)), nil
				}
				tq := sqlc.New(tx)
				row, err = tq.UpsertDocument(ctx, params)
				if err != nil {
					_ = tx.Rollback()
					return errResult(fmt.Sprintf("upsert document failed: %v", err)), nil
				}
				if err := tq.DeleteChunksByDocumentID(ctx, sqlc.DeleteChunksByDocumentIDParams{
					DocumentID:    row.ID,
					WorkspaceHash: ws,
				}); err != nil {
					_ = tx.Rollback()
					return errResult(fmt.Sprintf("delete chunks failed: %v", err)), nil
				}
				for _, ch := range chunks {
					_, err := tq.UpsertChunk(ctx, sqlc.UpsertChunkParams{
						DocumentID:        row.ID,
						WorkspaceHash:     ws,
						ContentHash:       ch.Hash,
						Content:           ch.Content,
						ChunkIndex:        int32(ch.Sequence),
						StartLine:         sql.NullInt32{Int32: int32(ch.StartLine), Valid: true},
						EndLine:           sql.NullInt32{Int32: int32(ch.EndLine), Valid: true},
						Metadata:          chunkMeta,
						ChunkType:         "raw",
						EmbeddingStrategy: "raw_code",
					})
					if err != nil {
						_ = tx.Rollback()
						return errResult(fmt.Sprintf("upsert chunk failed: %v", err)), nil
					}
				}
				if err := tx.Commit(); err != nil {
					return errResult(fmt.Sprintf("commit failed: %v", err)), nil
				}
				chunkCount = len(chunks)
			} else {
				row, err = a.queries.UpsertDocument(ctx, params)
				if err != nil {
					return errResult(fmt.Sprintf("upsert document failed: %v", err)), nil
				}
				if err := a.queries.DeleteChunksByDocumentID(ctx, sqlc.DeleteChunksByDocumentIDParams{
					DocumentID:    row.ID,
					WorkspaceHash: ws,
				}); err != nil {
					return errResult(fmt.Sprintf("delete chunks failed: %v", err)), nil
				}
				for _, ch := range chunks {
					_, err := a.queries.UpsertChunk(ctx, sqlc.UpsertChunkParams{
						DocumentID:        row.ID,
						WorkspaceHash:     ws,
						ContentHash:       ch.Hash,
						Content:           ch.Content,
						ChunkIndex:        int32(ch.Sequence),
						StartLine:         sql.NullInt32{Int32: int32(ch.StartLine), Valid: true},
						EndLine:           sql.NullInt32{Int32: int32(ch.EndLine), Valid: true},
						Metadata:          chunkMeta,
						ChunkType:         "raw",
						EmbeddingStrategy: "raw_code",
					})
					if err != nil {
						return errResult(fmt.Sprintf("upsert chunk failed: %v", err)), nil
					}
				}
				chunkCount = len(chunks)
			}

			return textResult(map[string]any{
				"id":             row.ID.String(),
				"hash":           row.ContentHash,
				"collection":     row.Collection,
				"workspace_hash": row.WorkspaceHash,
				"chunk_count":    chunkCount,
			})
		},
	)
}

func registerMemoryTags(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_tags",
			Description: "List collections/tags in a workspace. Use to discover available memory categories before filtered searches, not for code navigation.",
			InputSchema: toolSchema(map[string]map[string]any{
				"workspace": {"type": "string", "description": "Workspace identifier — name (e.g. 'nano-brain') or full hash. Optional if the MCP connection was configured with a default workspace via the ?workspace= URL query param; otherwise required."},
			}, []string{}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := a.requireWorkspace(ctx, args)
			if errRes != nil {
				return errRes, nil
			}
			if ws == "all" {
				return errResult("cross-workspace not supported for this tool"), nil
			}

			rows, err := a.queries.ListCollectionsWithLastUpdated(ctx, ws)
			if err != nil {
				return errResult(fmt.Sprintf("list collections failed: %v", err)), nil
			}

			type collectionInfo struct {
				Name          string `json:"name"`
				DocumentCount int64  `json:"document_count"`
				LastUpdated   string `json:"last_updated"`
			}
			results := make([]collectionInfo, 0, len(rows))
			for _, r := range rows {
				lastUpdated := ""
				if t, ok := r.LastUpdated.(time.Time); ok {
					lastUpdated = t.Format(time.RFC3339)
				}
				results = append(results, collectionInfo{
					Name:          r.Name,
					DocumentCount: r.DocumentCount,
					LastUpdated:   lastUpdated,
				})
			}
			return textResult(results)
		},
	)
}

func registerMemoryStatus(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_status",
			Description: "Server, database, and embedding queue health. Use when search results look stale/missing, after indexing a workspace, or before assuming nano-brain has no relevant context. Check queue_pending before relying on vector/hybrid results.",
			InputSchema: toolSchema(map[string]map[string]any{}, nil),
		},
		func(ctx context.Context, _ *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			pgStatus := "healthy"
			if a.pool != nil {
				if err := a.pool.Ping(ctx); err != nil {
					pgStatus = "unreachable"
				}
			} else {
				pgStatus = "not configured"
			}

			resp := map[string]any{
				"pg_status":       pgStatus,
				"active_provider": a.embedCfg.Provider,
			}

			if a.embedQueue != nil {
				resp["queue_depth"] = a.embedQueue.Depth()
				resp["queue_capacity"] = a.embedQueue.Capacity()
				resp["queue_status"] = a.embedQueue.Status()
				resp["queue_pending"] = a.embedQueue.PendingCount()
			}

			return textResult(resp)
		},
	)
}

func registerMemoryUpdate(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_update",
			Description: "Trigger re-embedding/reindexing work for a registered workspace. Use only when indexed content appears stale after checking memory_status; normal file watching should handle routine updates.",
			InputSchema: toolSchema(map[string]map[string]any{
				"workspace": {"type": "string", "description": "Workspace identifier — name (e.g. 'nano-brain') or full hash. Optional if the MCP connection was configured with a default workspace via the ?workspace= URL query param; otherwise required."},
			}, []string{}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := requireRegisteredWorkspace(ctx, a, args)
			if errRes != nil {
				return errRes, nil
			}
			return textResult(map[string]string{
				"status":  "accepted",
				"message": fmt.Sprintf("reindex requested for workspace %s", ws),
			})
		},
	)
}

func registerMemoryWakeUp(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_wake_up",
			Description: "Session-start workspace briefing. Call first when entering a workspace to orient the agent with recent memories, active collections, stats, and last activity before deeper search.",
			InputSchema: toolSchema(map[string]map[string]any{
				"workspace": {"type": "string", "description": "Workspace identifier — name (e.g. 'nano-brain') or full hash. Optional if the MCP connection was configured with a default workspace via the ?workspace= URL query param; otherwise required."},
				"limit":     {"type": "number", "description": "Number of recent memories (default 10, max 50)"},
			}, []string{}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := a.requireWorkspace(ctx, args)
			if errRes != nil {
				return errRes, nil
			}
			if ws == "all" {
				return errResult("cross-workspace not supported for this tool"), nil
			}
			limit := argInt(args, "limit", 10, 50)

			// Required since #338/PR #340 — nil makes ANY('{}'::text[]) always false. See #356.
			docs, err := a.queries.RecentDocuments(ctx, sqlc.RecentDocumentsParams{
				WorkspaceHash: ws,
				MaxResults:    int32(limit),
				Collections:   []string{"memory", "sessions"},
			})
			if err != nil {
				return errResult(fmt.Sprintf("recent documents failed: %v", err)), nil
			}

			docStats, err := a.queries.WorkspaceDocStats(ctx, ws)
			if err != nil {
				return errResult(fmt.Sprintf("workspace stats failed: %v", err)), nil
			}

			chunkCount, err := a.queries.WorkspaceChunkCount(ctx, ws)
			if err != nil {
				return errResult(fmt.Sprintf("chunk count failed: %v", err)), nil
			}

			collections, err := a.queries.ListCollectionsWithLastUpdated(ctx, ws)
			if err != nil {
				return errResult(fmt.Sprintf("collections failed: %v", err)), nil
			}

			type recentMemory struct {
				ID      string   `json:"id"`
				Title   string   `json:"title"`
				Snippet string   `json:"snippet"`
				Tags    []string `json:"tags"`
				Date    string   `json:"date"`
			}
			type activeCollection struct {
				Name          string `json:"name"`
				DocumentCount int64  `json:"document_count"`
				LastUpdated   string `json:"last_updated"`
			}

			memories := make([]recentMemory, 0, len(docs))
			for _, d := range docs {
				tags := d.Tags
				if tags == nil {
					tags = []string{}
				}
				memories = append(memories, recentMemory{
					ID:      d.ID.String(),
					Title:   d.Title,
					Snippet: d.Snippet,
					Tags:    tags,
					Date:    d.UpdatedAt.Format(time.RFC3339),
				})
			}

			cols := make([]activeCollection, 0, len(collections))
			for _, c := range collections {
				lastUpdated := ""
				if t, ok := c.LastUpdated.(time.Time); ok {
					lastUpdated = t.Format(time.RFC3339)
				}
				cols = append(cols, activeCollection{
					Name:          c.Name,
					DocumentCount: c.DocumentCount,
					LastUpdated:   lastUpdated,
				})
			}

			var lastActivity string
			if t, ok := docStats.LastUpdated.(time.Time); ok {
				lastActivity = t.Format(time.RFC3339)
			}

			var summary string
			timeAgo := "never"
			if t, ok := docStats.LastUpdated.(time.Time); ok {
				d := time.Since(t)
				switch {
				case d < time.Minute:
					timeAgo = "just now"
				case d < time.Hour:
					timeAgo = fmt.Sprintf("%dm ago", int(d.Minutes()))
				case d < 24*time.Hour:
					timeAgo = fmt.Sprintf("%dh ago", int(d.Hours()))
				default:
					timeAgo = fmt.Sprintf("%dd ago", int(d.Hours()/24))
				}
			}
			summary = fmt.Sprintf("Workspace has %d documents across %d collections. Last activity: %s.",
				docStats.TotalDocuments, len(collections), timeAgo)

			return textResult(map[string]any{
				"summary":            summary,
				"recent_memories":    memories,
				"active_collections": cols,
				"stats": map[string]any{
					"total_documents": docStats.TotalDocuments,
					"total_chunks":    chunkCount,
					"last_activity":   lastActivity,
				},
			})
		},
	)
}

func registerMemoryGraph(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_graph",
			Description: "One-hop code graph lookup. Use for direct callers/callees, imports, and containment around a known file or file::symbol. For broad blast radius before editing use memory_impact; for downstream chains use memory_trace. Node accepts workspace-relative or absolute paths (e.g. \"internal/x.go::F\" or \"/abs/path/internal/x.go::F\").",
			InputSchema: toolSchema(map[string]map[string]any{
				"workspace": {"type": "string", "description": "Workspace identifier — name (e.g. 'nano-brain') or full hash. Optional if the MCP connection was configured with a default workspace via the ?workspace= URL query param; otherwise required."},
				"node":      {"type": "string", "description": "Known file path or file::symbol to inspect. Use memory_symbols first if you only know the symbol name."},
				"direction": {"type": "string", "description": "Edge direction: out (what this node calls/imports/contains), in (direct callers/importers/containers), both"},
				"edge_type": {"type": "string", "description": "Filter by edge type: contains, imports, calls (empty = all). Use calls for execution relationships."},
				"paths":     {"type": "string", "description": "Output path style: \"absolute\" (default, current behavior) or \"relative\" (strip workspace prefix from edge source/target where present)"},
			}, []string{"node"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := a.requireWorkspace(ctx, args)
			if errRes != nil {
				return errRes, nil
			}
			node := argString(args, "node")
			if node == "" {
				return errResult("node is required"), nil
			}
			node, err = normalizeNodeForQuery(ctx, a.queries, ws, node)
			if err != nil {
				return errResult(err.Error()), nil
			}
			direction := argString(args, "direction")
			if direction == "" {
				direction = "out"
			}
			edgeType := argString(args, "edge_type")
			pathStyle := argString(args, "paths")

			type edgeResult struct {
				Source   string `json:"source"`
				Target   string `json:"target"`
				EdgeType string `json:"edge_type"`
			}

			var rows []sqlc.GraphEdge
			switch direction {
			case "in":
				rows, err = a.queries.GetIncomingEdges(ctx, sqlc.GetIncomingEdgesParams{
					WorkspaceHash: ws,
					TargetNode:    node,
					Column3:       edgeType,
				})
			case "both":
				out, errOut := a.queries.GetOutgoingEdges(ctx, sqlc.GetOutgoingEdgesParams{
					WorkspaceHash: ws,
					SourceNode:    node,
					Column3:       edgeType,
				})
				in, errIn := a.queries.GetIncomingEdges(ctx, sqlc.GetIncomingEdgesParams{
					WorkspaceHash: ws,
					TargetNode:    node,
					Column3:       edgeType,
				})
				if errOut != nil {
					err = errOut
				} else if errIn != nil {
					err = errIn
				} else {
					rows = append(out, in...)
				}
			default:
				rows, err = a.queries.GetOutgoingEdges(ctx, sqlc.GetOutgoingEdgesParams{
					WorkspaceHash: ws,
					SourceNode:    node,
					Column3:       edgeType,
				})
			}

			if err != nil {
				return errResult(fmt.Sprintf("graph query failed: %v", err)), nil
			}

			var wsRoot string
			if pathStyle == "relative" {
				wsRoot = lookupWorkspaceRoot(ctx, a.queries, ws)
			}
			results := make([]edgeResult, 0, len(rows))
			for _, r := range rows {
				results = append(results, edgeResult{
					Source:   stripWorkspacePrefix(wsRoot, r.SourceNode),
					Target:   stripWorkspacePrefix(wsRoot, r.TargetNode),
					EdgeType: r.EdgeType,
				})
			}
			return textResult(map[string]any{
				"node":      stripWorkspacePrefix(wsRoot, node),
				"direction": direction,
				"edges":     results,
				"count":     len(results),
			})
		},
	)
}

func registerMemoryTrace(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_trace",
			Description: "Downstream call-chain trace from a known entry symbol. Use to answer 'what does this function eventually call?' with transitive calls and cycle detection. For one-hop neighbors use memory_graph; for affected callers before edits use memory_impact. Node accepts workspace-relative or absolute paths.",
			InputSchema: toolSchema(map[string]map[string]any{
				"workspace":        {"type": "string", "description": "Workspace identifier — name (e.g. 'nano-brain') or full hash. Optional if the MCP connection was configured with a default workspace via the ?workspace= URL query param; otherwise required."},
				"node":             {"type": "string", "description": "Known entry symbol (e.g. file::FunctionName) whose downstream calls should be followed. Use memory_symbols first if needed."},
				"max_depth":        {"type": "number", "description": "Max traversal depth 1-10 (default 5)"},
				"paths":            {"type": "string", "description": "Output path style: \"absolute\" (default) or \"relative\""},
				"include_external": {"type": "boolean", "description": "Include calls to builtins/third-party symbols that cannot be resolved to a workspace file (default false — these are dropped)"},
			}, []string{"node"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := a.requireWorkspace(ctx, args)
			if errRes != nil {
				return errRes, nil
			}
			node := argString(args, "node")
			if node == "" {
				return errResult("node is required"), nil
			}
			node, err = normalizeNodeForQuery(ctx, a.queries, ws, node)
			if err != nil {
				return errResult(err.Error()), nil
			}
			maxDepth := argInt(args, "max_depth", 5, 10)
			pathStyle := argString(args, "paths")
			includeExternal, _ := args["include_external"].(bool)

			seen := map[string]bool{node: true}
			type traceItem struct {
				Node      string `json:"node"`
				Name      string `json:"name"`
				Depth     int    `json:"depth"`
				Via       string `json:"via"`
				Ambiguous bool   `json:"ambiguous,omitempty"`
				External  bool   `json:"external,omitempty"`
			}
			var chain []traceItem

			type frame struct {
				node  string
				depth int
				via   string
			}
			queue := []frame{{node: node, depth: 0, via: ""}}

			for len(queue) > 0 {
				cur := queue[0]
				queue = queue[1:]
				if cur.depth >= maxDepth {
					continue
				}
				var edges []sqlc.GraphEdge
				var err error
				if strings.Contains(cur.node, "::") {
					edges, err = a.queries.GetOutgoingEdges(ctx, sqlc.GetOutgoingEdgesParams{
						WorkspaceHash: ws,
						SourceNode:    cur.node,
						Column3:       "calls",
					})
				} else {
					edges, err = a.queries.GetOutgoingEdgesBySymbol(ctx, sqlc.GetOutgoingEdgesBySymbolParams{
						WorkspaceHash: ws,
						SourceNode:    cur.node,
						Column3:       "calls",
					})
				}
				if err != nil {
					return errResult(fmt.Sprintf("trace query failed: %v", err)), nil
				}
				for _, e := range edges {
					// calls-edge targets are stored as bare identifiers (e.g.
					// "applyCredit"); requalify each one as "file::symbol" so the
					// chain stays feedable into memory_get/memory_graph and two
					// unrelated files defining the same-named symbol don't collide
					// into a single traversal node.
					if strings.Contains(e.TargetNode, "::") {
						if seen[e.TargetNode] {
							continue
						}
						seen[e.TargetNode] = true
						chain = append(chain, traceItem{
							Node:  e.TargetNode,
							Name:  e.TargetNode[strings.Index(e.TargetNode, "::")+2:],
							Depth: cur.depth + 1,
							Via:   cur.node,
						})
						queue = append(queue, frame{node: e.TargetNode, depth: cur.depth + 1, via: cur.node})
						continue
					}

					matches, rErr := a.queries.ResolveSymbolByName(ctx, sqlc.ResolveSymbolByNameParams{
						WorkspaceHash: ws,
						Column2:       e.TargetNode,
					})
					if rErr != nil {
						return errResult(fmt.Sprintf("trace query failed: %v", rErr)), nil
					}
					if len(matches) == 0 {
						// Builtin or third-party symbol — not defined anywhere in
						// this workspace. Drop by default; traversal stops here
						// either way since it has no outgoing edges of its own.
						if !includeExternal || seen[e.TargetNode] {
							continue
						}
						seen[e.TargetNode] = true
						chain = append(chain, traceItem{
							Node:     e.TargetNode,
							Name:     e.TargetNode,
							Depth:    cur.depth + 1,
							Via:      cur.node,
							External: true,
						})
						continue
					}
					ambiguous := len(matches) > 1
					for _, m := range matches {
						// Symbol docs are stored as "<relpath>?symbol=...". Guard
						// against a malformed source_path with no "?" (e.g. a
						// hand-written doc via memory_write) so a bad row can't
						// panic the trace handler on a negative slice index.
						qIdx := strings.Index(m.SourcePath, "?")
						if qIdx < 0 {
							continue
						}
						qualified := m.SourcePath[:qIdx] + "::" + e.TargetNode
						if seen[qualified] {
							continue
						}
						seen[qualified] = true
						chain = append(chain, traceItem{
							Node:      qualified,
							Name:      e.TargetNode,
							Depth:     cur.depth + 1,
							Via:       cur.node,
							Ambiguous: ambiguous,
						})
						queue = append(queue, frame{node: qualified, depth: cur.depth + 1, via: cur.node})
					}
				}
			}

			var wsRoot string
			if pathStyle == "relative" {
				wsRoot = lookupWorkspaceRoot(ctx, a.queries, ws)
				for i := range chain {
					chain[i].Node = stripWorkspacePrefix(wsRoot, chain[i].Node)
					chain[i].Via = stripWorkspacePrefix(wsRoot, chain[i].Via)
				}
			}

			return textResult(map[string]any{
				"entry": stripWorkspacePrefix(wsRoot, node),
				"chain": chain,
				"count": len(chain),
			})
		},
	)
}

func registerMemoryImpact(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_impact",
			Description: "Pre-change blast-radius analysis. Use before editing/refactoring a file or symbol to find affected callers/importers/dependents via reverse import/call traversal. For direct one-hop relationships use memory_graph; for downstream calls use memory_trace. Node accepts workspace-relative or absolute paths.",
			InputSchema: toolSchema(map[string]map[string]any{
				"workspace": {"type": "string", "description": "Workspace identifier — name (e.g. 'nano-brain') or full hash. Optional if the MCP connection was configured with a default workspace via the ?workspace= URL query param; otherwise required."},
				"node":      {"type": "string", "description": "File path or file::symbol you plan to change. Use paths='relative' to save tokens in large workspaces."},
				"direction": {"type": "string", "description": "Edge direction: \"in\" (default, affected callers/importers/dependents), \"out\" (dependencies this node uses)"},
				"edge_type": {"type": "string", "description": "Filter by edge type: imports, calls (empty = all)"},
				"max_depth": {"type": "number", "description": "Traversal depth 1-3 (default 1)"},
				"paths":     {"type": "string", "description": "Output path style: \"absolute\" (default) or \"relative\""},
			}, []string{"node"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := a.requireWorkspace(ctx, args)
			if errRes != nil {
				return errRes, nil
			}
			node := argString(args, "node")
			if node == "" {
				return errResult("node is required"), nil
			}
			node, err = normalizeNodeForQuery(ctx, a.queries, ws, node)
			if err != nil {
				return errResult(err.Error()), nil
			}
			edgeType := argString(args, "edge_type")
			direction := argString(args, "direction")
			if direction == "" {
				direction = "in"
			}
			if direction != "in" && direction != "out" {
				return errResult("direction must be \"in\" or \"out\""), nil
			}
			maxDepth := argInt(args, "max_depth", 1, 3)
			pathStyle := argString(args, "paths")

			frontier := []string{node}
			if direction == "in" {
				frontier = symbol.ExpandImpactFrontier(frontier)
			}
			seen := map[string]bool{node: true}
			queried := map[string]bool{}

			type impactItem struct {
				Node     string `json:"node"`
				Depth    int    `json:"depth"`
				EdgeType string `json:"edge_type"`
			}
			var impacted []impactItem

			for depth := 1; depth <= maxDepth && len(frontier) > 0; depth++ {
				switch direction {
				case "out":
					rows, err := a.queries.GetOutgoingEdgesBySources(ctx, sqlc.GetOutgoingEdgesBySourcesParams{
						WorkspaceHash: ws,
						Column2:       frontier,
						Column3:       edgeType,
					})
					if err != nil {
						return errResult(fmt.Sprintf("impact query failed: %v", err)), nil
					}
					var next []string
					for _, r := range rows {
						if seen[r.TargetNode] {
							continue
						}
						seen[r.TargetNode] = true
						impacted = append(impacted, impactItem{
							Node:     r.TargetNode,
							Depth:    depth,
							EdgeType: r.EdgeType,
						})
						next = append(next, r.TargetNode)
					}
					frontier = next
				default:
					targets := make([]string, 0, len(frontier))
					for _, f := range frontier {
						if queried[f] {
							continue
						}
						queried[f] = true
						targets = append(targets, f)
					}
					if len(targets) == 0 {
						frontier = nil
						break
					}
					rows, err := a.queries.GetImpactorsByTargets(ctx, sqlc.GetImpactorsByTargetsParams{
						WorkspaceHash: ws,
						Column2:       targets,
						Column3:       edgeType,
					})
					if err != nil {
						return errResult(fmt.Sprintf("impact query failed: %v", err)), nil
					}
					var next []string
					for _, r := range rows {
						if seen[r.SourceNode] {
							continue
						}
						seen[r.SourceNode] = true
						impacted = append(impacted, impactItem{
							Node:     r.SourceNode,
							Depth:    depth,
							EdgeType: r.EdgeType,
						})
						next = append(next, r.SourceNode)
					}
					frontier = symbol.ExpandImpactFrontier(next)
				}
			}

			var wsRoot string
			if pathStyle == "relative" {
				wsRoot = lookupWorkspaceRoot(ctx, a.queries, ws)
				for i := range impacted {
					impacted[i].Node = stripWorkspacePrefix(wsRoot, impacted[i].Node)
				}
			}

			return textResult(map[string]any{
				"node":     stripWorkspacePrefix(wsRoot, node),
				"impacted": impacted,
				"count":    len(impacted),
			})
		},
	)
}

func registerMemorySymbols(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_symbols",
			Description: "Code symbol lookup. Use when you know or suspect a function, method, class, interface, type, const, or variable name and need its source file/signature before graph/impact/trace. Prefer this over free-text search for exact code identifiers.",
			InputSchema: toolSchema(map[string]map[string]any{
				"workspace": {"type": "string", "description": "Workspace identifier — name (e.g. 'nano-brain') or full hash. Optional if the MCP connection was configured with a default workspace via the ?workspace= URL query param; otherwise required."},
				"query":     {"type": "string", "description": "Symbol name or partial identifier to locate, such as PriceModel, handleSubmit, UserService, or TRADE_PROCESS_STATES"},
				"kind":      {"type": "string", "description": "Symbol kind: function, method, type, interface, struct, const, var"},
				"limit":     {"type": "number", "description": "Max results (default 50)"},
			}, []string{}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := a.requireWorkspace(ctx, args)
			if errRes != nil {
				return errRes, nil
			}
			query := argString(args, "query")
			kind := argString(args, "kind")
			limit := int32(argInt(args, "limit", 50, 200))

			rows, err := a.queries.ListSymbolsByWorkspace(ctx, sqlc.ListSymbolsByWorkspaceParams{
				WorkspaceHash: ws,
				Column2:       query,
				Column3:       kind,
				Limit:         limit,
			})
			if err != nil {
				return errResult(fmt.Sprintf("list symbols failed: %v", err)), nil
			}

			type symbolResult struct {
				Name       string  `json:"name"`
				Kind       string  `json:"kind,omitempty"`
				Language   string  `json:"language,omitempty"`
				Signature  string  `json:"signature,omitempty"`
				SourcePath string  `json:"source_path"`
				Summary    *string `json:"summary"`
				StartLine  int     `json:"start_line,omitempty"`
				EndLine    int     `json:"end_line,omitempty"`
			}
			results := make([]symbolResult, 0, len(rows))
			for _, r := range rows {
				item := symbolResult{
					Name:       r.Title,
					SourcePath: r.SourcePath,
				}
				if r.Metadata.Valid {
					var meta map[string]string
					if err := json.Unmarshal(r.Metadata.RawMessage, &meta); err == nil {
						item.Kind = meta["kind"]
						item.Language = meta["language"]
						item.Signature = meta["signature"]
						item.StartLine, _ = strconv.Atoi(meta["line"])
						item.EndLine, _ = strconv.Atoi(meta["end_line"])
					}
				}
				results = append(results, item)
			}
			return textResult(map[string]any{
				"symbols": results,
				"count":   len(results),
			})
		},
	)
}

func registerMemoryWorkspacesResolve(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_workspaces_resolve",
			Description: "Resolve a filesystem path to a deterministic workspace hash and report whether it is registered. Read-only — does not register the workspace; use POST /api/v1/init for that.",
			InputSchema: toolSchema(map[string]map[string]any{
				"path": {"type": "string", "description": "Absolute filesystem path to the project root"},
			}, []string{"path"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			path, _ := args["path"].(string)
			if path == "" {
				return errResult("path is required"), nil
			}
			absPath, err := filepath.Abs(path)
			if err != nil {
				return errResult("invalid path"), nil
			}
			hash, err := storage.WorkspaceHash(absPath)
			if err != nil {
				return errResult("invalid path"), nil
			}

			ws, err := a.queries.GetWorkspaceByHash(ctx, hash)
			if err == nil {
				useName := ws.Name
				if useName == "" {
					useName = ws.Hash
				}
				return textResult(map[string]any{
					"workspace_hash": ws.Hash,
					"workspace_name": ws.Name,
					"root_path":      ws.Path,
					"registered":     true,
					"use":            useName,
				})
			}
			if errors.Is(err, sql.ErrNoRows) {
				return textResult(map[string]any{
					"workspace_hash": hash,
					"workspace_name": filepath.Base(absPath),
					"root_path":      absPath,
					"registered":     false,
					"use":            hash,
				})
			}
			return errResult(fmt.Sprintf("resolve workspace failed: %v", err)), nil
		},
	)
}

func registerMemoryFlow(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_flow",
			Description: "HTTP/request execution-flow visualization. Use for route-level questions like 'what happens on POST /api/v1/write?' Shows middleware, handler, downstream calls, and optional Mermaid/sequence output. For non-HTTP symbol chains use memory_trace; for one function CFG use memory_flowchart. Returns found:false when the entry is not indexed or flow indexing is disabled.",
			InputSchema: toolSchema(map[string]map[string]any{
				"workspace":         {"type": "string", "description": "Workspace identifier — name (e.g. 'nano-brain') or full hash. Optional if the MCP connection was configured with a default workspace via the ?workspace= URL query param; otherwise required."},
				"entry":             {"type": "string", "description": "HTTP route entry point to visualize, e.g. 'POST /api/v1/write'. Use method + path when possible."},
				"max_depth":         {"type": "number", "description": "Max call-chain depth 1-10 (default: config value)"},
				"format":            {"type": "string", "description": "Output format: 'mermaid' (default), 'sequence' (sequence diagram), or 'json'"},
				"stitch_workspaces": {"type": "array", "description": "Target workspace hashes to stitch cross-service integration edges against", "items": map[string]any{"type": "string"}},
			}, []string{"entry"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			if !a.flowCfg.Enabled {
				return textResult(map[string]any{
					"found":   false,
					"message": "flow indexing disabled",
				})
			}

			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := a.requireWorkspace(ctx, args)
			if errRes != nil {
				return errRes, nil
			}
			entry := argString(args, "entry")
			if entry == "" {
				return errResult("entry is required"), nil
			}
			format := argString(args, "format")
			if format == "" {
				format = "mermaid"
			}
			maxDepth := argInt(args, "max_depth", a.flowCfg.MaxDepth, a.flowCfg.MaxDepth)

			rawEdges, err := a.queries.ListAllEdgesByWorkspace(ctx, ws)
			if err != nil {
				return errResult(fmt.Sprintf("flow query failed: %v", err)), nil
			}

			edges := make([]graph.Edge, 0, len(rawEdges))
			for _, r := range rawEdges {
				e := graph.Edge{
					SourceNode: r.SourceNode,
					TargetNode: r.TargetNode,
					Kind:       graph.EdgeKind(r.EdgeType),
					SourceFile: r.SourceFile,
				}
				if len(r.Metadata) > 0 {
					var meta map[string]any
					if jsonErr := json.Unmarshal(r.Metadata, &meta); jsonErr == nil {
						if lang, ok := meta["language"].(string); ok {
							e.Language = lang
						}
						e.Metadata = meta
					}
				}
				edges = append(edges, e)
			}

			// HTTP routes are the primary entries. If no HTTP edge matches, allow a
			// Rails/Ruby class, job, worker, or service entry that has indexed graph
			// edges. BuildFlow always emits the entry node itself, so node count cannot
			// distinguish a real flow from an unknown entry.
			if !mcpHasFlowEntry(edges, entry) {
				return textResult(map[string]any{
					"found":   false,
					"entry":   entry,
					"message": "entry not found among flow edges",
				})
			}

			f := flow.BuildFlow(edges, entry, maxDepth, a.flowCfg.MaxFanout)

			stitchWorkspaces := argStringSlice(args, "stitch_workspaces")
			if len(stitchWorkspaces) > 0 {
				publishEdges := filterPublishEdges(edges)
				stitched := flow.Stitch(ctx, publishEdges, stitchWorkspaces, a.queries)
				appendStitchedToFlow(&f, stitched)
			}

			type nodeItem struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				Role      string `json:"role"`
				Ambiguous bool   `json:"ambiguous,omitempty"`
			}
			var chain []nodeItem
			var externals []nodeItem
			for _, n := range f.Nodes {
				ni := nodeItem{
					ID:        n.ID,
					Name:      n.Name,
					Role:      string(n.Role),
					Ambiguous: n.Ambiguous,
				}
				if n.Role == flow.RoleExternal {
					externals = append(externals, ni)
				} else {
					chain = append(chain, ni)
				}
			}
			if chain == nil {
				chain = []nodeItem{}
			}
			if externals == nil {
				externals = []nodeItem{}
			}

			type edgeItem struct {
				From        string `json:"from"`
				To          string `json:"to"`
				Kind        string `json:"kind"`
				Line        int    `json:"line,omitempty"`
				Conditional bool   `json:"conditional,omitempty"`
			}
			allNodes := make([]nodeItem, 0, len(f.Nodes))
			for _, n := range f.Nodes {
				allNodes = append(allNodes, nodeItem{
					ID:        n.ID,
					Name:      n.Name,
					Role:      string(n.Role),
					Ambiguous: n.Ambiguous,
				})
			}
			graphEdges := make([]edgeItem, 0, len(f.Edges))
			for _, e := range f.Edges {
				graphEdges = append(graphEdges, edgeItem{
					From:        e.From,
					To:          e.To,
					Kind:        e.Kind,
					Line:        e.Line,
					Conditional: e.Conditional,
				})
			}

			result := map[string]any{
				"found":     true,
				"entry":     f.Entry,
				"method":    f.Method,
				"path":      f.Path,
				"chain":     chain,
				"externals": externals,
				"nodes":     allNodes,
				"edges":     graphEdges,
			}
			if diagram := flow.Render(f, format, loadCFGsForFlow(ctx, a.queries, ws, format, f)...); diagram != "" {
				result["mermaid"] = diagram
			}

			return textResult(result)
		},
	)
}

func mcpHasFlowEntry(edges []graph.Edge, entry string) bool {
	for _, e := range edges {
		if e.Kind == graph.EdgeHTTP && e.SourceNode == entry {
			return true
		}
	}
	if strings.Contains(entry, " ") {
		return false
	}
	for _, e := range edges {
		if e.SourceNode == entry || mcpFlowSymbolMatches(mcpFlowSymbolPart(e.SourceNode), entry) {
			return true
		}
		if e.Kind == graph.EdgeContains && (e.TargetNode == entry || mcpFlowSymbolMatches(mcpFlowSymbolPart(e.TargetNode), entry)) {
			return true
		}
	}
	return false
}

func mcpFlowSymbolPart(node string) string {
	if idx := strings.LastIndex(node, "::"); idx >= 0 {
		return node[idx+2:]
	}
	return node
}

func mcpFlowSymbolMatches(symbol, entry string) bool {
	return symbol == entry || (!strings.Contains(entry, "#") && strings.HasPrefix(symbol, entry+"#"))
}

func loadCFGsForFlow(ctx context.Context, q flow.CFGQuerier, ws, format string, f flow.Flow) []flow.FlowCFGs {
	if format != "sequence" {
		return nil
	}
	cfgs, err := flow.LoadFlowCFGs(ctx, q, ws, f.Entry)
	if err != nil || cfgs == nil {
		return nil
	}
	return []flow.FlowCFGs{cfgs}
}

func filterPublishEdges(edges []graph.Edge) []graph.Edge {
	var out []graph.Edge
	for _, e := range edges {
		if e.Kind != graph.EdgeIntegration {
			continue
		}
		if topic, ok := e.Metadata["topic"].(string); ok && topic != "" {
			out = append(out, e)
		}
	}
	return out
}

func appendStitchedToFlow(f *flow.Flow, stitched []flow.FlowEdge) {
	if len(stitched) == 0 {
		return
	}
	existing := make(map[string]bool, len(f.Nodes))
	for _, n := range f.Nodes {
		existing[n.ID] = true
	}
	for _, se := range stitched {
		if !existing[se.To] {
			existing[se.To] = true
			f.Nodes = append(f.Nodes, flow.FlowNode{
				ID:   se.To,
				Name: se.To,
				Role: flow.RoleIntegration,
			})
		}
		f.Edges = append(f.Edges, se)
	}
}

func registerMemoryWorkspacesList(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_workspaces_list",
			Description: "List all registered workspaces with their paths, hashes, and document counts. Use this to find the correct workspace hash before calling other tools. Returns empty list if no workspaces are registered.",
			InputSchema: toolSchema(map[string]map[string]any{}, []string{}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			rows, err := a.queries.ListWorkspacesWithStats(ctx)
			if err != nil {
				return errResult(fmt.Sprintf("list workspaces failed: %v", err)), nil
			}

			workspaces := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				ws := map[string]any{
					"workspace_hash": r.Hash,
					"name":           r.Name,
					"root_path":      r.Path,
					"document_count": r.DocumentCount,
					"chunk_count":    r.ChunkCount,
				}
				if t, ok := r.LastDocumentUpdated.(time.Time); ok {
					ws["last_document_updated"] = t.Format(time.RFC3339)
				}
				workspaces = append(workspaces, ws)
			}

			return textResult(map[string]any{
				"workspaces": workspaces,
				"count":      len(workspaces),
			})
		},
	)
}

func registerMemoryTicket(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_ticket",
			Description: "Returns all sessions tagged with a ticket ID across ALL workspaces and sources. Use to answer 'what work has been done on DEV-4706?' Accepts Jira-style (DEV-1234, PROJ-42) or hash-style (#42) ticket IDs. Returns a formatted markdown list with title, source, workspace, and content snippet.",
			InputSchema: toolSchema(map[string]map[string]any{
				"ticket": {"type": "string", "description": "Ticket ID to look up, e.g. DEV-4706 or #42"},
			}, []string{"ticket"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ticket := strings.TrimSpace(argString(args, "ticket"))
			if ticket == "" {
				return errResult("ticket is required"), nil
			}

			// The write path (harvest/tickets.go) stores ticket IDs uppercased
			// (e.g. "ticket:DEV-4706"); ANY(tags) on a TEXT[] is case-sensitive,
			// so the query input must be uppercased to match. "#42"-style IDs
			// have no letters, so ToUpper is a no-op — consistent with write path.
			tagValue := "ticket:" + strings.ToUpper(ticket)
			rows, err := a.queries.ListDocumentsByTag(ctx, sqlc.ListDocumentsByTagParams{
				Column1:    tagValue,
				Collection: "sessions",
				Limit:      50,
			})
			if err != nil {
				return errResult(fmt.Sprintf("ticket query failed: %v", err)), nil
			}

			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: formatTicketSessions(ticket, rows)}},
			}, nil
		},
	)
}

// formatTicketSessions renders the memory_ticket result as a markdown list.
// Returns a "no sessions" message for an empty result set. Source is derived
// from each row's source_path scheme via the shared storage.SourceFromPath
// helper; results span all workspaces (the underlying query is unscoped).
func formatTicketSessions(ticket string, rows []sqlc.ListDocumentsByTagRow) string {
	if len(rows) == 0 {
		return fmt.Sprintf("No sessions found for ticket %s.", ticket)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "## Sessions for ticket %s\n\n", ticket)
	for _, row := range rows {
		wsShort := row.WorkspaceHash
		if len(wsShort) > 8 {
			wsShort = wsShort[:8]
		}
		src := storage.SourceFromPath(row.SourcePath)
		snip := strings.TrimSpace(row.Content)
		runes := []rune(snip)
		if len(runes) > 300 {
			snip = string(runes[:300])
		}
		fmt.Fprintf(&sb, "- **%s** (`%s`, workspace `%s`)\n  %s\n\n",
			row.Title, src, wsShort, snip)
	}
	return sb.String()
}
