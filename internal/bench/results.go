package bench

// BenchmarkResults holds the output of a benchmark run.
type BenchmarkResults struct {
	Scale         int    `json:"scale"`
	WorkspaceHash string `json:"workspace_hash"`
	Timestamp     string `json:"timestamp"`
	Version       string `json:"version"`

	// Quality metrics
	PrecisionAt5 float64 `json:"precision_at_5"`
	RecallAt10   float64 `json:"recall_at_10"`
	MRR          float64 `json:"mrr"`

	// Latency percentiles (milliseconds)
	QueryP50ms float64 `json:"query_p50_ms"`
	QueryP95ms float64 `json:"query_p95_ms"`

	// Per-query detail
	QueryCount int `json:"query_count"`
}
