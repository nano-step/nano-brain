package bench

import (
	"math"
	"testing"
)

func almostEqualDelta(a, b, tolerance float64) bool {
	return math.Abs(a-b) < tolerance
}

func TestCompareAllMetricsWithinThreshold(t *testing.T) {
	baseline := &BenchmarkResults{
		PrecisionAt5: 0.8,
		RecallAt10:   0.75,
		MRR:          0.9,
		QueryP50ms:   50.0,
		QueryP95ms:   100.0,
	}
	newResults := &BenchmarkResults{
		PrecisionAt5: 0.79,
		RecallAt10:   0.74,
		MRR:          0.89,
		QueryP50ms:   51.0,
		QueryP95ms:   101.0,
	}
	result := Compare(newResults, baseline)
	if !result.Passed {
		t.Errorf("expected Passed=true, got %v", result.Passed)
	}
	if len(result.Regressions) != 0 {
		t.Errorf("expected 0 regressions, got %d", len(result.Regressions))
	}
}

func TestComparePrecisionAt5Exactly10DropBoundary(t *testing.T) {
	baseline := &BenchmarkResults{
		PrecisionAt5: 0.80,
		RecallAt10:   0.75,
		MRR:          0.9,
		QueryP50ms:   50.0,
		QueryP95ms:   100.0,
	}
	newResults := &BenchmarkResults{
		PrecisionAt5: 0.70,
		RecallAt10:   0.75,
		MRR:          0.9,
		QueryP50ms:   50.0,
		QueryP95ms:   100.0,
	}
	result := Compare(newResults, baseline)
	if !result.Passed {
		t.Errorf("expected Passed=true at exactly 0.10 drop, got %v", result.Passed)
	}
	if len(result.Regressions) != 0 {
		t.Errorf("expected 0 regressions at boundary, got %d", len(result.Regressions))
	}
}

func TestComparePrecisionAt5Over10DropRegression(t *testing.T) {
	baseline := &BenchmarkResults{
		PrecisionAt5: 0.80,
		RecallAt10:   0.75,
		MRR:          0.9,
		QueryP50ms:   50.0,
		QueryP95ms:   100.0,
	}
	newResults := &BenchmarkResults{
		PrecisionAt5: 0.69,
		RecallAt10:   0.75,
		MRR:          0.9,
		QueryP50ms:   50.0,
		QueryP95ms:   100.0,
	}
	result := Compare(newResults, baseline)
	if result.Passed {
		t.Errorf("expected Passed=false for 0.11 drop, got %v", result.Passed)
	}
	if len(result.Regressions) != 1 {
		t.Errorf("expected 1 regression, got %d", len(result.Regressions))
	}
	if result.Regressions[0].Metric != "P@5" {
		t.Errorf("expected P@5 regression, got %s", result.Regressions[0].Metric)
	}
}

func TestCompareRecallAt10Over10DropRegression(t *testing.T) {
	baseline := &BenchmarkResults{
		PrecisionAt5: 0.80,
		RecallAt10:   0.80,
		MRR:          0.9,
		QueryP50ms:   50.0,
		QueryP95ms:   100.0,
	}
	newResults := &BenchmarkResults{
		PrecisionAt5: 0.80,
		RecallAt10:   0.69,
		MRR:          0.9,
		QueryP50ms:   50.0,
		QueryP95ms:   100.0,
	}
	result := Compare(newResults, baseline)
	if result.Passed {
		t.Errorf("expected Passed=false for 0.11 drop, got %v", result.Passed)
	}
	if len(result.Regressions) != 1 {
		t.Errorf("expected 1 regression, got %d", len(result.Regressions))
	}
	if result.Regressions[0].Metric != "R@10" {
		t.Errorf("expected R@10 regression, got %s", result.Regressions[0].Metric)
	}
}

func TestCompareMRROver05DropRegression(t *testing.T) {
	baseline := &BenchmarkResults{
		PrecisionAt5: 0.80,
		RecallAt10:   0.75,
		MRR:          1.0,
		QueryP50ms:   50.0,
		QueryP95ms:   100.0,
	}
	newResults := &BenchmarkResults{
		PrecisionAt5: 0.80,
		RecallAt10:   0.75,
		MRR:          0.94,
		QueryP50ms:   50.0,
		QueryP95ms:   100.0,
	}
	result := Compare(newResults, baseline)
	if result.Passed {
		t.Errorf("expected Passed=false for 0.06 drop, got %v", result.Passed)
	}
	if len(result.Regressions) != 1 {
		t.Errorf("expected 1 regression, got %d", len(result.Regressions))
	}
	if result.Regressions[0].Metric != "MRR" {
		t.Errorf("expected MRR regression, got %s", result.Regressions[0].Metric)
	}
}

func TestCompareMRRExactly05DropBoundary(t *testing.T) {
	baseline := &BenchmarkResults{
		PrecisionAt5: 0.80,
		RecallAt10:   0.75,
		MRR:          1.0,
		QueryP50ms:   50.0,
		QueryP95ms:   100.0,
	}
	newResults := &BenchmarkResults{
		PrecisionAt5: 0.80,
		RecallAt10:   0.75,
		MRR:          0.95,
		QueryP50ms:   50.0,
		QueryP95ms:   100.0,
	}
	result := Compare(newResults, baseline)
	if !result.Passed {
		t.Errorf("expected Passed=true at exactly 0.05 drop, got %v", result.Passed)
	}
	if len(result.Regressions) != 0 {
		t.Errorf("expected 0 regressions at boundary, got %d", len(result.Regressions))
	}
}

