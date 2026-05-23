package bench

import (
	"fmt"
)

// ComparisonResult holds the output of comparing two benchmark runs.
type ComparisonResult struct {
	Passed      bool             `json:"passed"`
	Regressions []Regression     `json:"regressions"`
	Deltas      map[string]Delta `json:"deltas"`
}

// Regression describes a metric that failed its threshold check.
type Regression struct {
	Metric    string  `json:"metric"`
	Baseline  float64 `json:"baseline"`
	New       float64 `json:"new"`
	Drop      float64 `json:"drop"`
	Threshold float64 `json:"threshold"`
	Message   string  `json:"message"`
}

// Delta describes the change in a metric between two runs.
type Delta struct {
	Baseline float64 `json:"baseline"`
	New      float64 `json:"new"`
	Change   float64 `json:"change"` // positive = improvement
}

// Compare takes new and baseline benchmark results and returns a ComparisonResult.
// Thresholds (from spec):
// - P@5: drop > 0.10 from baseline → regression (higher is better)
// - R@10: drop > 0.10 from baseline → regression (higher is better)
// - MRR: drop > 0.05 from baseline → regression (higher is better)
// - Query P95: new > 2x baseline → regression (lower is better)
// Uses epsilon comparison for floating-point thresholds.
const epsilon = 1e-9

func Compare(newResults, baseline *BenchmarkResults) *ComparisonResult {
	result := &ComparisonResult{
		Passed:      true,
		Regressions: []Regression{},
		Deltas:      make(map[string]Delta),
	}

	// P@5: higher is better, drop > 0.10 is regression
	p5Drop := baseline.PrecisionAt5 - newResults.PrecisionAt5
	result.Deltas["P@5"] = Delta{
		Baseline: baseline.PrecisionAt5,
		New:      newResults.PrecisionAt5,
		Change:   newResults.PrecisionAt5 - baseline.PrecisionAt5,
	}
	if p5Drop > 0.10+epsilon {
		result.Passed = false
		result.Regressions = append(result.Regressions, Regression{
			Metric:    "P@5",
			Baseline:  baseline.PrecisionAt5,
			New:       newResults.PrecisionAt5,
			Drop:      p5Drop,
			Threshold: 0.10,
			Message:   fmt.Sprintf("P@5 dropped by %.4f (threshold: 0.10)", p5Drop),
		})
	}

	// R@10: higher is better, drop > 0.10 is regression
	r10Drop := baseline.RecallAt10 - newResults.RecallAt10
	result.Deltas["R@10"] = Delta{
		Baseline: baseline.RecallAt10,
		New:      newResults.RecallAt10,
		Change:   newResults.RecallAt10 - baseline.RecallAt10,
	}
	if r10Drop > 0.10+epsilon {
		result.Passed = false
		result.Regressions = append(result.Regressions, Regression{
			Metric:    "R@10",
			Baseline:  baseline.RecallAt10,
			New:       newResults.RecallAt10,
			Drop:      r10Drop,
			Threshold: 0.10,
			Message:   fmt.Sprintf("R@10 dropped by %.4f (threshold: 0.10)", r10Drop),
		})
	}

	// MRR: higher is better, drop > 0.05 is regression
	mrrDrop := baseline.MRR - newResults.MRR
	result.Deltas["MRR"] = Delta{
		Baseline: baseline.MRR,
		New:      newResults.MRR,
		Change:   newResults.MRR - baseline.MRR,
	}
	if mrrDrop > 0.05+epsilon {
		result.Passed = false
		result.Regressions = append(result.Regressions, Regression{
			Metric:    "MRR",
			Baseline:  baseline.MRR,
			New:       newResults.MRR,
			Drop:      mrrDrop,
			Threshold: 0.05,
			Message:   fmt.Sprintf("MRR dropped by %.4f (threshold: 0.05)", mrrDrop),
		})
	}

	// Query P95: lower is better, new > 2x baseline is regression
	p95Ratio := 2.0
	result.Deltas["Query P95"] = Delta{
		Baseline: baseline.QueryP95ms,
		New:      newResults.QueryP95ms,
		Change:   baseline.QueryP95ms - newResults.QueryP95ms,
	}
	if newResults.QueryP95ms > p95Ratio*baseline.QueryP95ms+epsilon {
		result.Passed = false
		result.Regressions = append(result.Regressions, Regression{
			Metric:    "Query P95",
			Baseline:  baseline.QueryP95ms,
			New:       newResults.QueryP95ms,
			Drop:      newResults.QueryP95ms / baseline.QueryP95ms,
			Threshold: p95Ratio,
			Message:   fmt.Sprintf("Query P95 increased by %.2fx (threshold: 2.00x)", newResults.QueryP95ms/baseline.QueryP95ms),
		})
	}

	// Query P50: lower is better, for completeness (no threshold check)
	result.Deltas["Query P50"] = Delta{
		Baseline: baseline.QueryP50ms,
		New:      newResults.QueryP50ms,
		Change:   baseline.QueryP50ms - newResults.QueryP50ms, // positive = improvement (lower is better)
	}

	return result
}
