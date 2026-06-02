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
	"strings"
	"time"

	"github.com/google/uuid"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nano-brain/nano-brain/internal/chunk"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
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
	registerMemoryWorkspacesResolve(server, a)
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

func requireWorkspace(args map[string]any) (string, *mcpsdk.CallToolResult) {
	ws := argString(args, "workspace")
	if ws == "" {
		return "", errResult("workspace is required")
	}
	return ws, nil
}

// requireRegisteredWorkspace extends requireWorkspace with a registration check
// against the workspaces table. Use in write tool handlers (memory_write,
// memory_update) — MCP transport bypasses HTTP middleware so registration
// enforcement must happen inside each write tool (issue #238). Rejects the
// literal "all" since cross-workspace writes are not supported.
func requireRegisteredWorkspace(ctx context.Context, a *Adapter, args map[string]any) (string, *mcpsdk.CallToolResult) {
	ws, errRes := requireWorkspace(args)
	if errRes != nil {
		return "", errRes
	}
	if ws == "all" {
		return "", errResult("workspace_all_not_supported: this tool does not accept the 'all' workspace scope; provide a specific registered workspace hash")
	}
	if _, err := a.queries.GetWorkspaceByHash(ctx, ws); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errResult(fmt.Sprintf("workspace_not_registered: workspace_hash %q is not registered; use POST /api/v1/init to register it first", ws))
		}
		return "", errResult(fmt.Sprintf("workspace_lookup_failed: %v", err))
	}
	return ws, nil
}

func registerMemoryQuery(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_query",
			Description: "Hybrid search combining BM25 text and vector similarity",
			InputSchema: toolSchema(map[string]map[string]any{
				"query":       {"type": "string", "description": "Search query"},
				"workspace":   {"type": "string", "description": "Workspace hash"},
				"max_results": {"type": "number", "description": "Max results (default 10, max 100)"},
			}, []string{"query", "workspace"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := requireWorkspace(args)
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
			results, err := a.searchService.HybridSearch(ctx, query, ws, maxResults, nil)
			if err != nil {
				return errResult(fmt.Sprintf("hybrid search failed: %v", err)), nil
			}
			return textResult(results)
		},
	)
}

