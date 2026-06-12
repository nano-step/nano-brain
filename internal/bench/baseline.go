package bench

var NanoBrainBaselineV1 = &BenchmarkResults{
	Version:           "v0.3.0-dedup+ranking",
	DatasetVersion:    NanoBrainDatasetVersion,
	Scale:             20,
	WorkspaceHash:     "7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f",
	PrecisionAt5:      0.0,
	RecallAt10:        0.0,
	MRR:               0.0,
	PrecisionAt5Paths: 0.090,
	RecallAt10Paths:   0.662,
	MRRPaths:          0.172,
	QueryP50ms:        5695.4,
	QueryP95ms:        10242.9,
	QueryCount:        20,
}
