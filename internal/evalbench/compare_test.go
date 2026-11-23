package evalbench_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nano-brain/nano-brain/internal/evalbench"
)

func TestCompareEvalReports_DetectsRegressionsAndIgnoresLLMNoise(t *testing.T) {
	baseline := evalbench.EvalReport{
		Version:       "1",
		OverallRecall: 0.900,
		OverallMRR:    0.850,
		Suites: []evalbench.SuiteResult{
			{Suite: "go", Status: "ok", TaskCount: 10, OverallRecall: 0.900, OverallMRR: 0.850},
		},
		ToolLatencies: []evalbench.ToolLatencySummary{
			{Tool: "memory_impact", Count: 5, P50Ms: 12, P95Ms: 25},
			{Tool: "memory_query", Count: 5, P50Ms: 120, P95Ms: 300},
		},
		TokenEfficiency: evalbench.TokenEfficiencySummary{
			TasksEvaluated: 10,
			AgentTokens:    1000,
		},
		Replay: &evalbench.ReplaySummary{
			Status:        "ok",
			TotalReplayed: 20,
			LiveEmptyRate: 0.100,
		},
	}

	// Passing candidate: LLM query p95 fluctuated (+200ms), non-LLM impact stayed fast.
	passing := baseline
	passing.ToolLatencies = []evalbench.ToolLatencySummary{
		{Tool: "memory_impact", Count: 5, P50Ms: 14, P95Ms: 40},
		{Tool: "memory_query", Count: 5, P50Ms: 180, P95Ms: 520},
	}
	cmpPass := evalbench.CompareEvalReports(baseline, passing, evalbench.DefaultCompareThresholds())
	if !cmpPass.Passed {
		t.Fatalf("expected comparison to pass, got regressions: %+v", cmpPass.Regressions)
	}

	// Regressing candidate across recall, MRR, tokens, non-LLM p95, and replay empty rate.
	regressed := evalbench.EvalReport{
		Version:       "1",
		OverallRecall: 0.820,
		OverallMRR:    0.780,
		Suites: []evalbench.SuiteResult{
			{Suite: "go", Status: "ok", TaskCount: 10, OverallRecall: 0.820, OverallMRR: 0.780},
		},
		ToolLatencies: []evalbench.ToolLatencySummary{
			{Tool: "memory_impact", Count: 5, P50Ms: 30, P95Ms: 110},
		},
		TokenEfficiency: evalbench.TokenEfficiencySummary{
			TasksEvaluated: 10,
			AgentTokens:    1500,
		},
		Replay: &evalbench.ReplaySummary{
			Status:        "ok",
			TotalReplayed: 20,
			LiveEmptyRate: 0.250,
		},
	}

	cmpFail := evalbench.CompareEvalReports(baseline, regressed, evalbench.DefaultCompareThresholds())
	if cmpFail.Passed {
		t.Fatal("expected regression failure, got Passed=true")
	}
	foundMetrics := make(map[string]bool)
	for _, r := range cmpFail.Regressions {
		foundMetrics[r.Metric] = true
	}
	for _, want := range []string{
		"overall_recall",
		"overall_mrr",
		"suite.go.recall",
		"token_usage.agent_tokens",
		"latency.memory_impact.p95_ms",
		"replay.empty_rate",
	} {
		if !foundMetrics[want] {
			t.Errorf("expected regression metric %q in %+v", want, cmpFail.Regressions)
		}
	}
}

func TestFormatMarkdownAndProfilesUnmarshal(t *testing.T) {
	var flatCfg evalbench.ProfilesConfig
	rawFlat := `{"vue":{"workspace":"vue-app","vars":{"PROJECT":"app"}}}`
	if err := json.Unmarshal([]byte(rawFlat), &flatCfg); err != nil {
		t.Fatalf("unmarshal flat profiles: %v", err)
	}
	if flatCfg.Suites["vue"].Vars["PROJECT"] != "app" {
		t.Fatalf("unexpected flat profile vars: %+v", flatCfg)
	}

	var nestedCfg evalbench.ProfilesConfig
	rawNested := `{"suites":{"rails":{"workspace":"rails-app"}}}`
	if err := json.Unmarshal([]byte(rawNested), &nestedCfg); err != nil {
		t.Fatalf("unmarshal nested profiles: %v", err)
	}
	if nestedCfg.Suites["rails"].Workspace != "rails-app" {
		t.Fatalf("unexpected nested profile workspace: %+v", nestedCfg)
	}

	rep := evalbench.EvalReport{
		Version:            "1",
		ServerURL:          "http://localhost:3199",
		Transport:          evalbench.TransportMCP,
		StrictImmutable:    true,
		Mutated:            false,
		OverallRecall:      0.950,
		OverallFixedRecall: 0.900,
		OverallRecallAt5:   0.925,
		OverallMRR:         0.875,
		SnapshotDiff:       evalbench.SnapshotDiff{Note: "Zero mutations verified."},
		Suites: []evalbench.SuiteResult{
			{
				Suite:               "vue",
				Status:              "ok",
				TaskCount:           4,
				OverallRecall:       0.950,
				OverallFixedRecall:  0.900,
				OverallRecallAt5:    0.925,
				OverallMRR:          0.875,
				ByCategory:          map[string]float64{"trace": 1.0},
				ByCategoryRecallAt5: map[string]float64{"trace": 1.0},
				ByCategoryMRR:       map[string]float64{"trace": 1.0},
				TokenEfficiency: evalbench.TokenEfficiencySummary{
					TasksEvaluated:       4,
					TasksWithBaseline:    4,
					FixedTokens:          120,
					AgentTokens:          240,
					NaiveTokens:          2400,
					FilesReadCount:       4,
					TokenSavingsFixedPct: 95.0,
					TokenSavingsAgentPct: 90.0,
				},
			},
		},
		CollectionDistribution: map[string]int{"code": 8, "memory": 2},
		TokenEfficiency: evalbench.TokenEfficiencySummary{
			TasksEvaluated:       4,
			TasksWithBaseline:    4,
			FixedTokens:          120,
			AgentTokens:          240,
			NaiveTokens:          2400,
			FilesReadCount:       4,
			TokenSavingsFixedPct: 95.0,
			TokenSavingsAgentPct: 90.0,
		},
	}
	md := evalbench.FormatMarkdown(rep)
	for _, snippet := range []string{
		"# nano-brain Read-Only Evaluation Scorecard",
		"`vue`",
		"90.0%",
		"Context-Gathering Token Efficiency (#529)",
		"Search Collection Mix",
	} {
		if !strings.Contains(md, snippet) {
			t.Errorf("expected Markdown scorecard to contain %q, got:\n%s", snippet, md)
		}
	}
}
