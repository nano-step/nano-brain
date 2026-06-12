// Package bench provides benchmark dataset generation and evaluation tools.
package bench

// DatasetEntry represents a single query-document pair for retrieval evaluation.
type DatasetEntry struct {
	Query              string   `json:"query"`
	RelevantDocIDs     []string `json:"relevant_doc_ids"`
	RelevantSourcePaths []string `json:"relevant_source_paths,omitempty"`
	SourceDocID        string   `json:"source_doc_id"`
	SourceTitle        string   `json:"source_title"`
}

// BenchmarkDataset holds a complete benchmark dataset with metadata.
type BenchmarkDataset struct {
	Version       string         `json:"version"`        // Dataset version (e.g., "v1") — increment when queries change
	Scale         int            `json:"scale"`
	WorkspaceHash string         `json:"workspace_hash"`
	Entries       []DatasetEntry `json:"entries"`
}
