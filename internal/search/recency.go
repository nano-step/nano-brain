package search

import (
	"math"
	"sort"
	"time"
)

// ApplyRecencyBoost blends RRF scores with an exponential-decay recency signal.
// final = (1 - weight) * score + weight * exp(-ln2 * ageDays / halfLife)
func ApplyRecencyBoost(results []Result, weight float64, halfLifeDays int, now time.Time) []Result {
	if weight == 0 || len(results) == 0 {
		return results
	}

	halfLife := float64(halfLifeDays)
	ln2 := math.Ln2

	for i := range results {
		ageDays := now.Sub(results[i].UpdatedAt).Hours() / 24
		if ageDays < 0 {
			ageDays = 0
		}
		multiplier := math.Exp(-ln2 * ageDays / halfLife)
		results[i].Score = (1-weight)*results[i].Score + weight*multiplier
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}
