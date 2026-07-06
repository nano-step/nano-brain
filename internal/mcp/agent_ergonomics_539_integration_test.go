//go:build integration

package mcp_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sqlc-dev/pqtype"

	"github.com/nano-brain/nano-brain/internal/config"
	internalmcp "github.com/nano-brain/nano-brain/internal/mcp"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/rs/zerolog"
)

// setupFindingsMCP mirrors setupGraphMCP but leaves the fixture empty so each
// test (#539 findings 1-3) can seed only the documents/chunks/edges it needs.
// Unlike setupGraphMCP, graph edges here follow the real watcher convention
// (workspace-relative source/target nodes, e.g. filepath.Rel + ToSlash) —
// see internal/watcher/watcher.go relPath/relFile.
func setupFindingsMCP(t *testing.T) (context.Context, *sqlc.Queries, string, func(string, map[string]any) *mcpsdk.CallToolResult) {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })

	q := sqlc.New(db)
	wsHash := fmt.Sprintf("%x", sha256.Sum256([]byte("test_ws_"+uuid.New().String())))
	wsPath := "/tmp/test-ws-" + uuid.New().String()[:8]
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash, Name: "test-ws", Path: wsPath,
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(q, db, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{}, config.FlowConfig{}, nil, zerolog.Nop())
	internalmcp.RegisterTools(server, adapter)

	ct, st := mcpsdk.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })

	callTool := func(name string, args map[string]any) *mcpsdk.CallToolResult {
		t.Helper()
		result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("CallTool(%s): %v", name, err)
		}
		return result
	}
	return ctx, q, wsHash, callTool
}

// --- Finding 2: memory_get chunk-id resolution -----------------------------

func TestMemoryGet_ChunkIDResolvesToParentDocument(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)

	doc, err := q.UpsertDocument(ctx, sqlc.UpsertDocumentParams{
		WorkspaceHash: wsHash,
		ContentHash:   "hash-1",
		Title:         "doc-with-chunks",
		Content:       "full document content",
		SourcePath:    "notes/doc-with-chunks.md",
		Collection:    "memory",
	})
	if err != nil {
		t.Fatalf("upsert document: %v", err)
	}

	chunkID, err := q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
		DocumentID:        doc.ID,
		WorkspaceHash:     wsHash,
		ContentHash:       "chunk-hash-1",
		Content:           "chunk content",
		ChunkIndex:        0,
		ChunkType:         "text",
		EmbeddingStrategy: "none",
	})
	if err != nil {
		t.Fatalf("upsert chunk: %v", err)
	}

	// Bare UUID form.
	bare := callTool("memory_get", map[string]any{"workspace": wsHash, "path": chunkID.String()})
	bareResp := unmarshalGraphResp(t, bare)
	if bareResp["id"].(string) != doc.ID.String() {
		t.Errorf("bare chunk id: got document id %v, want %v", bareResp["id"], doc.ID)
	}
	if bareResp["content"].(string) != "full document content" {
		t.Errorf("bare chunk id: got content %q, want parent document content", bareResp["content"])
	}

	// #<uuid> form.
	hashForm := callTool("memory_get", map[string]any{"workspace": wsHash, "path": "#" + chunkID.String()})
	hashResp := unmarshalGraphResp(t, hashForm)
	if hashResp["id"].(string) != doc.ID.String() {
		t.Errorf("#chunk id: got document id %v, want %v", hashResp["id"], doc.ID)
	}

	// Document id form still works unchanged.
	docIDResult := callTool("memory_get", map[string]any{"workspace": wsHash, "path": "#" + doc.ID.String()})
	docIDResp := unmarshalGraphResp(t, docIDResult)
	if docIDResp["id"].(string) != doc.ID.String() {
		t.Errorf("#doc id: got %v, want %v", docIDResp["id"], doc.ID)
	}
}

func TestMemoryGet_UnknownIDReturnsCleanError(t *testing.T) {
	_, _, wsHash, callTool := setupFindingsMCP(t)

	unknown := uuid.New()
	result := callTool("memory_get", map[string]any{"workspace": wsHash, "path": "#" + unknown.String()})
	if !result.IsError {
		t.Fatal("expected error result for unknown id")
	}
	msg := result.Content[0].(*mcpsdk.TextContent).Text
	if strings.Contains(msg, "sql:") {
		t.Errorf("error message leaks raw sql error: %q", msg)
	}
	if !strings.Contains(msg, unknown.String()) {
		t.Errorf("error message should reference the requested id, got: %q", msg)
	}
}

