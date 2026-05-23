// Package bench provides benchmark dataset generation and evaluation tools.
package bench

// DatasetEntry represents a single query-document pair for retrieval evaluation.
type DatasetEntry struct {
	Query          string   `json:"query"`
	RelevantDocIDs []string `json:"relevant_doc_ids"`
	SourceDocID    string   `json:"source_doc_id"`
	SourceTitle    string   `json:"source_title"`
}

// BenchmarkDataset holds a complete benchmark dataset with metadata.
type BenchmarkDataset struct {
	Scale         int            `json:"scale"`
	WorkspaceHash string         `json:"workspace_hash"`
	GeneratedAt   string         `json:"generated_at"`
	Entries       []DatasetEntry `json:"entries"`
}