func TestCompareQueryP95Exactly2xBoundary(t *testing.T) {
	baseline := &BenchmarkResults{
		PrecisionAt5: 0.80,
		RecallAt10:   0.75,
		MRR:          0.9,
		QueryP50ms:   50.0,
		QueryP95ms:   100.0,
	}
	newResults := &BenchmarkResults{
		PrecisionAt5: 0.80,
		RecallAt10:   0.75,
		MRR:          0.9,
		QueryP50ms:   50.0,
		QueryP95ms:   200.0,
	}
	result := Compare(newResults, baseline)
	if !result.Passed {
		t.Errorf("expected Passed=true at exactly 2.0x, got %v", result.Passed)
	}
	if len(result.Regressions) != 0 {
		t.Errorf("expected 0 regressions at boundary, got %d", len(result.Regressions))
	}
}

func TestCompareQueryP95Over2xRegression(t *testing.T) {
	baseline := &BenchmarkResults{
		PrecisionAt5: 0.80,
		RecallAt10:   0.75,
		MRR:          0.9,
		QueryP50ms:   50.0,
		QueryP95ms:   100.0,
	}
	newResults := &BenchmarkResults{
		PrecisionAt5: 0.80,
		RecallAt10:   0.75,
		MRR:          0.9,
		QueryP50ms:   50.0,
		QueryP95ms:   200.1,
	}
	result := Compare(newResults, baseline)
	if result.Passed {
		t.Errorf("expected Passed=false for 2.001x, got %v", result.Passed)
	}
	if len(result.Regressions) != 1 {
		t.Errorf("expected 1 regression, got %d", len(result.Regressions))
	}
	if result.Regressions[0].Metric != "Query P95" {
		t.Errorf("expected Query P95 regression, got %s", result.Regressions[0].Metric)
	}
}

func TestCompareMultipleRegressions(t *testing.T) {
	baseline := &BenchmarkResults{
		PrecisionAt5: 0.80,
		RecallAt10:   0.80,
		MRR:          1.0,
		QueryP50ms:   50.0,
		QueryP95ms:   100.0,
	}
	newResults := &BenchmarkResults{
		PrecisionAt5: 0.69,
		RecallAt10:   0.69,
		MRR:          0.94,
		QueryP50ms:   50.0,
		QueryP95ms:   250.0,
	}
	result := Compare(newResults, baseline)
	if result.Passed {
		t.Errorf("expected Passed=false with multiple regressions, got %v", result.Passed)
	}
	if len(result.Regressions) != 4 {
		t.Errorf("expected 4 regressions, got %d", len(result.Regressions))
	}
}

func TestCompareAllMetricsImproved(t *testing.T) {
	baseline := &BenchmarkResults{
		PrecisionAt5: 0.70,
		RecallAt10:   0.70,
		MRR:          0.8,
		QueryP50ms:   100.0,
		QueryP95ms:   200.0,
	}
	newResults := &BenchmarkResults{
		PrecisionAt5: 0.85,
		RecallAt10:   0.85,
		MRR:          0.95,
		QueryP50ms:   50.0,
		QueryP95ms:   100.0,
	}
	result := Compare(newResults, baseline)
	if !result.Passed {
		t.Errorf("expected Passed=true with all improvements, got %v", result.Passed)
	}
	if len(result.Regressions) != 0 {
		t.Errorf("expected 0 regressions, got %d", len(result.Regressions))
	}

	if result.Deltas["P@5"].Change <= 0 {
		t.Errorf("expected positive change for P@5 improvement, got %f", result.Deltas["P@5"].Change)
	}
	if result.Deltas["Query P95"].Change <= 0 {
		t.Errorf("expected positive change for Query P95 improvement (lower is better), got %f", result.Deltas["Query P95"].Change)
	}
}

func TestCompareDeltasComputed(t *testing.T) {
	baseline := &BenchmarkResults{
		PrecisionAt5: 0.80,
		RecallAt10:   0.75,
		MRR:          0.9,
		QueryP50ms:   50.0,
		QueryP95ms:   100.0,
	}
	newResults := &BenchmarkResults{
		PrecisionAt5: 0.75,
		RecallAt10:   0.70,
		MRR:          0.85,
		QueryP50ms:   55.0,
		QueryP95ms:   120.0,
	}
	result := Compare(newResults, baseline)

	if len(result.Deltas) != 5 {
		t.Errorf("expected 5 deltas, got %d", len(result.Deltas))
	}

	p5Delta := result.Deltas["P@5"]
	if !almostEqualDelta(p5Delta.Baseline, 0.80, 0.001) {
		t.Errorf("expected P@5 baseline 0.80, got %f", p5Delta.Baseline)
	}
	if !almostEqualDelta(p5Delta.New, 0.75, 0.001) {
		t.Errorf("expected P@5 new 0.75, got %f", p5Delta.New)
	}
	if !almostEqualDelta(p5Delta.Change, -0.05, 0.001) {
		t.Errorf("expected P@5 change -0.05, got %f", p5Delta.Change)
	}

	p95Delta := result.Deltas["Query P95"]
	if !almostEqualDelta(p95Delta.Change, -20.0, 0.001) {
		t.Errorf("expected Query P95 change -20.0 (positive improvement), got %f", p95Delta.Change)
	}
}
