package search

import "sort"

func ApplyEntityBoost(results []Result, matchedChunkIDs map[string]int, boostFactor float64) []Result {
	if len(results) == 0 || len(matchedChunkIDs) == 0 || boostFactor <= 0 {
		return results
	}

	boosted := make([]Result, len(results))
	for i, r := range results {
		boosted[i] = r
		if count, ok := matchedChunkIDs[r.ID]; ok {
			boosted[i].Score += float64(count) * boostFactor
		}
	}

	sort.Slice(boosted, func(i, j int) bool {
		if boosted[i].Score != boosted[j].Score {
			return boosted[i].Score > boosted[j].Score
		}
		return boosted[i].ID < boosted[j].ID
	})

	return boosted
}
