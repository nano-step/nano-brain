package bench

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

type mockStore struct {
	docs []DocumentRow
	err  error
}

func (m *mockStore) ListDocumentsByWorkspace(_ context.Context, _ string) ([]DocumentRow, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.docs, nil
}

func (m *mockStore) GetDocumentByID(_ context.Context, _ uuid.UUID, _ string) (*DocumentRow, error) {
	if m.err != nil {
		return nil, m.err
	}
	return nil, fmt.Errorf("not implemented in mock")
}

func makeDocs(n int) []DocumentRow {
	docs := make([]DocumentRow, n)
	for i := range docs {
		docs[i] = DocumentRow{
			ID:            uuid.New(),
			WorkspaceHash: "ws-test",
			ContentHash:   fmt.Sprintf("hash-%d", i),
			Title:         fmt.Sprintf("Document %d", i),
			SourcePath:    fmt.Sprintf("/path/to/doc-%d.md", i),
			Collection:    "test-collection",
		}
	}
	return docs
}

func TestGenerate_Success(t *testing.T) {
	docs := makeDocs(10)
	store := &mockStore{docs: docs}

	ds, err := Generate(context.Background(), store, "ws-test", 5, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ds.Scale != 5 {
		t.Errorf("scale = %d, want 5", ds.Scale)
	}
	if ds.WorkspaceHash != "ws-test" {
		t.Errorf("workspace_hash = %q, want %q", ds.WorkspaceHash, "ws-test")
	}
	if len(ds.Entries) != 5 {
		t.Errorf("entries count = %d, want 5", len(ds.Entries))
	}
	for _, e := range ds.Entries {
		if e.Query == "" {
			t.Error("entry has empty query")
		}
		if len(e.RelevantDocIDs) != 1 {
			t.Errorf("relevant_doc_ids count = %d, want 1", len(e.RelevantDocIDs))
		}
		if e.SourceDocID == "" {
			t.Error("entry has empty source_doc_id")
		}
	}
}

func TestGenerate_InsufficientDocuments(t *testing.T) {
	store := &mockStore{docs: makeDocs(3)}

	_, err := Generate(context.Background(), store, "ws-test", 10, nil)
	if err == nil {
		t.Fatal("expected error for insufficient documents")
	}
}

func TestGenerate_InvalidScale(t *testing.T) {
	store := &mockStore{docs: makeDocs(10)}

	_, err := Generate(context.Background(), store, "ws-test", 0, nil)
	if err == nil {
		t.Fatal("expected error for scale=0")
	}
	_, err = Generate(context.Background(), store, "ws-test", -1, nil)
	if err == nil {
		t.Fatal("expected error for scale=-1")
	}
}

func TestGenerate_StoreError(t *testing.T) {
	store := &mockStore{err: fmt.Errorf("db connection failed")}

	_, err := Generate(context.Background(), store, "ws-test", 5, nil)
	if err == nil {
		t.Fatal("expected error from store failure")
	}
}

func TestGenerate_ListError(t *testing.T) {
	store := &mockStore{err: fmt.Errorf("list query failed")}

	_, err := Generate(context.Background(), store, "ws-test", 1, nil)
	if err == nil {
		t.Fatal("expected error from list failure")
	}
}

func TestGenerate_EmptyTitleAndPath(t *testing.T) {
	id := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	store := &mockStore{docs: []DocumentRow{{ID: id, WorkspaceHash: "ws-test", ContentHash: "h1"}}}

	ds, err := Generate(context.Background(), store, "ws-test", 1, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ds.Entries) != 1 {
		t.Fatalf("entries count = %d, want 1", len(ds.Entries))
	}
	if ds.Entries[0].Query != id.String() {
		t.Errorf("query = %q, want doc ID %q", ds.Entries[0].Query, id.String())
	}
}

func TestGenerate_Deterministic(t *testing.T) {
	docs := makeDocs(20)
	store := &mockStore{docs: docs}

	ds1, err := Generate(context.Background(), store, "ws-test", 10, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	store2 := &mockStore{docs: docs}
	ds2, err := Generate(context.Background(), store2, "ws-test", 10, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := range ds1.Entries {
		if ds1.Entries[i].SourceDocID != ds2.Entries[i].SourceDocID {
			t.Errorf("entry %d: source_doc_id mismatch (not deterministic)", i)
		}
	}
}

func TestDeriveQuery(t *testing.T) {
	tests := []struct {
		name string
		doc  DocumentRow
		want string
	}{
		{
			name: "uses title when present",
			doc:  DocumentRow{Title: "My Document Title", SourcePath: "/path"},
			want: "My Document Title",
		},
		{
			name: "strips Summary prefix from title",
			doc:  DocumentRow{Title: "Summary: setup", SourcePath: "/path"},
			want: "setup",
		},
		{
			name: "strips query params from title",
			doc:  DocumentRow{Title: "handleRequest?symbol=setup&kind=method", SourcePath: "/path"},
			want: "handleRequest",
		},
		{
			name: "uses filename from path when title empty",
			doc:  DocumentRow{Title: "", SourcePath: "/path/to/file.md"},
			want: "file.md",
		},
		{
			name: "falls back to doc ID when both empty",
			doc:  DocumentRow{ID: uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"), Title: "", SourcePath: ""},
			want: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		},
		{
			name: "trims whitespace from title",
			doc:  DocumentRow{Title: "  spaced title  ", SourcePath: "/path"},
			want: "spaced title",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveQuery(tt.doc)
			if got != tt.want {
				t.Errorf("deriveQuery() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBenchmarkDataset_JSONRoundtrip(t *testing.T) {
	ds := BenchmarkDataset{
		Scale:         5,
		WorkspaceHash: "ws-test",
		GeneratedAt:   "2025-01-01T00:00:00Z",
		Entries: []DatasetEntry{
			{
				Query:          "test query",
				RelevantDocIDs: []string{"id-1"},
				SourceDocID:    "id-1",
				SourceTitle:    "Test Doc",
			},
		},
	}

	data, err := json.Marshal(ds)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded BenchmarkDataset
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.Scale != ds.Scale {
		t.Errorf("scale = %d, want %d", decoded.Scale, ds.Scale)
	}
	if decoded.WorkspaceHash != ds.WorkspaceHash {
		t.Errorf("workspace_hash = %q, want %q", decoded.WorkspaceHash, ds.WorkspaceHash)
	}
	if len(decoded.Entries) != 1 {
		t.Fatalf("entries count = %d, want 1", len(decoded.Entries))
	}
	if decoded.Entries[0].Query != "test query" {
		t.Errorf("entry query = %q, want %q", decoded.Entries[0].Query, "test query")
	}
}

func TestBenchmarkResults_JSONRoundtrip(t *testing.T) {
	results := BenchmarkResults{
		Scale:        42,
		WorkspaceHash: "ws-prod-hash",
		Timestamp:    "2025-01-15T12:34:56Z",
		Version:      "v1.2.3",

		// Quality metrics
		PrecisionAt5: 0.8523,
		RecallAt10:   0.7142,
		MRR:          0.6789,

		// Latency percentiles (milliseconds)
		QueryP50ms: 45.67,
		QueryP95ms: 234.56,

		// Per-query detail
		QueryCount: 100,
	}

	// Marshal to JSON
	data, err := json.Marshal(results)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// Unmarshal back
	var decoded BenchmarkResults
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	// Verify all fields match
	if decoded.Scale != results.Scale {
		t.Errorf("scale = %d, want %d", decoded.Scale, results.Scale)
	}
	if decoded.WorkspaceHash != results.WorkspaceHash {
		t.Errorf("workspace_hash = %q, want %q", decoded.WorkspaceHash, results.WorkspaceHash)
	}
	if decoded.Timestamp != results.Timestamp {
		t.Errorf("timestamp = %q, want %q", decoded.Timestamp, results.Timestamp)
	}
	if decoded.Version != results.Version {
		t.Errorf("version = %q, want %q", decoded.Version, results.Version)
	}
	if decoded.PrecisionAt5 != results.PrecisionAt5 {
		t.Errorf("precision_at_5 = %.4f, want %.4f", decoded.PrecisionAt5, results.PrecisionAt5)
	}
	if decoded.RecallAt10 != results.RecallAt10 {
		t.Errorf("recall_at_10 = %.4f, want %.4f", decoded.RecallAt10, results.RecallAt10)
	}
	if decoded.MRR != results.MRR {
		t.Errorf("mrr = %.4f, want %.4f", decoded.MRR, results.MRR)
	}
	if decoded.QueryP50ms != results.QueryP50ms {
		t.Errorf("query_p50_ms = %.2f, want %.2f", decoded.QueryP50ms, results.QueryP50ms)
	}
	if decoded.QueryP95ms != results.QueryP95ms {
		t.Errorf("query_p95_ms = %.2f, want %.2f", decoded.QueryP95ms, results.QueryP95ms)
	}
	if decoded.QueryCount != results.QueryCount {
		t.Errorf("query_count = %d, want %d", decoded.QueryCount, results.QueryCount)
	}
}
