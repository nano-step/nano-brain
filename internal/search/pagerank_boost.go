package search

import "sort"

func ApplyPageRankBoost(results []Result, scores map[string]float64, weight float64) []Result {
	if len(results) == 0 || weight <= 0 || len(scores) == 0 {
		return results
	}

	boosted := make([]Result, len(results))
	for i, r := range results {
		boosted[i] = r
		importance := 0.5
		if s, ok := scores[r.SourcePath]; ok {
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
