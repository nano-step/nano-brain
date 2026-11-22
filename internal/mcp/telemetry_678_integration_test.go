//go:build integration

package mcp_test

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nano-brain/nano-brain/internal/storage"
)

type toolTelemetryRow struct {
	workspace   string
	query       string
	resultCount int
	payload     struct {
		Tool        string         `json:"tool"`
		IsError     bool           `json:"is_error"`
		Args        map[string]any `json:"args"`
		ResultCount *int           `json:"result_count"`
	}
}

// waitToolTelemetry polls for the mcp_tool row of one tool; rows are written
// asynchronously after the tool call returns.
func waitToolTelemetry(t *testing.T, db *sql.DB, tool string) toolTelemetryRow {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var row toolTelemetryRow
		var payload []byte
		err := db.QueryRow(`SELECT workspace_hash, query_text, result_count, payload FROM telemetry_logs
			WHERE event_type = 'mcp_tool' AND payload->>'tool' = $1
			ORDER BY created_at DESC, ctid DESC LIMIT 1`, tool).
			Scan(&row.workspace, &row.query, &row.resultCount, &payload)
		if err == nil {
			if err := json.Unmarshal(payload, &row.payload); err != nil {
				t.Fatalf("payload %s: %v", payload, err)
			}
			return row
		}
		if err != sql.ErrNoRows {
			t.Fatalf("query telemetry_logs: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("no mcp_tool telemetry row for %s", tool)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Issue #678: MCP tool calls left no row in telemetry_logs.
func TestToolCallsRecordTelemetry(t *testing.T) {
	_, db, _, wsHash, callTool := setupSearchMCP(t)
	// The test database is shared; drop stale mcp_tool rows so waits below
	// only see rows written by this test.
	if _, err := db.Exec(`DELETE FROM telemetry_logs WHERE event_type = 'mcp_tool'`); err != nil {
		t.Fatalf("clean telemetry_logs: %v", err)
	}

	t.Run("search with no hits, workspace given by name", func(t *testing.T) {
		callTool("memory_search", map[string]any{"workspace": "test-ws", "query": "zzznothing"})
		row := waitToolTelemetry(t, db, "memory_search")
		// The shared test DB accumulates workspaces named "test-ws" and
		// GetWorkspaceByName has no ORDER BY, so which hash is returned is
		// arbitrary — assert only that the name was resolved to hash form.
		if len(row.workspace) != 64 || !storage.IsHex(row.workspace) {
			t.Errorf("workspace = %q, want a resolved 64-char hash", row.workspace)
		}
		if row.query != "zzznothing" || row.payload.Args["query"] != "zzznothing" {
			t.Errorf("query = %q, args = %v", row.query, row.payload.Args)
		}
		if row.payload.IsError || row.payload.ResultCount == nil || *row.payload.ResultCount != 0 || row.resultCount != 0 {
			t.Errorf("is_error = %v, result_count = %v / %d, want a counted 0", row.payload.IsError, row.payload.ResultCount, row.resultCount)
		}
	})

	t.Run("tool error", func(t *testing.T) {
		res := callTool("memory_get", map[string]any{"workspace": wsHash, "path": "missing/doc.md"})
		if !res.IsError {
			t.Fatal("expected memory_get on a missing path to be a tool error")
		}
		row := waitToolTelemetry(t, db, "memory_get")
		if !row.payload.IsError || row.payload.ResultCount != nil {
			t.Errorf("is_error = %v, result_count = %v, want error without a count", row.payload.IsError, row.payload.ResultCount)
		}
	})

	t.Run("long argument is truncated", func(t *testing.T) {
		content := strings.Repeat("x", 5000)
		callTool("memory_write", map[string]any{"workspace": wsHash, "content": content, "title": "t"})
		row := waitToolTelemetry(t, db, "memory_write")
		got, _ := row.payload.Args["content"].(string)
		if len(got) >= len(content) || !strings.HasPrefix(content, strings.TrimSuffix(got, "…")) {
			t.Errorf("content arg stored with %d bytes, want a truncated prefix", len(got))
		}
	})

	t.Run("unresolvable workspace stores an empty hash", func(t *testing.T) {
		callTool("memory_query", map[string]any{"workspace": "no-such-workspace-678", "query": "x"})
		row := waitToolTelemetry(t, db, "memory_query")
		if row.workspace != "" {
			t.Errorf("workspace = %q, want empty for an unresolvable workspace", row.workspace)
		}
		if row.payload.Args["workspace"] != "no-such-workspace-678" {
			t.Errorf("args.workspace = %v, want the raw input preserved", row.payload.Args["workspace"])
		}
	})

	t.Run("tool without a workspace", func(t *testing.T) {
		callTool("memory_workspaces_list", map[string]any{})
		row := waitToolTelemetry(t, db, "memory_workspaces_list")
		if row.workspace != "" || row.payload.IsError {
			t.Errorf("workspace = %q, is_error = %v", row.workspace, row.payload.IsError)
		}
		if row.payload.ResultCount == nil || *row.payload.ResultCount < 1 {
			t.Errorf("result_count = %v, want at least 1 workspace", row.payload.ResultCount)
		}
	})
}
