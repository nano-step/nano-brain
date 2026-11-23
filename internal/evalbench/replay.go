package evalbench

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ReplayOptions configures Tier 2 read-only replay of nano-brain-observe JSONL logs.
type ReplayOptions struct {
	LogPaths       []string
	Limit          int
	VerbosePrivate bool
}

var replayGlobalTools = map[string]bool{
	"memory_status":             true,
	"memory_workspaces_list":    true,
	"memory_workspaces_resolve": true,
	"memory_ticket":             true,
}

var replayOverlapTools = map[string]bool{
	"memory_query":     true,
	"memory_search":    true,
	"memory_vsearch":   true,
	"memory_symbols":   true,
	"memory_get":       true,
	"memory_multi_get": true,
	"memory_graph":     true,
	"memory_impact":    true,
	"memory_trace":     true,
	"memory_flow":      true,
}

type observeLogEntry struct {
	Kind        string          `json:"kind"`
	Tool        string          `json:"tool"`
	Args        json.RawMessage `json:"args"`
	Workspace   string          `json:"workspace"`
	Status      string          `json:"status"`
	ResultCount *int            `json:"result_count"`
	LatencyMs   float64         `json:"latency_ms"`
	Response    string          `json:"response"`
}

// ExpandReplayLogPaths expands tilde prefixes, directories, and globs into sorted JSONL file paths.
func ExpandReplayLogPaths(patterns []string) ([]string, error) {
	seen := make(map[string]bool)
	var files []string
	add := func(p string) {
		clean := filepath.Clean(p)
		if !seen[clean] {
			seen[clean] = true
			files = append(files, clean)
		}
	}

	for _, raw := range patterns {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "~/") || p == "~" {
			if home, err := os.UserHomeDir(); err == nil {
				if p == "~" {
					p = home
				} else {
					p = filepath.Join(home, p[2:])
				}
			}
		}

		if strings.ContainsAny(p, "*?[") {
			matches, err := filepath.Glob(p)
			if err != nil {
				return nil, fmt.Errorf("invalid replay log glob %q: %w", raw, err)
			}
			for _, m := range matches {
				if info, statErr := os.Stat(m); statErr == nil && !info.IsDir() {
					add(m)
				}
			}
			continue
		}

		info, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat replay path %q: %w", raw, err)
		}
		if info.IsDir() {
			matches, _ := filepath.Glob(filepath.Join(p, "calls-*.jsonl"))
			for _, m := range matches {
				if fi, sErr := os.Stat(m); sErr == nil && !fi.IsDir() {
					add(m)
				}
			}
			continue
		}
		add(p)
	}

	sort.Strings(files)
	return files, nil
}

func parseObserveArgs(raw json.RawMessage) (map[string]any, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return map[string]any{}, nil
	}
	if strings.HasPrefix(trimmed, "{") {
		var out map[string]any
		if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
			return nil, err
		}
		if out == nil {
			out = map[string]any{}
		}
		return out, nil
	}
	if strings.HasPrefix(trimmed, "\"") {
		var inner string
		if err := json.Unmarshal([]byte(trimmed), &inner); err != nil {
			return nil, err
		}
		inner = strings.TrimSpace(inner)
		if inner == "" {
			return map[string]any{}, nil
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(inner), &out); err != nil {
			return nil, err
		}
		if out == nil {
			out = map[string]any{}
		}
		return out, nil
	}
	return map[string]any{}, nil
}

