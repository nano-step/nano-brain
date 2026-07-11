package bench

import (
	"context"
	"fmt"
	"time"

	"github.com/nano-brain/nano-brain/internal/search"
)

type Searcher interface {
	HybridSearch(ctx context.Context, query string, workspace string, maxResults int, tags []string, timeRange *search.TimeRangeFilter, chunkType string, hypothetical string) ([]search.Result, error)
}

func Run(ctx context.Context, dataset *BenchmarkDataset, searcher Searcher, version string) (*BenchmarkResults, error) {
	if dataset == nil || len(dataset.Entries) == 0 {
		return nil, fmt.Errorf("dataset is empty")
	}

	queryResults := make([]QueryResult, 0, len(dataset.Entries))
	latencies := make([]float64, 0, len(dataset.Entries))

	for _, entry := range dataset.Entries {
		start := time.Now()
		results, err := searcher.HybridSearch(ctx, entry.Query, dataset.WorkspaceHash, 10, nil, nil, "", "")
		elapsed := time.Since(start).Seconds() * 1000

		if err != nil {
			return nil, fmt.Errorf("search failed for query %q: %w", entry.Query, err)
		}

		returnedIDs := make([]string, len(results))
		returnedPaths := make([]string, len(results))
		for i, r := range results {
			returnedIDs[i] = r.DocumentID
			returnedPaths[i] = r.SourcePath
		}

		queryResults = append(queryResults, QueryResult{
			Query:               entry.Query,
			RelevantDocIDs:      entry.RelevantDocIDs,
			RelevantSourcePaths: entry.RelevantSourcePaths,
			ReturnedDocIDs:      returnedIDs,
			ReturnedSourcePaths: returnedPaths,
			LatencyMs:           elapsed,
		})
		latencies = append(latencies, elapsed)
	}

	return &BenchmarkResults{
		DatasetVersion:    dataset.Version,
		Scale:             dataset.Scale,
		WorkspaceHash:     dataset.WorkspaceHash,
		Timestamp:         time.Now().UTC().Format(time.RFC3339),
		Version:           version,
		PrecisionAt5:      PrecisionAtK(queryResults, 5),
		RecallAt10:        RecallAtK(queryResults, 10),
		MRR:               MeanReciprocalRank(queryResults),
		PrecisionAt5Paths: PrecisionAtKPaths(queryResults, 5),
		RecallAt10Paths:   RecallAtKPaths(queryResults, 10),
		MRRPaths:          MRRPaths(queryResults),
		QueryP50ms:        Percentile(latencies, 50),
		QueryP95ms:        Percentile(latencies, 95),
		QueryCount:        len(queryResults),
	}, nil
}