// --- Finding 1: memory_trace qualifies bare calls-edge targets --------------

func upsertSymbolDoc(t *testing.T, ctx context.Context, q *sqlc.Queries, wsHash, file, name, kind, content, line, endLine string) {
	t.Helper()
	metaBytes := []byte(fmt.Sprintf(`{"source_type":"symbol","kind":%q,"language":"go","signature":%q,"line":%q,"end_line":%q}`, kind, content, line, endLine))
	sourcePath := file + "?symbol=" + name + "&kind=" + kind
	_, err := q.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: wsHash,
		ContentHash:   sourcePath,
		Title:         name,
		Content:       content,
		SourcePath:    sourcePath,
		Collection:    "code",
		Tags:          []string{"symbol", "go", kind},
		Metadata:      pqtype.NullRawMessage{RawMessage: metaBytes, Valid: true},
	})
	if err != nil {
		t.Fatalf("upsert symbol doc %s: %v", sourcePath, err)
	}
}

func TestMemoryTrace_QualifiesBareTargetsAndDropsExternal(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)

	// helper is defined once in callee.go — a bare "calls" edge to it should
	// resolve to the qualified "callee.go::helper" node.
	upsertSymbolDoc(t, ctx, q, wsHash, "callee.go", "helper", "function", "func helper() {}", "1", "1")

	edges := []struct{ source, target, etype string }{
		{"entry.go::Main", "helper", "calls"}, // in-repo, bare -> should qualify
		{"entry.go::Main", "push", "calls"},   // builtin, bare -> should be dropped by default
	}
	for _, e := range edges {
		if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
			WorkspaceHash: wsHash,
			SourceNode:    e.source,
			TargetNode:    e.target,
			EdgeType:      e.etype,
			SourceFile:    "entry.go",
			Metadata:      []byte("{}"),
		}); err != nil {
			t.Fatalf("upsert edge %+v: %v", e, err)
		}
	}

	result := callTool("memory_trace", map[string]any{
		"workspace": wsHash,
		"node":      "entry.go::Main",
		"max_depth": float64(2),
	})
	resp := unmarshalGraphResp(t, result)
	chain, _ := resp["chain"].([]any)
	if len(chain) != 1 {
		t.Fatalf("chain length = %d, want 1 (builtin 'push' should be dropped): %+v", len(chain), chain)
	}
	item := chain[0].(map[string]any)
	if item["node"].(string) != "callee.go::helper" {
		t.Errorf("chain[0].node = %q, want callee.go::helper", item["node"])
	}
	if item["name"].(string) != "helper" {
		t.Errorf("chain[0].name = %q, want helper", item["name"])
	}
	if v, ok := item["external"]; ok && v.(bool) {
		t.Errorf("chain[0] should not be external")
	}

	// With include_external=true, the builtin call shows up flagged.
	withExternal := callTool("memory_trace", map[string]any{
		"workspace":        wsHash,
		"node":             "entry.go::Main",
		"max_depth":        float64(2),
		"include_external": true,
	})
	extResp := unmarshalGraphResp(t, withExternal)
	extChain, _ := extResp["chain"].([]any)
	if len(extChain) != 2 {
		t.Fatalf("chain length with include_external = %d, want 2: %+v", len(extChain), extChain)
	}
	var sawPush bool
	for _, c := range extChain {
		cm := c.(map[string]any)
		if cm["node"].(string) == "push" {
			sawPush = true
			if ext, _ := cm["external"].(bool); !ext {
				t.Errorf("push entry should be flagged external: %+v", cm)
			}
		}
	}
	if !sawPush {
		t.Error("expected 'push' to appear when include_external=true")
	}
}

