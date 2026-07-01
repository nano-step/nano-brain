package mcp

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/migrations"
	"github.com/pressly/goose/v3"
)

// mcpInternalTestDSN mirrors tools_security_test.go's mcpSecTestDSN — kept
// as a private duplicate here since that helper lives in package mcp_test
// and this file needs Postgres access from within package mcp (to reach the
// unexported ctxKeyDefaultWorkspace type).
func mcpInternalTestDSN() string {
	if v := os.Getenv("NANO_BRAIN_TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_test?sslmode=disable"
}

// setupInternalTestPG mirrors tools_security_test.go's setupMCPSecTestPG —
// an isolated schema per test against the nanobrain_test Postgres instance.
func setupInternalTestPG(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping postgres-dependent test in -short mode")
	}

	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(mcpInternalTestDSN())
	if err != nil {
		t.Skip("postgres not available: " + err.Error())
	}

	schema := fmt.Sprintf("test_mcpi_%x", sha256.Sum256([]byte(t.Name())))[:19]
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s, public", schema))
		return err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Skip("postgres not available: " + err.Error())
	}

	_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Skip("postgres not available: " + err.Error())
	}

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	goose.SetTableName(schema + "_goose_version")
	migrateDB := stdlib.OpenDBFromPool(pool)
	if err := goose.UpContext(ctx, migrateDB, "."); err != nil {
		migrateDB.Close()
		pool.Close()
		t.Fatal(err)
	}
	migrateDB.Close()
	goose.SetTableName("goose_db_version")

	pgDB := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() {
		pgDB.Close()
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	return pgDB
}