// RunReplay reads nano-brain-observe JSONL logs, filters strictly to read-only
// mcp_tool calls, replays them over /mcp, and returns aggregated replay metrics
// plus any resolved workspace hashes touched during replay.
func RunReplay(ctx context.Context, client *EvalClient, opts ReplayOptions, snapshot WorkspaceSnapshot) (ReplaySummary, []string, error) {
	files, err := ExpandReplayLogPaths(opts.LogPaths)
	if err != nil {
		return ReplaySummary{Status: "error", SkipReason: err.Error()}, nil, err
	}
	if len(files) == 0 {
		return ReplaySummary{
			Status:     "skipped",
			SkipReason: "no observe JSONL log files found",
		}, nil, nil
	}

	type toolAccum struct {
		count         int
		recEmpty      int
		liveEmpty     int
		recErr        int
		liveErr       int
		recLatencies  []float64
		liveLatencies []float64
		overlaps      []float64
	}
	type wsAccum struct {
		slot          string
		count         int
		liveEmpty     int
		liveErr       int
		liveLatencies []float64
	}

	byTool := make(map[string]*toolAccum)
	byWS := make(map[string]*wsAccum)
	wsSlotMap := make(map[string]string)
	nextSlot := 0
	touchedHashes := make(map[string]bool)

	slotFor := func(rawWS string) string {
		trimmed := strings.TrimSpace(rawWS)
		if trimmed == "" || trimmed == "all" {
			return "global"
		}
		if wsInfo, ok, _ := client.ResolveWorkspaceReadOnly(ctx, TransportMCP, trimmed, snapshot); ok && wsInfo.Hash != "" {
			touchedHashes[wsInfo.Hash] = true
			trimmed = wsInfo.Hash
		}
		if slot, exists := wsSlotMap[trimmed]; exists {
			return slot
		}
		nextSlot++
		slot := fmt.Sprintf("ws-%d", nextSlot)
		wsSlotMap[trimmed] = slot
		return slot
	}

	var summary ReplaySummary
	summary.Status = "ok"
	summary.FilesRead = len(files)

	var allRecLat, allLiveLat, allOverlaps []float64
	var totalRecEmpty, totalLiveEmpty, totalRecErr, totalLiveErr int

	limitReached := false
	for _, fpath := range files {
		if limitReached {
			break
		}
		fh, openErr := os.Open(fpath)
		if openErr != nil {
			return ReplaySummary{}, nil, fmt.Errorf("open replay log %s: %w", fpath, openErr)
		}
		scanner := bufio.NewScanner(fh)
		scanner.Buffer(make([]byte, 64*1024), 8<<20)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var entry observeLogEntry
			if json.Unmarshal([]byte(line), &entry) != nil {
				continue
			}
			summary.TotalEntries++

			if entry.Kind != "mcp_tool" {
				summary.SkippedNonTool++
				continue
			}
			if !AllowedMCPTools[entry.Tool] {
				summary.SkippedMutations++
				continue
			}
			if opts.Limit > 0 && summary.TotalReplayed >= opts.Limit {
				limitReached = true
				break
			}

			args, argErr := parseObserveArgs(entry.Args)
			if argErr != nil {
				summary.SkippedNonTool++
				continue
			}

			rawWS, _ := args["workspace"].(string)
			if rawWS == "" && entry.Workspace != "" && !replayGlobalTools[entry.Tool] {
				rawWS = entry.Workspace
				args["workspace"] = entry.Workspace
			}
			wsSlot := "global"
			if !replayGlobalTools[entry.Tool] {
				wsSlot = slotFor(rawWS)
			}

			recText, recRespErr := UnwrapRecordedResponse(entry.Response)
			recParsed := ParseToolPayload(entry.Tool, []byte(recText))
			recIsErr := entry.Status == "error" || recRespErr
			recIsEmpty := !recIsErr && (entry.Status == "empty" ||
				(entry.ResultCount != nil && *entry.ResultCount == 0) ||
				(entry.Status == "" && entry.ResultCount == nil && recParsed.IsEmpty))

			liveRes, callErr := client.CallMCPTool(ctx, entry.Tool, args)
			if callErr != nil {
				_ = fh.Close()
				return ReplaySummary{}, nil, fmt.Errorf("replay %s: %w", entry.Tool, callErr)
			}

			liveIsErr := liveRes.IsError
			liveIsEmpty := !liveIsErr && liveRes.IsEmpty

			summary.TotalReplayed++
			allRecLat = append(allRecLat, entry.LatencyMs)
			allLiveLat = append(allLiveLat, liveRes.LatencyMs)
			if recIsEmpty {
				totalRecEmpty++
			}
			if liveIsEmpty {
				totalLiveEmpty++
			}
			if recIsErr {
				totalRecErr++
			}
			if liveIsErr {
				totalLiveErr++
			}

			ta, ok := byTool[entry.Tool]
			if !ok {
				ta = &toolAccum{}
				byTool[entry.Tool] = ta
			}
			ta.count++
			ta.recLatencies = append(ta.recLatencies, entry.LatencyMs)
			ta.liveLatencies = append(ta.liveLatencies, liveRes.LatencyMs)
			if recIsEmpty {
				ta.recEmpty++
			}
			if liveIsEmpty {
				ta.liveEmpty++
			}
			if recIsErr {
				ta.recErr++
			}
			if liveIsErr {
				ta.liveErr++
			}

			var overlapPtr *float64
			if replayOverlapTools[entry.Tool] && !recIsErr && !liveIsErr {
				recKeys := extractTop5Keys(recParsed.OrderedItems)
				liveKeys := extractTop5Keys(liveRes.OrderedItems)
				if len(recKeys) > 0 && len(liveKeys) > 0 {
					ov := jaccardTop5(recKeys, liveKeys)
					ta.overlaps = append(ta.overlaps, ov)
					allOverlaps = append(allOverlaps, ov)
					ovRounded := round3(ov)
					overlapPtr = &ovRounded
				}
			}

			wa, ok := byWS[wsSlot]
			if !ok {
				wa = &wsAccum{slot: wsSlot}
				byWS[wsSlot] = wa
			}
			wa.count++
			wa.liveLatencies = append(wa.liveLatencies, liveRes.LatencyMs)
			if liveIsEmpty {
				wa.liveEmpty++
			}
			if liveIsErr {
				wa.liveErr++
			}

			if opts.VerbosePrivate {
				liveStatus := "ok"
				if liveIsErr {
					liveStatus = "error"
				} else if liveIsEmpty {
					liveStatus = "empty"
				}
				summary.Details = append(summary.Details, ReplayCallDetail{
					Tool:              entry.Tool,
					WorkspaceSlot:     wsSlot,
					Args:              args,
					RecordedStatus:    entry.Status,
					LiveStatus:        liveStatus,
					RecordedLatencyMs: entry.LatencyMs,
					LiveLatencyMs:     round2(liveRes.LatencyMs),
					Top5Overlap:       overlapPtr,
				})
			}
		}
		scanErr := scanner.Err()
		_ = fh.Close()
		if scanErr != nil {
			return ReplaySummary{}, nil, fmt.Errorf("scan replay log %s: %w", fpath, scanErr)
		}
	}

	if summary.TotalReplayed > 0 {
		denom := float64(summary.TotalReplayed)
		summary.RecordedEmptyRate = round3(float64(totalRecEmpty) / denom)
		summary.LiveEmptyRate = round3(float64(totalLiveEmpty) / denom)
		summary.RecordedErrorRate = round3(float64(totalRecErr) / denom)
		summary.LiveErrorRate = round3(float64(totalLiveErr) / denom)
		summary.RecordedP50Ms = round2(percentileFloat64(allRecLat, 0.50))
		summary.RecordedP95Ms = round2(percentileFloat64(allRecLat, 0.95))
		summary.LiveP50Ms = round2(percentileFloat64(allLiveLat, 0.50))
		summary.LiveP95Ms = round2(percentileFloat64(allLiveLat, 0.95))
	}
	if len(allOverlaps) > 0 {
		summary.Top5OverlapMean = round3(meanFloat64(allOverlaps))
		summary.Top5OverlapSamples = len(allOverlaps)
	}

	for tool, ta := range byTool {
		denom := float64(ta.count)
		item := ReplayToolSummary{
			Tool:               tool,
			Count:              ta.count,
			RecordedEmptyCount: ta.recEmpty,
			LiveEmptyCount:     ta.liveEmpty,
			RecordedEmptyRate:  round3(float64(ta.recEmpty) / denom),
			LiveEmptyRate:      round3(float64(ta.liveEmpty) / denom),
			RecordedErrorCount: ta.recErr,
			LiveErrorCount:     ta.liveErr,
			RecordedErrorRate:  round3(float64(ta.recErr) / denom),
			LiveErrorRate:      round3(float64(ta.liveErr) / denom),
			RecordedP50Ms:      round2(percentileFloat64(ta.recLatencies, 0.50)),
			RecordedP95Ms:      round2(percentileFloat64(ta.recLatencies, 0.95)),
			LiveP50Ms:          round2(percentileFloat64(ta.liveLatencies, 0.50)),
			LiveP95Ms:          round2(percentileFloat64(ta.liveLatencies, 0.95)),
		}
		if len(ta.overlaps) > 0 {
			item.Top5OverlapMean = round3(meanFloat64(ta.overlaps))
			item.Top5OverlapSamples = len(ta.overlaps)
		}
		summary.ByTool = append(summary.ByTool, item)
	}
	sort.Slice(summary.ByTool, func(i, j int) bool {
		return summary.ByTool[i].Tool < summary.ByTool[j].Tool
	})

	for _, wa := range byWS {
		denom := float64(wa.count)
		summary.ByWorkspace = append(summary.ByWorkspace, ReplayWorkspaceSummary{
			Slot:          wa.slot,
			Count:         wa.count,
			LiveEmptyRate: round3(float64(wa.liveEmpty) / denom),
			LiveErrorRate: round3(float64(wa.liveErr) / denom),
			LiveP50Ms:     round2(percentileFloat64(wa.liveLatencies, 0.50)),
			LiveP95Ms:     round2(percentileFloat64(wa.liveLatencies, 0.95)),
		})
	}
	sort.Slice(summary.ByWorkspace, func(i, j int) bool {
		return summary.ByWorkspace[i].Slot < summary.ByWorkspace[j].Slot
	})

	hashes := make([]string, 0, len(touchedHashes))
	for h := range touchedHashes {
		hashes = append(hashes, h)
	}
	sort.Strings(hashes)
	return summary, hashes, nil
}

func extractTop5Keys(items []OrderedItem) []string {
	seen := make(map[string]bool)
	var keys []string
	for _, it := range items {
		if it.Rank > 5 && len(keys) >= 5 {
			break
		}
		k := strings.TrimSpace(firstNonEmpty(it.Path, it.ID, it.Name))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		keys = append(keys, k)
		if len(keys) >= 5 {
			break
		}
	}
	return keys
}

func jaccardTop5(a, b []string) float64 {
	setA := make(map[string]bool, len(a))
	union := make(map[string]bool, len(a)+len(b))
	for _, k := range a {
		setA[k] = true
		union[k] = true
	}
	inter := 0
	for _, k := range b {
		union[k] = true
		if setA[k] {
			inter++
		}
	}
	if len(union) == 0 {
		return 0
	}
	return float64(inter) / float64(len(union))
}
