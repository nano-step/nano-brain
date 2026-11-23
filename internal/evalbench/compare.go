package evalbench

import (
	"fmt"
)

// DefaultCompareThresholds returns default regression tolerances for CompareEvalReports.
func DefaultCompareThresholds() CompareThresholds {
	return CompareThresholds{
		MaxRecallDrop:         0.01,
		MaxMRRDrop:            0.02,
		MaxEmptyRateIncrease:  0.05,
		MaxTokenIncreasePct:   25.0,
		MaxP95LatencyIncrease: 50.0,
	}
}

var llmBackedTools = map[string]bool{
	"memory_query":   true,
	"memory_vsearch": true,
}

// CompareEvalReports checks current against baseline and returns any regressions
// exceeding thresholds across OverallRecall, OverallMRR, per-suite recall/MRR,
// replay EmptyRate, token usage, and non-LLM tool P95 latency.
func CompareEvalReports(baseline, current EvalReport, thresholds CompareThresholds) EvalComparison {
	var regressions []RegressionFinding

	hasEvalSuites := hasCompletedSuites(baseline.Suites) && hasCompletedSuites(current.Suites)
	if hasEvalSuites {
		if drop := baseline.OverallRecall - current.OverallRecall; drop > thresholds.MaxRecallDrop {
			regressions = append(regressions, RegressionFinding{
				Metric:   "overall_recall",
				Baseline: baseline.OverallRecall,
				Current:  current.OverallRecall,
				Delta:    round3(current.OverallRecall - baseline.OverallRecall),
				Message:  fmt.Sprintf("overall recall dropped from %.3f to %.3f (max drop %.3f)", baseline.OverallRecall, current.OverallRecall, thresholds.MaxRecallDrop),
			})
		}
		if drop := baseline.OverallMRR - current.OverallMRR; drop > thresholds.MaxMRRDrop {
			regressions = append(regressions, RegressionFinding{
				Metric:   "overall_mrr",
				Baseline: baseline.OverallMRR,
				Current:  current.OverallMRR,
				Delta:    round3(current.OverallMRR - baseline.OverallMRR),
				Message:  fmt.Sprintf("overall MRR dropped from %.3f to %.3f (max drop %.3f)", baseline.OverallMRR, current.OverallMRR, thresholds.MaxMRRDrop),
			})
		}
	}

	baseSuites := make(map[string]SuiteResult, len(baseline.Suites))
	for _, s := range baseline.Suites {
		if s.Status == "ok" {
			baseSuites[s.Suite] = s
		}
	}
	for _, cur := range current.Suites {
		if cur.Status != "ok" {
			continue
		}
		base, ok := baseSuites[cur.Suite]
		if !ok {
			continue
		}
		if drop := base.OverallRecall - cur.OverallRecall; drop > thresholds.MaxRecallDrop {
			regressions = append(regressions, RegressionFinding{
				Metric:   fmt.Sprintf("suite.%s.recall", cur.Suite),
				Baseline: base.OverallRecall,
				Current:  cur.OverallRecall,
				Delta:    round3(cur.OverallRecall - base.OverallRecall),
				Message:  fmt.Sprintf("suite %s recall dropped from %.3f to %.3f", cur.Suite, base.OverallRecall, cur.OverallRecall),
			})
		}
		if drop := base.OverallMRR - cur.OverallMRR; drop > thresholds.MaxMRRDrop {
			regressions = append(regressions, RegressionFinding{
				Metric:   fmt.Sprintf("suite.%s.mrr", cur.Suite),
				Baseline: base.OverallMRR,
				Current:  cur.OverallMRR,
				Delta:    round3(cur.OverallMRR - base.OverallMRR),
				Message:  fmt.Sprintf("suite %s MRR dropped from %.3f to %.3f", cur.Suite, base.OverallMRR, cur.OverallMRR),
			})
		}
	}

	if baseline.TokenEfficiency.AgentTokens > 0 && current.TokenEfficiency.AgentTokens > 0 {
		pctIncrease := (float64(current.TokenEfficiency.AgentTokens-baseline.TokenEfficiency.AgentTokens) / float64(baseline.TokenEfficiency.AgentTokens)) * 100.0
		if pctIncrease > thresholds.MaxTokenIncreasePct {
			regressions = append(regressions, RegressionFinding{
				Metric:   "token_usage.agent_tokens",
				Baseline: float64(baseline.TokenEfficiency.AgentTokens),
				Current:  float64(current.TokenEfficiency.AgentTokens),
				Delta:    round3(pctIncrease),
				Message:  fmt.Sprintf("agent token usage increased by %.1f%% (%d -> %d, max +%.1f%%)", pctIncrease, baseline.TokenEfficiency.AgentTokens, current.TokenEfficiency.AgentTokens, thresholds.MaxTokenIncreasePct),
			})
		}
	}

	baseLat := make(map[string]ToolLatencySummary, len(baseline.ToolLatencies))
	for _, tl := range baseline.ToolLatencies {
		baseLat[tl.Tool] = tl
	}
	for _, cur := range current.ToolLatencies {
		if llmBackedTools[cur.Tool] || cur.Count == 0 {
			continue
		}
		base, ok := baseLat[cur.Tool]
		if !ok || base.Count == 0 {
			continue
		}
		if inc := cur.P95Ms - base.P95Ms; inc > thresholds.MaxP95LatencyIncrease {
			regressions = append(regressions, RegressionFinding{
				Metric:   fmt.Sprintf("latency.%s.p95_ms", cur.Tool),
				Baseline: base.P95Ms,
				Current:  cur.P95Ms,
				Delta:    round3(inc),
				Message:  fmt.Sprintf("non-LLM tool %s p95 latency increased by %.1fms (%.1fms -> %.1fms, max +%.1fms)", cur.Tool, inc, base.P95Ms, cur.P95Ms, thresholds.MaxP95LatencyIncrease),
			})
		}
	}

	if baseline.Replay != nil && current.Replay != nil && baseline.Replay.TotalReplayed > 0 && current.Replay.TotalReplayed > 0 {
		if inc := current.Replay.LiveEmptyRate - baseline.Replay.LiveEmptyRate; inc > thresholds.MaxEmptyRateIncrease {
			regressions = append(regressions, RegressionFinding{
				Metric:   "replay.empty_rate",
				Baseline: baseline.Replay.LiveEmptyRate,
				Current:  current.Replay.LiveEmptyRate,
				Delta:    round3(inc),
				Message:  fmt.Sprintf("replay empty rate increased from %.3f to %.3f (max +%.3f)", baseline.Replay.LiveEmptyRate, current.Replay.LiveEmptyRate, thresholds.MaxEmptyRateIncrease),
			})
		}
	}

	return EvalComparison{
		Passed:      len(regressions) == 0,
		Regressions: regressions,
	}
}

func hasCompletedSuites(suites []SuiteResult) bool {
	for _, s := range suites {
		if s.Status == "ok" && s.TaskCount > 0 {
			return true
		}
	}
	return false
}
