package mcp

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nano-brain/nano-brain/internal/chunk"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	pgvector_go "github.com/pgvector/pgvector-go"
	"github.com/sqlc-dev/pqtype"
)

// RegisterTools adds all 9 MCP tools to the server.
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
			results, err := a.searchService.HybridSearch(ctx, query, ws, maxResults)
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
				if s > total {
					s = total
				}
				if s > e {
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
			ws, errRes := requireWorkspace(args)
			if errRes != nil {
				return errRes, nil
			}
			if ws == "all" {
				return errResult("workspace 'all' is not valid for write operations"), nil
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
		func(_ context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := requireWorkspace(args)
			if errRes != nil {
				return errRes, nil
			}
			if ws == "all" {
				return errResult("workspace 'all' is not valid for write operations"), nil
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