func ticketRow(workspaceHash, title, sourcePath, content string) sqlc.ListDocumentsByTagRow {
	return sqlc.ListDocumentsByTagRow{
		ID:            uuid.New(),
		WorkspaceHash: workspaceHash,
		Title:         title,
		SourcePath:    sourcePath,
		Collection:    "sessions",
		Tags:          []string{"ticket:DEV-4706"},
		Content:       content,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

// TestFormatTicketSessions_CrossWorkspace: rows from two distinct workspaces
// both appear in the rendered markdown, proving the result is not scoped to a
// single workspace. Source is derived from the source_path scheme.
func TestFormatTicketSessions_CrossWorkspace(t *testing.T) {
	rows := []sqlc.ListDocumentsByTagRow{
		ticketRow("ws-aaaaaaa1", "Session A", "summary://claude/sess-1", "Worked on DEV-4706 in A"),
		ticketRow("ws-bbbbbbb2", "Session B", "summary://opencode/sess-2", "Worked on DEV-4706 in B"),
	}

	out := formatTicketSessions("DEV-4706", rows)

	if !strings.Contains(out, "## Sessions for ticket DEV-4706") {
		t.Errorf("missing header, got:\n%s", out)
	}
	if !strings.Contains(out, "Session A") || !strings.Contains(out, "Session B") {
		t.Errorf("expected both sessions, got:\n%s", out)
	}
	// Both workspaces present (truncated to 8 chars).
	if !strings.Contains(out, "ws-aaaaa") || !strings.Contains(out, "ws-bbbbb") {
		t.Errorf("expected both workspace hashes, got:\n%s", out)
	}
	// Source derived from path scheme.
	if !strings.Contains(out, "`claude`") || !strings.Contains(out, "`opencode`") {
		t.Errorf("expected both sources, got:\n%s", out)
	}
}

// TestFormatTicketSessions_Unknown: empty result set returns the "no sessions"
// message rather than an empty list or error.
func TestFormatTicketSessions_Unknown(t *testing.T) {
	out := formatTicketSessions("DEV-9999", nil)
	if out != "No sessions found for ticket DEV-9999." {
		t.Errorf("expected no-sessions message, got %q", out)
	}
}

func TestOmitEmptyTags(t *testing.T) {
	item := mcpSearchResultItem{ID: "test-id", Title: "test", Tags: nil}
	data, _ := json.Marshal(item)
	if strings.Contains(string(data), `"tags"`) {
		t.Error("nil tags should be omitted")
	}

	item.Tags = []string{}
	data, _ = json.Marshal(item)
	if strings.Contains(string(data), `"tags"`) {
		t.Error("empty tags should be omitted")
	}

	item.Tags = []string{"foo"}
	data, _ = json.Marshal(item)
	if !strings.Contains(string(data), `"tags":["foo"]`) {
		t.Error("non-empty tags should be present")
	}
}

func TestOmitWorkspaceHash(t *testing.T) {
	item := mcpSearchResultItem{ID: "test-id", WorkspaceHash: ""}
	data, _ := json.Marshal(item)
	if strings.Contains(string(data), `"workspace_hash"`) {
		t.Error("empty workspace_hash should be omitted")
	}
}

func TestEpochTimestamps(t *testing.T) {
	now := time.Now()
	item := mcpSearchResultItem{ID: "test-id", CreatedAt: now.Unix(), UpdatedAt: now.Unix()}
	data, _ := json.Marshal(item)
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if ca, ok := parsed["created_at"]; ok {
		if _, isNum := ca.(float64); !isNum {
			t.Errorf("epoch: expected numeric created_at, got %T", ca)
		}
	}

	item2 := mcpSearchResultItem{ID: "test-id", CreatedAt: now, UpdatedAt: now}
	data2, _ := json.Marshal(item2)
	var parsed2 map[string]any
	if err := json.Unmarshal(data2, &parsed2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if ca, ok := parsed2["created_at"]; ok {
		if _, isStr := ca.(string); !isStr {
			t.Errorf("rfc3339: expected string created_at, got %T", ca)
		}
	}
}

func TestFilterFields(t *testing.T) {
	item := mcpSearchResultItem{
		ID: "test-id", Title: "hello", Snippet: "snip",
		Score: 0.95, Collection: "memory", SourcePath: "/foo",
	}
	fieldSet := map[string]bool{"title": true, "score": true}
	filtered := filterFields(item, fieldSet)

	if filtered["id"] != "test-id" {
		t.Error("id always included")
	}
	if filtered["title"] != "hello" {
		t.Error("title requested but missing")
	}
	if filtered["score"] != 0.95 {
		t.Error("score requested but missing")
	}
	if _, exists := filtered["snippet"]; exists {
		t.Error("snippet not requested but present")
	}
	if _, exists := filtered["collection"]; exists {
		t.Error("collection not requested but present")
	}
}

// TestRequireWorkspace_ExplicitArgWins: an explicit "workspace" arg always
// wins over a context-injected default (D-03). Both values resolve via the
// "all" special-case (no DB round-trip needed), so this stays -short-safe;
// the explicit arg "all" must be what's returned, not the context default.
func TestRequireWorkspace_ExplicitArgWins(t *testing.T) {
	a := &Adapter{}
	ctx := context.WithValue(context.Background(), ctxKeyDefaultWorkspace{}, "ws-ctx-default")
	args := map[string]any{"workspace": "all"}

	ws, errRes := a.requireWorkspace(ctx, args)
	if errRes != nil {
		t.Fatalf("expected no error, got %v", errRes)
	}
	if ws != "all" {
		t.Errorf("expected explicit arg %q to win over context default, got %q", "all", ws)
	}
}

// TestRequireWorkspace_ContextFallback: when the "workspace" arg is omitted,
// requireWorkspace falls back to the context-injected default (D-01). The
// context default is "all" here purely so resolution short-circuits without
// a DB round-trip; the assertion only cares that the "workspace is required"
// error is NOT returned, proving the context value was picked up as input.
func TestRequireWorkspace_ContextFallback(t *testing.T) {
	a := &Adapter{}
	ctx := context.WithValue(context.Background(), ctxKeyDefaultWorkspace{}, "all")
	args := map[string]any{}

	ws, errRes := a.requireWorkspace(ctx, args)
	if errRes != nil {
		t.Fatalf("expected context fallback to be used, got error: %v", errRes)
	}
	if ws != "all" {
		t.Errorf("expected context default to be resolved, got %q", ws)
	}
}

// TestRequireWorkspace_NoArgNoDefaultErrors: when neither the explicit arg
// nor a context default is present, requireWorkspace returns the exact same
// "workspace is required" error as before the fallback was added (D-04).
func TestRequireWorkspace_NoArgNoDefaultErrors(t *testing.T) {
	a := &Adapter{}
	ctx := context.Background()
	args := map[string]any{}

	ws, errRes := a.requireWorkspace(ctx, args)
	if errRes == nil {
		t.Fatalf("expected error, got success with workspace %q", ws)
	}
	if len(errRes.Content) == 0 {
		t.Fatal("expected error content")
	}
	text := errRes.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "workspace is required") {
		t.Errorf("expected \"workspace is required\" error text, got %q", text)
	}
}

// TestWrapStreamableHandler_RejectsAllAsDefault proves D-02: a connection
// configured with `?workspace=all` must never let that value become the
// per-connection default, since requireWorkspace's own "all" special-case
// would otherwise apply to the fallback too — silently turning every
// omitted-arg tool call on such a connection into a cross-workspace query.
// "all" must stay reachable only as an explicit per-call argument.
func TestWrapStreamableHandler_RejectsAllAsDefault(t *testing.T) {
	var gotDefault any
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotDefault = r.Context().Value(ctxKeyDefaultWorkspace{})
	})

	cases := []struct {
		name      string
		query     string
		wantValue any
	}{
		{"all is rejected as a default", "?workspace=all", nil},
		{"empty is treated as absent", "?workspace=", nil},
		{"a real name is accepted as a default", "?workspace=nano-brain", "nano-brain"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotDefault = nil
			req := httptest.NewRequest(http.MethodPost, "/mcp"+tc.query, nil)
			rec := httptest.NewRecorder()
			WrapStreamableHandler(inner).ServeHTTP(rec, req)
			if gotDefault != tc.wantValue {
				t.Errorf("expected context default %v, got %v", tc.wantValue, gotDefault)
			}
		})
	}
}

