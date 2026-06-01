package search

import (
	"encoding/json"
	"strings"
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
	s := string(raw)
	wantKeys := []string{
		`"id"`, `"document_id"`, `"workspace_hash"`, `"title"`, `"snippet"`,
		`"content"`, `"score"`, `"tags"`, `"collection"`, `"source_path"`,
		`"created_at"`, `"updated_at"`,
	}
	for _, k := range wantKeys {
		if !strings.Contains(s, k) {
			t.Errorf("expected snake_case key %s in JSON, got: %s", k, s)
		}
	}
	forbiddenKeys := []string{
		`"ID"`, `"DocumentID"`, `"WorkspaceHash"`, `"Title"`,
	}
	for _, k := range forbiddenKeys {
		if strings.Contains(s, k) {
			t.Errorf("forbidden PascalCase key %s present in JSON (issue #303 regression): %s", k, s)
		}
	}
}