func TestMemoryTrace_AmbiguousSameNameSymbolsYieldDistinctNodes(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)

	upsertSymbolDoc(t, ctx, q, wsHash, "a.go", "process", "function", "func process() {}", "1", "1")
	upsertSymbolDoc(t, ctx, q, wsHash, "b.go", "process", "function", "func process() {}", "1", "1")

	if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
		WorkspaceHash: wsHash,
		SourceNode:    "entry.go::Main",
		TargetNode:    "process",
		EdgeType:      "calls",
		SourceFile:    "entry.go",
		Metadata:      []byte("{}"),
	}); err != nil {
		t.Fatalf("upsert edge: %v", err)
	}

	result := callTool("memory_trace", map[string]any{
		"workspace": wsHash,
		"node":      "entry.go::Main",
		"max_depth": float64(2),
	})
	resp := unmarshalGraphResp(t, result)
	chain, _ := resp["chain"].([]any)
	if len(chain) != 2 {
		t.Fatalf("chain length = %d, want 2 (one per definition of 'process'): %+v", len(chain), chain)
	}
	seen := map[string]bool{}
	for _, c := range chain {
		cm := c.(map[string]any)
		node := cm["node"].(string)
		seen[node] = true
		if ambiguous, _ := cm["ambiguous"].(bool); !ambiguous {
			t.Errorf("chain entry %q should be marked ambiguous", node)
		}
	}
	if !seen["a.go::process"] || !seen["b.go::process"] {
		t.Errorf("expected distinct nodes a.go::process and b.go::process, got %+v", seen)
	}
}

func TestMemoryTrace_EntrySymbolDoesNotReappearViaNameOnlyEdge(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)

	upsertSymbolDoc(t, ctx, q, wsHash, "entry.go", "Main", "function", "func Main() {}", "1", "1")

	// A self-recursive call, stored bare like any other calls-edge target.
	if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
		WorkspaceHash: wsHash,
		SourceNode:    "entry.go::Main",
		TargetNode:    "Main",
		EdgeType:      "calls",
		SourceFile:    "entry.go",
		Metadata:      []byte("{}"),
	}); err != nil {
		t.Fatalf("upsert edge: %v", err)
	}

	result := callTool("memory_trace", map[string]any{
		"workspace": wsHash,
		"node":      "entry.go::Main",
		"max_depth": float64(3),
	})
	resp := unmarshalGraphResp(t, result)
	chain, _ := resp["chain"].([]any)
	if len(chain) != 0 {
		t.Fatalf("chain length = %d, want 0 (entry symbol must not re-appear via its own name-only edge): %+v", len(chain), chain)
	}
}

// --- Finding 3: memory_symbols line span + memory_get symbol body ----------

func TestMemorySymbols_ExposesLineSpan(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)

	upsertSymbolDoc(t, ctx, q, wsHash, "body.go", "Foo", "function", "func Foo() {}", "2", "4")

	result := callTool("memory_symbols", map[string]any{"workspace": wsHash, "query": "Foo"})
	resp := unmarshalGraphResp(t, result)
	symbols, _ := resp["symbols"].([]any)
	if len(symbols) != 1 {
		t.Fatalf("symbols count = %d, want 1", len(symbols))
	}
	sym := symbols[0].(map[string]any)
	if int(sym["start_line"].(float64)) != 2 {
		t.Errorf("start_line = %v, want 2", sym["start_line"])
	}
	if int(sym["end_line"].(float64)) != 4 {
		t.Errorf("end_line = %v, want 4", sym["end_line"])
	}
}

func TestMemoryGet_SymbolPathReturnsFullBody(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)

	parentContent := "line1\nline2\nline3\nline4\nline5"
	if _, err := q.UpsertDocument(ctx, sqlc.UpsertDocumentParams{
		WorkspaceHash: wsHash,
		ContentHash:   "body-hash",
		Title:         "body.go",
		Content:       parentContent,
		SourcePath:    "body.go",
		Collection:    "code",
	}); err != nil {
		t.Fatalf("upsert parent file doc: %v", err)
	}
	upsertSymbolDoc(t, ctx, q, wsHash, "body.go", "Foo", "function", "func Foo() {}", "2", "4")

	result := callTool("memory_get", map[string]any{
		"workspace": wsHash,
		"path":      "body.go?symbol=Foo&kind=function",
	})
	resp := unmarshalGraphResp(t, result)
	if resp["content"].(string) != "line2\nline3\nline4" {
		t.Errorf("content = %q, want full body slice [2:4]", resp["content"])
	}

	// Explicit start_line/end_line still take precedence, sliced from the
	// same parent body.
	explicit := callTool("memory_get", map[string]any{
		"workspace":  wsHash,
		"path":       "body.go?symbol=Foo&kind=function",
		"start_line": float64(1),
		"end_line":   float64(1),
	})
	explicitResp := unmarshalGraphResp(t, explicit)
	if explicitResp["content"].(string) != "line1" {
		t.Errorf("explicit content = %q, want line1", explicitResp["content"])
	}
}

