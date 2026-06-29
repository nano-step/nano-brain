package mcp

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

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