func registerMemorySearch(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_search",
			Description: "BM25 text search across memory documents",
			InputSchema: toolSchema(map[string]map[string]any{
				"query":       {"type": "string", "description": "Search query"},
				"workspace":   {"type": "string", "description": "Workspace hash"},
				"max_results": {"type": "number", "description": "Max results (default 10, max 100)"},
				"tags":        {"type": "array", "description": "Filter by tags", "items": map[string]any{"type": "string"}},
			}, []string{"query", "workspace"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := requireWorkspace(args)
			if errRes != nil {
				return errRes, nil
			}
			query := argString(args, "query")
			if query == "" {
				return errResult("query is required"), nil
			}
			maxResults := argInt(args, "max_results", 10, 100)
			tags := argStringSlice(args, "tags")

			type resultRow struct {
				ID            string    `json:"id"`
				DocumentID    string    `json:"document_id"`
				WorkspaceHash string    `json:"workspace_hash"`
				Title         string    `json:"title"`
				Content       string    `json:"content"`
				SourcePath    string    `json:"source_path"`
				Collection    string    `json:"collection"`
				Tags          []string  `json:"tags"`
				Score         float64   `json:"score"`
				CreatedAt     time.Time `json:"created_at"`
				UpdatedAt     time.Time `json:"updated_at"`
			}

			var results []resultRow
			limit := int32(maxResults)

			if ws == "all" {
				if len(tags) > 0 {
					rows, err := a.queries.BM25SearchAllWithTags(ctx, sqlc.BM25SearchAllWithTagsParams{
						Query:      query,
						Tags:       tags,
						MaxResults: limit,
					})
					if err != nil {
						return errResult(fmt.Sprintf("bm25 search failed: %v", err)), nil
					}
					for _, r := range rows {
						results = append(results, resultRow{
							ID: r.ID.String(), DocumentID: r.DocumentID.String(),
							WorkspaceHash: r.WorkspaceHash, Title: r.Title,
							Content: r.Content, SourcePath: r.SourcePath,
							Collection: r.Collection, Tags: r.Tags,
							Score: r.Score, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
						})
					}
				} else {
					rows, err := a.queries.BM25SearchAll(ctx, sqlc.BM25SearchAllParams{
						Query:      query,
						MaxResults: limit,
					})
					if err != nil {
						return errResult(fmt.Sprintf("bm25 search failed: %v", err)), nil
					}
					for _, r := range rows {
						results = append(results, resultRow{
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
					Query:         query,
					WorkspaceHash: ws,
					Tags:          tags,
					MaxResults:    limit,
				})
				if err != nil {
					return errResult(fmt.Sprintf("bm25 search failed: %v", err)), nil
				}
				for _, r := range rows {
					results = append(results, resultRow{
						ID: r.ID.String(), DocumentID: r.DocumentID.String(),
						WorkspaceHash: r.WorkspaceHash, Title: r.Title,
						Content: r.Content, SourcePath: r.SourcePath,
						Collection: r.Collection, Tags: r.Tags,
						Score: r.Score, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
					})
				}
			} else {
				rows, err := a.queries.BM25Search(ctx, sqlc.BM25SearchParams{
					Query:         query,
					WorkspaceHash: ws,
					MaxResults:    limit,
				})
				if err != nil {
					return errResult(fmt.Sprintf("bm25 search failed: %v", err)), nil
				}
				for _, r := range rows {
					results = append(results, resultRow{
						ID: r.ID.String(), DocumentID: r.DocumentID.String(),
						WorkspaceHash: r.WorkspaceHash, Title: r.Title,
						Content: r.Content, SourcePath: r.SourcePath,
						Collection: r.Collection, Tags: r.Tags,
						Score: r.Score, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
					})
				}
			}
			return textResult(results)
		},
	)
}

func registerMemoryVSearch(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_vsearch",
			Description: "Vector similarity search using embeddings",
			InputSchema: toolSchema(map[string]map[string]any{
				"query":       {"type": "string", "description": "Search query"},
				"workspace":   {"type": "string", "description": "Workspace hash"},
				"max_results": {"type": "number", "description": "Max results (default 10, max 100)"},
			}, []string{"query", "workspace"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := requireWorkspace(args)
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

			embedCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			vec, err := a.embedder.Embed(embedCtx, query)
			if err != nil {
				return errResult(fmt.Sprintf("embedding query failed: %v", err)), nil
			}

			type vsearchRow struct {
				ID            string    `json:"id"`
				DocumentID    string    `json:"document_id"`
				WorkspaceHash string    `json:"workspace_hash"`
				Title         string    `json:"title"`
				Content       string    `json:"content"`
				SourcePath    string    `json:"source_path"`
				Collection    string    `json:"collection"`
				Tags          []string  `json:"tags"`
				Score         float64   `json:"score"`
				CreatedAt     time.Time `json:"created_at"`
				UpdatedAt     time.Time `json:"updated_at"`
			}

			var results []vsearchRow
			if ws == "all" {
				rows, err := a.queries.VectorSearchAll(ctx, sqlc.VectorSearchAllParams{
					QueryEmbedding: pgvector_go.NewVector(vec),
					MaxResults:     int32(maxResults),
				})
				if err != nil {
					return errResult(fmt.Sprintf("vector search failed: %v", err)), nil
				}
				results = make([]vsearchRow, 0, len(rows))
				for _, r := range rows {
					results = append(results, vsearchRow{
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
					MaxResults:     int32(maxResults),
				})
				if err != nil {
					return errResult(fmt.Sprintf("vector search failed: %v", err)), nil
				}
				results = make([]vsearchRow, 0, len(rows))
				for _, r := range rows {
					results = append(results, vsearchRow{
						ID: r.ChunkID.String(), DocumentID: r.DocumentID.String(),
						WorkspaceHash: r.WorkspaceHash, Title: r.Title,
						Content: r.Content, SourcePath: r.SourcePath,
						Collection: r.Collection, Tags: r.Tags,
						Score: r.Score, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
					})
				}
			}
			return textResult(results)
		},
	)
}

func registerMemoryGet(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_get",
			Description: "Get a document by ID or path",
			InputSchema: toolSchema(map[string]map[string]any{
				"path":       {"type": "string", "description": "Document source_path or #<uuid> for lookup by ID"},
				"workspace":  {"type": "string", "description": "Workspace hash"},
				"start_line": {"type": "number", "description": "Start line (1-indexed, inclusive)"},
				"end_line":   {"type": "number", "description": "End line (1-indexed, inclusive)"},
			}, []string{"path", "workspace"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := requireWorkspace(args)
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
			if strings.HasPrefix(path, "#") {
				docID, parseErr := uuid.Parse(strings.TrimPrefix(path, "#"))
				if parseErr != nil {
					return errResult(fmt.Sprintf("invalid document ID: %v", parseErr)), nil
				}
				doc, err = a.queries.GetDocumentByID(ctx, sqlc.GetDocumentByIDParams{
					ID:            docID,
					WorkspaceHash: ws,
				})
			} else {
				doc, err = a.queries.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
					SourcePath:    path,
					WorkspaceHash: ws,
				})
			}
			if err != nil {
				return errResult(fmt.Sprintf("document not found: %v", err)), nil
			}

			content := doc.Content
			startLine := argInt(args, "start_line", 0, 1<<30)
			endLine := argInt(args, "end_line", 0, 1<<30)
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
			Description: "Write or update a document in memory",
			InputSchema: toolSchema(map[string]map[string]any{
				"content":     {"type": "string", "description": "Document content"},
				"workspace":   {"type": "string", "description": "Workspace hash"},
				"title":       {"type": "string", "description": "Document title"},
				"tags":        {"type": "array", "description": "Document tags", "items": map[string]any{"type": "string"}},
				"collection":  {"type": "string", "description": "Collection name (default: memory)"},
				"source_path": {"type": "string", "description": "Source file path"},
				"metadata":    {"type": "object", "description": "Additional metadata"},
				"supersedes":  {"type": "string", "description": "Document to supersede (#<uuid> or source_path)"},
			}, []string{"content", "workspace"}),
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
						DocumentID:    row.ID,
						WorkspaceHash: ws,
						ContentHash:   ch.Hash,
						Content:       ch.Content,
						ChunkIndex:    int32(ch.Sequence),
						StartLine:     sql.NullInt32{Int32: int32(ch.StartLine), Valid: true},
						EndLine:       sql.NullInt32{Int32: int32(ch.EndLine), Valid: true},
						Metadata:      chunkMeta,
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
						DocumentID:    row.ID,
						WorkspaceHash: ws,
						ContentHash:   ch.Hash,
						Content:       ch.Content,
						ChunkIndex:    int32(ch.Sequence),
						StartLine:     sql.NullInt32{Int32: int32(ch.StartLine), Valid: true},
						EndLine:       sql.NullInt32{Int32: int32(ch.EndLine), Valid: true},
						Metadata:      chunkMeta,
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
			Description: "List collections in a workspace",
			InputSchema: toolSchema(map[string]map[string]any{
				"workspace": {"type": "string", "description": "Workspace hash"},
			}, []string{"workspace"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := requireWorkspace(args)
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
			Description: "Server and embedding queue status",
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
			Description: "Trigger re-embedding of a workspace",
			InputSchema: toolSchema(map[string]map[string]any{
				"workspace": {"type": "string", "description": "Workspace hash"},
			}, []string{"workspace"}),
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
			Description: "Workspace briefing with recent activity and stats",
			InputSchema: toolSchema(map[string]map[string]any{
				"workspace": {"type": "string", "description": "Workspace hash"},
				"limit":     {"type": "number", "description": "Number of recent memories (default 10, max 50)"},
			}, []string{"workspace"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := requireWorkspace(args)
			if errRes != nil {
				return errRes, nil
			}
			if ws == "all" {
				return errResult("cross-workspace not supported for this tool"), nil
			}
			limit := argInt(args, "limit", 10, 50)

			docs, err := a.queries.RecentDocuments(ctx, sqlc.RecentDocumentsParams{
				WorkspaceHash: ws,
				Limit:         int32(limit),
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
			Description: "Query the knowledge graph: find imports, calls, and symbol containment relationships for a node",
			InputSchema: toolSchema(map[string]map[string]any{
				"workspace": {"type": "string", "description": "Workspace hash"},
				"node":      {"type": "string", "description": "Source node (file path or file::symbol)"},
				"direction": {"type": "string", "description": "Edge direction: out (default), in, both"},
				"edge_type": {"type": "string", "description": "Filter by edge type: contains, imports, calls (empty = all)"},
			}, []string{"workspace", "node"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := requireWorkspace(args)
			if errRes != nil {
				return errRes, nil
			}
			node := argString(args, "node")
			if node == "" {
				return errResult("node is required"), nil
			}
			direction := argString(args, "direction")
			if direction == "" {
				direction = "out"
			}
			edgeType := argString(args, "edge_type")

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

			results := make([]edgeResult, 0, len(rows))
			for _, r := range rows {
				results = append(results, edgeResult{
					Source:   r.SourceNode,
					Target:   r.TargetNode,
					EdgeType: r.EdgeType,
				})
			}
			return textResult(map[string]any{
				"node":      node,
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
			Description: "Trace the call chain from an entry symbol — shows what a function calls, transitively, with cycle detection",
			InputSchema: toolSchema(map[string]map[string]any{
				"workspace": {"type": "string", "description": "Workspace hash"},
				"node":      {"type": "string", "description": "Entry symbol (e.g. file::FunctionName)"},
				"max_depth": {"type": "number", "description": "Max traversal depth 1-10 (default 5)"},
			}, []string{"workspace", "node"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := requireWorkspace(args)
			if errRes != nil {
				return errRes, nil
			}
			node := argString(args, "node")
			if node == "" {
				return errResult("node is required"), nil
			}
			maxDepth := argInt(args, "max_depth", 5, 10)

			seen := map[string]bool{node: true}
			type traceItem struct {
				Node  string `json:"node"`
				Depth int    `json:"depth"`
				Via   string `json:"via"`
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
				edges, err := a.queries.GetOutgoingEdges(ctx, sqlc.GetOutgoingEdgesParams{
					WorkspaceHash: ws,
					SourceNode:    cur.node,
					Column3:       "calls",
				})
				if err != nil {
					return errResult(fmt.Sprintf("trace query failed: %v", err)), nil
				}
				for _, e := range edges {
					if seen[e.TargetNode] {
						continue
					}
					seen[e.TargetNode] = true
					chain = append(chain, traceItem{
						Node:  e.TargetNode,
						Depth: cur.depth + 1,
						Via:   cur.node,
					})
					queue = append(queue, frame{node: e.TargetNode, depth: cur.depth + 1, via: cur.node})
				}
			}

			return textResult(map[string]any{
				"entry": node,
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
			Description: "Find what would be affected if a node (file or symbol) changes — reverse import/call lookup with optional depth traversal",
			InputSchema: toolSchema(map[string]map[string]any{
				"workspace": {"type": "string", "description": "Workspace hash"},
				"node":      {"type": "string", "description": "The node to analyze (file path or file::symbol)"},
				"edge_type": {"type": "string", "description": "Filter by edge type: imports, calls (empty = all)"},
				"max_depth": {"type": "number", "description": "Traversal depth 1-3 (default 1)"},
			}, []string{"workspace", "node"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := requireWorkspace(args)
			if errRes != nil {
				return errRes, nil
			}
			node := argString(args, "node")
			if node == "" {
				return errResult("node is required"), nil
			}
			edgeType := argString(args, "edge_type")
			maxDepth := argInt(args, "max_depth", 1, 3)

			frontier := []string{node}
			seen := map[string]bool{node: true}

			type impactItem struct {
				Node     string `json:"node"`
				Depth    int    `json:"depth"`
				EdgeType string `json:"edge_type"`
			}
			var impacted []impactItem

			for depth := 1; depth <= maxDepth && len(frontier) > 0; depth++ {
				rows, err := a.queries.GetImpactorsByTargets(ctx, sqlc.GetImpactorsByTargetsParams{
					WorkspaceHash: ws,
					Column2:       frontier,
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
				frontier = next
			}

			return textResult(map[string]any{
				"node":     node,
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
			Description: "Search code symbols (functions, types, methods, interfaces) extracted from indexed source files",
			InputSchema: toolSchema(map[string]map[string]any{
				"workspace": {"type": "string", "description": "Workspace hash"},
				"query":     {"type": "string", "description": "Symbol name filter (partial match)"},
				"kind":      {"type": "string", "description": "Symbol kind: function, method, type, interface, struct, const, var"},
				"limit":     {"type": "number", "description": "Max results (default 50)"},
			}, []string{"workspace"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := requireWorkspace(args)
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
				Name       string `json:"name"`
				Kind       string `json:"kind,omitempty"`
				Language   string `json:"language,omitempty"`
				Signature  string `json:"signature,omitempty"`
				SourcePath string `json:"source_path"`
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
				return textResult(map[string]any{
					"workspace_hash": ws.Hash,
					"root_path":      ws.Path,
					"name":           ws.Name,
					"registered":     true,
				})
			}
			if errors.Is(err, sql.ErrNoRows) {
				return textResult(map[string]any{
					"workspace_hash": hash,
					"root_path":      absPath,
					"name":           filepath.Base(absPath),
					"registered":     false,
				})
			}
			return errResult(fmt.Sprintf("resolve workspace failed: %v", err)), nil
		},
	)
}