// TestRequireRegisteredWorkspace_UsesConnectionDefault proves D-05 write-path
// parity: requireRegisteredWorkspace (used by memory_write/memory_update)
// honors the connection-default context value the same way requireWorkspace
// does, and still enforces the registration check (GetWorkspaceByHash)
// against the resolved workspace — a connection default cannot write to an
// unregistered workspace. Also proves D-04 write-path: no default + no arg
// still returns "workspace is required".
func TestRequireRegisteredWorkspace_UsesConnectionDefault(t *testing.T) {
	pgDB := setupInternalTestPG(t)
	q := sqlc.New(pgDB)
	a := &Adapter{queries: q}

	wsName := "require-registered-ctx-default"
	wsHash := hex.EncodeToString(sha256.New().Sum([]byte(wsName)))
	if _, err := q.UpsertWorkspace(context.Background(), sqlc.UpsertWorkspaceParams{
		Hash: wsHash,
		Name: wsName,
		Path: "/tmp/" + wsName,
	}); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}

	t.Run("context default resolves to registered workspace", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), ctxKeyDefaultWorkspace{}, wsName)
		ws, errRes := requireRegisteredWorkspace(ctx, a, map[string]any{})
		if errRes != nil {
			text := errRes.Content[0].(*mcpsdk.TextContent).Text
			t.Fatalf("expected success via connection default, got error: %s", text)
		}
		if ws != wsHash {
			t.Errorf("expected resolved hash %q, got %q", wsHash, ws)
		}
	})

	t.Run("no default and no arg still requires workspace", func(t *testing.T) {
		ctx := context.Background()
		ws, errRes := requireRegisteredWorkspace(ctx, a, map[string]any{})
		if errRes == nil {
			t.Fatalf("expected error, got success with workspace %q", ws)
		}
		text := errRes.Content[0].(*mcpsdk.TextContent).Text
		if !strings.Contains(text, "workspace is required") {
			t.Errorf("expected \"workspace is required\", got: %s", text)
		}
	})
}

func TestPaginatedResponseOmitsTotal(t *testing.T) {
	total := 42
	qms := int64(15)
	resp := mcpSearchResponse{
		Results: []mcpSearchResultItem{},
		Total:   &total,
		QueryMs: &qms,
	}
	data, _ := json.Marshal(resp)
	if !strings.Contains(string(data), `"total":42`) {
		t.Error("first page should include total")
	}
	if !strings.Contains(string(data), `"query_ms":15`) {
		t.Error("first page should include query_ms")
	}

	resp2 := mcpSearchResponse{Results: []mcpSearchResultItem{}, Total: nil, QueryMs: nil}
	data2, _ := json.Marshal(resp2)
	if strings.Contains(string(data2), `"total"`) {
		t.Error("page 2+ should omit total")
	}
	if strings.Contains(string(data2), `"query_ms"`) {
		t.Error("page 2+ should omit query_ms")
	}
}
