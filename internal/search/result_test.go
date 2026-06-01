package search

import (
	"encoding/json"
	"testing"
	"time"
)

func TestResult_JSONFieldsUseSnakeCase(t *testing.T) {
	r := Result{
		ID:            "uuid-1",
		DocumentID:    "doc-2",
		WorkspaceHash: "ws-3",
		Title:         "test title",
		Snippet:       "snip",
		Content:       "body",
		Score:         0.75,
		Tags:          []string{"a"},
		Collection:    "memory",
		SourcePath:    "memory://x",
		CreatedAt:     time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	wantKeys := []string{
		"id", "document_id", "workspace_hash", "title", "snippet",
		"content", "score", "tags", "collection", "source_path",
		"created_at", "updated_at",
	}
	for _, k := range wantKeys {
		if _, ok := m[k]; !ok {
			t.Errorf("expected snake_case key %q in JSON map, got keys: %v", k, mapKeys(m))
		}
	}
	forbiddenKeys := []string{"ID", "DocumentID", "WorkspaceHash", "Title", "Snippet", "Content", "Score", "Tags", "Collection", "SourcePath", "CreatedAt", "UpdatedAt"}
	for _, k := range forbiddenKeys {
		if _, ok := m[k]; ok {
			t.Errorf("forbidden PascalCase key %q present in JSON map (issue #303 regression)", k)
		}
	}
}

func mapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
