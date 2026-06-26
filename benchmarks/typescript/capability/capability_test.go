//go:build capbench

package capability

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

const (
	datasetPath  = "dataset.json"
	baselinePath = "baseline_v1.json"
	resultsPath  = "results_current.json"
)

func TestCapabilityBenchmark(t *testing.T) {
	cfg := DefaultConfig()

	if err := CheckReachable(cfg.ServerURL); err != nil {
		t.Skipf("nano-brain server unreachable at %s: %v", cfg.ServerURL, err)
	}

	raw, err := os.ReadFile(datasetPath)
	if err != nil {
		t.Fatalf("read dataset: %v", err)
	}
	var dataset Dataset
	if err := json.Unmarshal(raw, &dataset); err != nil {
		t.Fatalf("parse dataset: %v", err)
	}
	if len(dataset.Tasks) == 0 {
		t.Fatal("dataset contains no tasks")
	}

	client := newHTTPClient()
	ctx := context.Background()

	taskResults := make([]TaskResult, 0, len(dataset.Tasks))
	for _, task := range dataset.Tasks {
		r := RunTask(ctx, client, cfg, dataset.Agent, task)
		taskResults = append(taskResults, r)
	}

	results := Aggregate(taskResults)

	printScorecard(t, results)

	writeJSON(t, resultsPath, results)

	if cfg.Freeze {
		writeJSON(t, baselinePath, results)
		t.Log("baseline frozen to", baselinePath)
		return
	}

	baselineRaw, err := os.ReadFile(baselinePath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Log("no baseline found; run with CAPBENCH_FREEZE=1 to freeze one")
			return
		}
		t.Fatalf("read baseline: %v", err)
	}
	var baseline BenchResults
	if err := json.Unmarshal(baselineRaw, &baseline); err != nil {
		t.Fatalf("parse baseline: %v", err)
	}

	printDelta(t, baseline, results)

	const regressionThreshold = 0.001
	if results.Overall < baseline.Overall-regressionThreshold {
		t.Errorf("REGRESSION: overall recall %.3f < baseline %.3f (delta %.3f)",
			results.Overall, baseline.Overall, results.Overall-baseline.Overall)
	}
}

func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

func printScorecard(t *testing.T, r BenchResults) {
	t.Helper()
	var sb strings.Builder

	sb.WriteString("\n=== CAPABILITY BENCHMARK SCORECARD ===\n")
	sb.WriteString(fmt.Sprintf("%-40s %-14s %-8s %-8s %s\n", "TASK ID", "CATEGORY", "FIXED", "AGENT", "RECALL"))
	sb.WriteString(strings.Repeat("-", 70) + "\n")

	for _, task := range r.Tasks {
		sb.WriteString(fmt.Sprintf("%-40s %-14s %.3f    %.3f    %.3f", task.ID, task.Category, task.FixedRecall, task.AgentRecall, task.Recall))
		if len(task.Missed) > 0 {
			sb.WriteString(fmt.Sprintf("  missed: %s", strings.Join(task.Missed, ", ")))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(strings.Repeat("-", 70) + "\n")
	sb.WriteString("BY CATEGORY:\n")

	cats := make([]string, 0, len(r.ByCategory))
	for c := range r.ByCategory {
		cats = append(cats, c)
	}
	sort.Strings(cats)
	for _, cat := range cats {
		sb.WriteString(fmt.Sprintf("  %-20s %.3f\n", cat, r.ByCategory[cat]))
	}

	sb.WriteString(fmt.Sprintf("\nOVERALL: %.3f\n", r.Overall))
	sb.WriteString("=====================================\n")

	t.Log(sb.String())
}

func printDelta(t *testing.T, baseline, current BenchResults) {
	t.Helper()
	var sb strings.Builder

	sb.WriteString("\n=== DELTA vs BASELINE ===\n")
	sb.WriteString(fmt.Sprintf("%-40s %-8s %-8s %s\n", "TASK ID", "BASE", "NOW", "DELTA"))
	sb.WriteString(strings.Repeat("-", 70) + "\n")

	baseMap := make(map[string]float64, len(baseline.Tasks))
	for _, bt := range baseline.Tasks {
		baseMap[bt.ID] = bt.Recall
	}

	for _, task := range current.Tasks {
		baseRecall := baseMap[task.ID]
		delta := task.Recall - baseRecall
		marker := ""
		if delta > 0.001 {
			marker = " +"
		} else if delta < -0.001 {
			marker = " *** REGRESSION"
		}
		sb.WriteString(fmt.Sprintf("%-40s %.3f    %.3f    %+.3f%s\n",
			task.ID, baseRecall, task.Recall, delta, marker))
	}

	sb.WriteString(strings.Repeat("-", 70) + "\n")
	sb.WriteString("BY CATEGORY:\n")

	cats := make([]string, 0)
	seen := make(map[string]bool)
	for c := range current.ByCategory {
		if !seen[c] {
			cats = append(cats, c)
			seen[c] = true
		}
	}
	sort.Strings(cats)
	for _, cat := range cats {
		cur := current.ByCategory[cat]
		base := baseline.ByCategory[cat]
		sb.WriteString(fmt.Sprintf("  %-20s base=%.3f now=%.3f delta=%+.3f\n", cat, base, cur, cur-base))
	}

	overallDelta := current.Overall - baseline.Overall
	sb.WriteString(fmt.Sprintf("\nOVERALL: base=%.3f now=%.3f delta=%+.3f\n",
		baseline.Overall, current.Overall, overallDelta))
	if overallDelta > 0.001 {
		sb.WriteString("IMPROVEMENT detected.\n")
	} else if overallDelta < -0.001 {
		sb.WriteString("REGRESSION detected.\n")
	} else {
		sb.WriteString("No significant change.\n")
	}
	sb.WriteString("=========================\n")

	t.Log(sb.String())
}

func writeJSON(t *testing.T, path string, v interface{}) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
