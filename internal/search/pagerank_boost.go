package search

import (
	"net/url"
	"sort"
	"strings"
)

func ApplyPageRankBoost(results []Result, scores map[string]float64, weight float64) []Result {
	if len(results) == 0 || weight <= 0 || len(scores) == 0 {
		return results
	}

	boosted := make([]Result, len(results))
	for i, r := range results {
		boosted[i] = r
		importance := 0.5
		symbolName := extractSymbolFromSourcePath(r.SourcePath)
		key := r.SourcePath + "::" + symbolName
		if s, ok := scores[key]; ok {
			importance = s
		} else if s, ok := scores[symbolName]; ok {
			importance = s
		}
		boosted[i].Score *= (1 + importance*weight)
	}

	sort.Slice(boosted, func(i, j int) bool {
		if boosted[i].Score != boosted[j].Score {
			return boosted[i].Score > boosted[j].Score
		}
		return boosted[i].ID < boosted[j].ID
	})

	return boosted
}

func extractSymbolFromSourcePath(sourcePath string) string {
	if idx := strings.Index(sourcePath, "?"); idx >= 0 {
		q, err := url.ParseQuery(sourcePath[idx+1:])
		if err == nil {
			if sym := q.Get("symbol"); sym != "" {
				return sym
			}
		}
	}
	return ""
}
