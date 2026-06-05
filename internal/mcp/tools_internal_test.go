package mcp

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

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
	json.Unmarshal(data, &parsed)
	if ca, ok := parsed["created_at"]; ok {
		if _, isNum := ca.(float64); !isNum {
			t.Errorf("epoch: expected numeric created_at, got %T", ca)
		}
	}

	item2 := mcpSearchResultItem{ID: "test-id", CreatedAt: now, UpdatedAt: now}
	data2, _ := json.Marshal(item2)
	var parsed2 map[string]any
	json.Unmarshal(data2, &parsed2)
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