func TestMemoryGet_SymbolPathWithParentButNoLineMetadata_ReturnsSignature(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)

	// Parent file exists, but the symbol was indexed before line metadata was
	// captured (the default state for any workspace not yet re-indexed). The
	// symbol path must return the signature, NOT dump the whole parent file.
	parentContent := "package p\n\nfunc Qux() {}\n\nfunc other() {}"
	if _, err := q.UpsertDocument(ctx, sqlc.UpsertDocumentParams{
		WorkspaceHash: wsHash,
		ContentHash:   "noline-parent-hash",
		Title:         "noline.go",
		Content:       parentContent,
		SourcePath:    "noline.go",
		Collection:    "code",
	}); err != nil {
		t.Fatalf("upsert parent file doc: %v", err)
	}
	// Symbol doc with metadata that lacks line/end_line.
	metaBytes := []byte(`{"source_type":"symbol","kind":"function","language":"go","signature":"func Qux() {}"}`)
	if _, err := q.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: wsHash,
		ContentHash:   "noline-sym-hash",
		Title:         "Qux",
		Content:       "func Qux() {}",
		SourcePath:    "noline.go?symbol=Qux&kind=function",
		Collection:    "code",
		Tags:          []string{"symbol", "go", "function"},
		Metadata:      pqtype.NullRawMessage{RawMessage: metaBytes, Valid: true},
	}); err != nil {
		t.Fatalf("upsert symbol doc: %v", err)
	}

	result := callTool("memory_get", map[string]any{
		"workspace": wsHash,
		"path":      "noline.go?symbol=Qux&kind=function",
	})
	resp := unmarshalGraphResp(t, result)
	if resp["content"].(string) != "func Qux() {}" {
		t.Errorf("content = %q, want the signature (must not dump the whole parent file)", resp["content"])
	}
}

func TestMemoryGet_GraphNodePathReturnsBody(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)

	// A "file::Symbol" node — exactly what memory_trace/memory_graph emit —
	// must be directly re-feedable into memory_get and return the symbol body.
	parentContent := "line1\nline2\nline3\nline4\nline5"
	if _, err := q.UpsertDocument(ctx, sqlc.UpsertDocumentParams{
		WorkspaceHash: wsHash,
		ContentHash:   "node-body-hash",
		Title:         "body.go",
		Content:       parentContent,
		SourcePath:    "body.go",
		Collection:    "code",
	}); err != nil {
		t.Fatalf("upsert parent file doc: %v", err)
	}
	upsertSymbolDoc(t, ctx, q, wsHash, "body.go", "Foo", "function", "func Foo() {}", "2", "4")

	result := callTool("memory_get", map[string]any{
		"workspace": wsHash,
		"path":      "body.go::Foo",
	})
	if result.IsError {
		t.Fatalf("memory_get on graph node errored: %v", result.Content[0].(*mcpsdk.TextContent).Text)
	}
	resp := unmarshalGraphResp(t, result)
	if resp["content"].(string) != "line2\nline3\nline4" {
		t.Errorf("content = %q, want body slice [2:4] resolved from the graph node", resp["content"])
	}

	// Unknown symbol in a real file → clean error, no leaked sql.
	missing := callTool("memory_get", map[string]any{"workspace": wsHash, "path": "body.go::Nope"})
	if !missing.IsError {
		t.Fatal("expected error for unknown symbol node")
	}
	if msg := missing.Content[0].(*mcpsdk.TextContent).Text; strings.Contains(msg, "sql:") {
		t.Errorf("error leaks raw sql: %q", msg)
	}
}

func TestMemoryGet_SymbolPathFallsBackToSignatureWhenParentMissing(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)

	// No parent file document exists for orphan.go. The symbol starts past
	// line 1, so a naive fallback would slice the 1-line signature by [3:5] and
	// return "" — assert we get the signature back intact instead.
	upsertSymbolDoc(t, ctx, q, wsHash, "orphan.go", "Bar", "function", "func Bar() {}", "3", "5")

	result := callTool("memory_get", map[string]any{
		"workspace": wsHash,
		"path":      "orphan.go?symbol=Bar&kind=function",
	})
	if result.IsError {
		t.Fatalf("expected fallback to signature, got error: %v", result.Content[0].(*mcpsdk.TextContent).Text)
	}
	resp := unmarshalGraphResp(t, result)
	if resp["content"].(string) != "func Bar() {}" {
		t.Errorf("content = %q, want fallback signature (not an empty slice)", resp["content"])
	}
}
