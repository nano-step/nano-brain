package bench

import "fmt"

type ComparisonResult struct {
	Passed      bool             `json:"passed"`
	DatasetOK   bool             `json:"dataset_ok"`
	Regressions []Regression     `json:"regressions"`
	Deltas      map[string]Delta `json:"deltas"`
}

type Regression struct {
	Metric    string  `json:"metric"`
	Baseline  float64 `json:"baseline"`
	New       float64 `json:"new"`
	Drop      float64 `json:"drop"`
	Threshold float64 `json:"threshold"`
	Message   string  `json:"message"`
}

type Delta struct {
	Baseline float64 `json:"baseline"`
	New      float64 `json:"new"`
	Change   float64 `json:"change"`
}

const epsilon = 1e-9

func Compare(newResults, baseline *BenchmarkResults) *ComparisonResult {
	result := &ComparisonResult{
		Passed:      true,
		DatasetOK:   true,
		Regressions: []Regression{},
		Deltas:      make(map[string]Delta),
	}

	if baseline.DatasetVersion != newResults.DatasetVersion {
		result.DatasetOK = false
		result.Passed = false
		result.Regressions = append(result.Regressions, Regression{
			Metric:   "DatasetVersion",
			Baseline: 0,
			New:      0,
			Message:  fmt.Sprintf("dataset version mismatch: baseline=%s, current=%s", baseline.DatasetVersion, newResults.DatasetVersion),
		})
		return result
	}

	checkMetric := func(name string, baseVal, newVal float64, threshold float64, higherIsBetter bool) {
		change := newVal - baseVal
		if !higherIsBetter {
			change = baseVal - newVal
		}
		result.Deltas[name] = Delta{Baseline: baseVal, New: newVal, Change: change}

		drop := baseVal - newVal
		if !higherIsBetter {
			drop = newVal - baseVal
		}
		if drop > threshold+epsilon {
			result.Passed = false
			result.Regressions = append(result.Regressions, Regression{
				Metric:    name,
				Baseline:  baseVal,
				New:       newVal,
				Drop:      drop,
				Threshold: threshold,
				Message:   fmt.Sprintf("%s dropped by %.4f (threshold: %.4f)", name, drop, threshold),
			})
		}
	}

	checkMetric("P@5 (paths)", baseline.PrecisionAt5Paths, newResults.PrecisionAt5Paths, 0.10, true)
	checkMetric("R@10 (paths)", baseline.RecallAt10Paths, newResults.RecallAt10Paths, 0.10, true)
	checkMetric("MRR (paths)", baseline.MRRPaths, newResults.MRRPaths, 0.05, true)

	p95Ratio := 2.0
	result.Deltas["Query P95"] = Delta{
		Baseline: baseline.QueryP95ms,
		New:      newResults.QueryP95ms,
		Change:   baseline.QueryP95ms - newResults.QueryP95ms,
	}
	if newResults.QueryP95ms > p95Ratio*baseline.QueryP95ms+epsilon {
		result.Passed = false
		ratio := 0.0
		if baseline.QueryP95ms > 0 {
			ratio = newResults.QueryP95ms / baseline.QueryP95ms
		}
		result.Regressions = append(result.Regressions, Regression{
			Metric:    "Query P95",
			Baseline:  baseline.QueryP95ms,
			New:       newResults.QueryP95ms,
			Drop:      ratio,
			Threshold: p95Ratio,
			Message:   fmt.Sprintf("Query P95 increased by %.2fx (threshold: 2.00x)", ratio),
		})
	}

	result.Deltas["Query P50"] = Delta{
		Baseline: baseline.QueryP50ms,
		New:      newResults.QueryP50ms,
		Change:   baseline.QueryP50ms - newResults.QueryP50ms,
	}

	return result
}
