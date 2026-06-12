package bench

type BenchmarkResults struct {
	DatasetVersion  string  `json:"dataset_version"`
	Scale           int     `json:"scale"`
	WorkspaceHash   string  `json:"workspace_hash"`
	Timestamp       string  `json:"timestamp"`
	Version         string  `json:"version"`
	PrecisionAt5    float64 `json:"precision_at_5"`
	RecallAt10      float64 `json:"recall_at_10"`
	MRR             float64 `json:"mrr"`
	PrecisionAt5Paths float64 `json:"precision_at_5_paths"`
	RecallAt10Paths   float64 `json:"recall_at_10_paths"`
	MRRPaths          float64 `json:"mrr_paths"`
	QueryP50ms      float64 `json:"query_p50_ms"`
	QueryP95ms      float64 `json:"query_p95_ms"`
	QueryCount      int     `json:"query_count"`
}
