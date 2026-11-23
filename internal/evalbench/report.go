package evalbench

import (
	"fmt"
	"sort"
	"strings"
)

// FormatMarkdown renders a human-readable Markdown scorecard for an EvalReport.
func FormatMarkdown(report EvalReport) string {
	var sb strings.Builder
	writeMarkdownReport(&sb, report)
	return sb.String()
}

func writeMarkdownReport(sb *strings.Builder, r EvalReport) {
	sb.WriteString("# nano-brain Read-Only Evaluation Scorecard\n\n")
	sb.WriteString(fmt.Sprintf("- **Server**: `%s`\n", r.ServerURL))
	sb.WriteString(fmt.Sprintf("- **Transport**: `%s`\n", r.Transport))
	sb.WriteString(fmt.Sprintf("- **Zero-Mutation Verified**: `%t` (`mutated=%t`, strict_immutable=`%t`)\n", !r.Mutated, r.Mutated, r.StrictImmutable))
	if r.SnapshotDiff.Note != "" {
		sb.WriteString(fmt.Sprintf("- **Snapshot Guard**: %s\n", r.SnapshotDiff.Note))
	}
	sb.WriteString("\n")

	if len(r.Suites) > 0 {
		sb.WriteString("## Tier 1 — Multi-Workspace Capability & Ranking\n\n")
		sb.WriteString("| Suite | Status | Tasks | Fixed Recall | Recall | Recall@5 | MRR | Token Savings (Agent) |\n")
		sb.WriteString("|---|---|---:|---:|---:|---:|---:|---:|\n")
		for _, s := range r.Suites {
			if s.Status != "ok" {
				reason := s.SkipReason
				if reason == "" {
					reason = s.Status
				}
				sb.WriteString(fmt.Sprintf("| `%s` | %s (%s) | 0 | - | - | - | - | - |\n", s.Suite, s.Status, reason))
				continue
			}
			savingsStr := "-"
			if s.TokenEfficiency.TasksWithBaseline > 0 {
				savingsStr = fmt.Sprintf("%.1f%%", s.TokenEfficiency.TokenSavingsAgentPct)
			}
			sb.WriteString(fmt.Sprintf("| `%s` | ok | %d | %.3f | %.3f | %.3f | %.3f | %s |\n",
				s.Suite, s.TaskCount, s.OverallFixedRecall, s.OverallRecall, s.OverallRecallAt5, s.OverallMRR, savingsStr))
		}
		sb.WriteString(fmt.Sprintf("| **Overall** | - | %d | **%.3f** | **%.3f** | **%.3f** | **%.3f** | - |\n\n",
			r.TokenEfficiency.TasksEvaluated, r.OverallFixedRecall, r.OverallRecall, r.OverallRecallAt5, r.OverallMRR))

		for _, s := range r.Suites {
			if s.Status != "ok" || len(s.ByCategory) == 0 {
				continue
			}
			sb.WriteString(fmt.Sprintf("### Suite `%s` by Category\n\n", s.Suite))
			sb.WriteString("| Category | Recall | Recall@5 | MRR |\n")
			sb.WriteString("|---|---:|---:|---:|\n")
			cats := make([]string, 0, len(s.ByCategory))
			for c := range s.ByCategory {
				cats = append(cats, c)
			}
			sort.Strings(cats)
			for _, c := range cats {
				sb.WriteString(fmt.Sprintf("| `%s` | %.3f | %.3f | %.3f |\n",
					c, s.ByCategory[c], s.ByCategoryRecallAt5[c], s.ByCategoryMRR[c]))
			}
			sb.WriteString("\n")
		}
	}

	if len(r.ToolLatencies) > 0 {
		sb.WriteString("## Per-Tool Latency\n\n")
		sb.WriteString("| Tool | Calls | Empty | Errors | p50 (ms) | p95 (ms) | Mean (ms) |\n")
		sb.WriteString("|---|---:|---:|---:|---:|---:|---:|\n")
		for _, tl := range r.ToolLatencies {
			sb.WriteString(fmt.Sprintf("| `%s` | %d | %d | %d | %.2f | %.2f | %.2f |\n",
				tl.Tool, tl.Count, tl.EmptyCount, tl.ErrorCount, tl.P50Ms, tl.P95Ms, tl.MeanMs))
		}
		sb.WriteString("\n")
	}

	if r.TokenEfficiency.TasksEvaluated > 0 {
		te := r.TokenEfficiency
		sb.WriteString("## Context-Gathering Token Efficiency (#529)\n\n")
		sb.WriteString("| Metric | Value |\n")
		sb.WriteString("|---|---:|\n")
		sb.WriteString(fmt.Sprintf("| Tasks Evaluated / With Local Baseline | %d / %d |\n", te.TasksEvaluated, te.TasksWithBaseline))
		sb.WriteString(fmt.Sprintf("| nano-brain Fixed Response | %d tokens (%d bytes) |\n", te.FixedTokens, te.FixedBytes))
		sb.WriteString(fmt.Sprintf("| nano-brain Agent Response | %d tokens (%d bytes) |\n", te.AgentTokens, te.AgentBytes))
		if te.TasksWithBaseline > 0 {
			sb.WriteString(fmt.Sprintf("| Naive Grep + Ground-Truth File Reads | %d tokens (%d bytes across %d files) |\n",
				te.NaiveTokens, te.NaiveBytes, te.FilesReadCount))
			sb.WriteString(fmt.Sprintf("| Token Savings (Fixed vs Naive) | **%.2f%%** |\n", te.TokenSavingsFixedPct))
			sb.WriteString(fmt.Sprintf("| Token Savings (Agent vs Naive) | **%.2f%%** |\n", te.TokenSavingsAgentPct))
		}
		sb.WriteString("\n")
	}

	if len(r.CollectionDistribution) > 0 {
		sb.WriteString("## Search Collection Mix\n\n")
		sb.WriteString("| Collection | Hits |\n")
		sb.WriteString("|---|---:|\n")
		for _, col := range []string{"code", "memory", "sessions", "other"} {
			if n, ok := r.CollectionDistribution[col]; ok && n > 0 {
				sb.WriteString(fmt.Sprintf("| `%s` | %d |\n", col, n))
			}
		}
		sb.WriteString("\n")
	}

	if r.Replay != nil {
		rep := r.Replay
		sb.WriteString("## Tier 2 — Observe Traffic Read-Only Replay\n\n")
		if rep.Status != "ok" {
			sb.WriteString(fmt.Sprintf("- **Status**: `%s` (%s)\n\n", rep.Status, rep.SkipReason))
			return
		}
		sb.WriteString(fmt.Sprintf("- **Files Read**: %d · **Total Log Entries**: %d · **Replayed**: %d · **Skipped Mutations**: %d · **Skipped Non-Tool**: %d\n",
			rep.FilesRead, rep.TotalEntries, rep.TotalReplayed, rep.SkippedMutations, rep.SkippedNonTool))
		sb.WriteString(fmt.Sprintf("- **Empty Rate**: recorded `%.3f` -> live `%.3f`\n", rep.RecordedEmptyRate, rep.LiveEmptyRate))
		sb.WriteString(fmt.Sprintf("- **Error Rate**: recorded `%.3f` -> live `%.3f`\n", rep.RecordedErrorRate, rep.LiveErrorRate))
		sb.WriteString(fmt.Sprintf("- **Latency (p50 / p95)**: recorded `%.2fms / %.2fms` -> live `%.2fms / %.2fms`\n",
			rep.RecordedP50Ms, rep.RecordedP95Ms, rep.LiveP50Ms, rep.LiveP95Ms))
		if rep.Top5OverlapSamples > 0 {
			sb.WriteString(fmt.Sprintf("- **Top-5 Result Overlap (Jaccard)**: `%.3f` (%d comparable calls)\n",
				rep.Top5OverlapMean, rep.Top5OverlapSamples))
		}
		sb.WriteString("\n")

		if len(rep.ByTool) > 0 {
			sb.WriteString("| Tool | Replayed | Rec Empty | Live Empty | Rec p50/p95 (ms) | Live p50/p95 (ms) | Top-5 Overlap |\n")
			sb.WriteString("|---|---:|---:|---:|---:|---:|---:|\n")
			for _, bt := range rep.ByTool {
				overlapStr := "-"
				if bt.Top5OverlapSamples > 0 {
					overlapStr = fmt.Sprintf("%.3f (%d)", bt.Top5OverlapMean, bt.Top5OverlapSamples)
				}
				sb.WriteString(fmt.Sprintf("| `%s` | %d | %.3f | %.3f | %.1f / %.1f | %.1f / %.1f | %s |\n",
					bt.Tool, bt.Count, bt.RecordedEmptyRate, bt.LiveEmptyRate,
					bt.RecordedP50Ms, bt.RecordedP95Ms, bt.LiveP50Ms, bt.LiveP95Ms, overlapStr))
			}
			sb.WriteString("\n")
		}

		if len(rep.ByWorkspace) > 0 {
			sb.WriteString("| Workspace Slot | Replayed | Live Empty Rate | Live Error Rate | Live p50/p95 (ms) |\n")
			sb.WriteString("|---|---:|---:|---:|---:|\n")
			for _, bw := range rep.ByWorkspace {
				sb.WriteString(fmt.Sprintf("| `%s` | %d | %.3f | %.3f | %.1f / %.1f |\n",
					bw.Slot, bw.Count, bw.LiveEmptyRate, bw.LiveErrorRate, bw.LiveP50Ms, bw.LiveP95Ms))
			}
			sb.WriteString("\n")
		}
	}
}
